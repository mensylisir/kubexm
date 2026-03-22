# kubexm 架构重构计划

## 主题
kubexm-script 架构重构：原子化执行流设计

## 层级职责

### 入口层 (bin/kubexm)
CLI入口，解析子命令和参数。

### 编排层 (Pipeline -> Module -> Task)

**Pipeline**: 跨主机的全流程定义。
**Module**: 功能组件级封装。
**Task**: 动作集封装，是 Step 的有序集合。

### 执行层 (Step -> Runner -> Connector)

**Step**: 最小不可分割单位（如 systemctl restart），要求原子化、幂等。
**Runner**: 屏蔽执行细节，负责将 Step 转换为具体的命令下发。
**Connector**: SSH 传输层封装。

## 重构目标

消除代码冗余，实现"一套 Step 库，多场景组装任务"。

## 优化点说明

1. **明确层级关系**: 将原本平铺的叙述改成了 1-7 的层级，AI 更容易理解调用链（Call Stack）。
2. **强化原子性与幂等性**: 这是运维工具稳健性的核心，在 Prompt 中强调这一点，AI 生成的代码会包含状态检查逻辑。
3. **引入组装概念**: 明确提到"组合优于继承"，这样 AI 在设计时会倾向于使用配置化或插件化的方式，而不是写死代码。
4. **补充基础设施层 (Connector)**: 明确它是 SSH 的封装。

## 项目目录结构

```
kubexm/
├── bin/
│   └── kubexm           # 入口脚本
├── internal/
│   ├── cmd/             # CLI 子命令处理
│   ├── pipeline/        # 流程编排逻辑
│   ├── module/          # 业务模块定义
│   ├── task/            # 任务组装逻辑
│   ├── step/            # 原子步骤库 (例如: restart_service.go, copy_file.go)
│   ├── runner/          # 执行器引擎
│   └── connector/       # SSH 封装
└── configs/             # 存放 pipeline/task 的 YAML 定义
```

## 核心执行流（自顶向下）

1. **Entrypoint (bin/kubexm)**: 基于 CLI 的入口，解析子命令和参数。
2. **Pipeline 层**: 全局工作流引擎，负责加载配置、初始化环境并按顺序触发 Module。
3. **Module 层**: 业务领域封装，管理一组相关的 Task。
4. **Task 层**: 逻辑任务集合（如 UpgradeKubelet），通过组装不同的原子 Step 实现。
5. **Step 层 (Atomic Unit)**: 最小执行单元。必须满足原子性和幂等性（如：若服务已运行则跳过）。
6. **Runner 层**: 执行驱动层。负责 Step 的生命周期（准备、执行、回退、清理）及重试逻辑。
7. **Connector 层**: 基础设施抽象。对 SSH 的封装，负责建立连接、命令下发及文件传输。

## 支撑体系

### Logger (日志系统)
- 支持分级日志（Debug, Info, Warn, Error）
- 结构化输出：支持 JSON 格式以便后期审计，同时支持带颜色的 Console 输出
- 任务关联：日志需自动携带 task_id 或 step_name 标签

### Context (上下文管理)
- 在 Pipeline、Task、Step 之间传递全局状态、变量和取消信号（Context Propagation）

### Parser (解析中心)
- 负责解析配置config.yaml、主机列表host.yaml、SSH 凭据、目标主机列表及业务参数

### Conf (配置中心)
- 支持多源配置（YAML/Environment/CLI Flags）

### Utils (通用工具集)
- 包含字符串处理、文件 I/O 辅助、网络检测、时间格式化等无状态工具函数

### Errors (异常处理)
- 自定义错误类型，明确区分"可恢复错误"（触发重试）与"致命错误"（终止 Pipeline）

### Containers
- 存储了各种操作系统的dockerfile，以便离线操作系统依赖的时候使用

### Templates
- 模板中心，存储各种模板

### Cache
- 缓存中心

## 设计要求

1. **组合优于继承**: Task 应通过配置或注册的方式组装 Step；Module应该通过配置或注册的方式组装Task; Pipeline应该通过配置或注册的方式组装Module，避免硬编码。
2. **解耦设计**: Step 严禁直接调用 Connector，必须通过 Runner 调用，以保持 Step 本身的纯粹性。
3. **强类型约束**: 各层级输入输出需定义明确的 Interface 或 Struct。

## Kubernetes安装类型

### kubeadm
使用 kubeadm init/join 方式部署 Kubernetes。

### kubexm (二进制)
使用二进制方式部署 Kubernetes。

## Etcd安装类型

### kubeadm
通过 kubeadm 部署 etcd 作为 static pod。

### kubexm (二进制)
使用二进制方式部署 etcd。

### exists
使用已存在的 etcd 集群，跳过安装，只进行配置。

## LoadBalancer配置

### 禁用模式
- `highAvailability.enabled = false`: 不部署负载均衡

### external模式 (在loadbalancer角色机器上部署)
- `type = kubexm-kh`: 在loadbalancer角色机器上部署 keepalived + haproxy
- `type = kubexm-kn`: 在loadbalancer角色机器上部署 keepalived + nginx
- `type = kube-vip`: 使用 kube-vip 作为负载均衡
- `type = external`: 已存在负载均衡，跳过部署，直接使用

### internal模式 (在所有worker上部署代理到master)
- `type = haproxy` 且 `kubernetes.type = kubeadm`: worker上使用静态pod部署haproxy
- `type = haproxy` 且 `kubernetes.type = kubexm`: worker上使用二进制部署haproxy
- `type = nginx` 且 `kubernetes.type = kubeadm`: worker上使用静态pod部署nginx
- `type = nginx` 且 `kubernetes.type = kubexm`: worker上使用二进制部署nginx

## 运行模式

### 离线模式
1. 在有网络的机器上执行 `kubexm download -f config.yaml`
2. 将 packages 目录复制到内网环境
3. 执行 `kubexm create cluster -f config.yaml`

### 在线模式
1. 直接执行 `kubexm create cluster -f config.yaml`
2. 程序自动下载所需资源并创建集群

### 中心机器原则
- 所有操作都通过堡垒机(中心机器)发起
- 所有 kubernetes 机器上的包、配置、文件、证书都通过堡垒机分发
- host.yaml 中配置机器地址，不允许 localhost/127.0.0.1
- 即使操作本机，也使用 SSH 大网地址连接

## 主机配置约束

- host.yaml 中定义所有主机
- 不允许 localhost 或 127.0.0.1
- 即使操作本机，也要检测本机大网地址，使用 SSH 操作

## 离线工具依赖

脚本中使用的所有工具都需要离线化:
- jq (预编译二进制)
- yq (预编译二进制)
- 其他运维工具

## 下载流程说明

### download 命令行为
- `kubexm download -f config.yaml` 在有网络的环境下执行
- download 时不需要校验 host.yaml，因为在离线模式下 download 是将有网络的包下载的过程
- 正式安装时需要将整个 packages 目录打包到离线环境

## 重构后删除

重构完成后删除原来的目录（cmd/, pkg/ 等旧目录）
