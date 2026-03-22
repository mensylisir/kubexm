# Kubexm 持久化计划（执行中）

本文件用于持久化记录 kubexm 的核心规则、执行流与重构目标。后续任何实现与重构都必须以此为准。

**范围与入口**
- 入口：`bin/kubexm`（CLI）
- 执行链路：`Pipeline -> Module -> Task -> Step -> Runner -> Connector`
- 目标：消除冗余，实现“一套 Step 库，多场景组装任务”，保证原子性与幂等性

**安装类型约束（硬性规则）**
- Kubernetes 安装类型：仅允许 `kubeadm` 与 `kubexm`（二进制）
- Etcd 安装类型：仅允许 `kubeadm`、`kubexm`、`exists`（已存在，跳过安装，仅配置）

**负载均衡规则（硬性规则）**
- 负载均衡总开关：`enabled`（启用/禁用）
- `loadbalancer_mode`：
  - `external`：在 `loadbalancer` 角色机器上部署
  - `internal`：在所有 `worker` 上部署代理，kubelet 连接本地 LB
  - `kube-vip`：使用 kube-vip
  - `exists`：已有负载均衡，跳过部署，直接使用
- `loadbalancer_type` 与组合约束：
  - `external + kubexm_kh`：`keepalived + haproxy`
  - `external + kubexm_kn`：`keepalived + nginx`
  - `internal + haproxy + kubernetes_type=kubeadm`：worker 上静态 Pod 部署 haproxy
  - `internal + haproxy + kubernetes_type=kubexm`：worker 上二进制部署 haproxy
  - `internal + nginx + kubernetes_type=kubeadm`：worker 上静态 Pod 部署 nginx
  - `internal + nginx + kubernetes_type=kubexm`：worker 上二进制部署 nginx
  - `kube-vip`：使用 kube-vip 作为负载均衡
  - `exists`：跳过部署

**在线/离线流程（硬性规则）**
- 离线模式流程：
  - `kubexm download` 在有网络环境执行
  - 生成离线包后拷贝至内网环境（同时携带 `packages/` 离线包目录）
  - 内网执行 `kubexm create cluster --${arguments}`
  - `download` 不校验 `host.yaml`（因为下载发生在有网环境）
- 在线模式流程：
  - `kubexm create cluster --${arguments}` 自动执行下载与安装
- 下载统一入口：
  - 在线下载仅允许在 Preflight `PrepareAssets` 执行
  - 离线模式仅允许 `ExtractBundle` 解包
  - 业务 Task/Step 不再各自发起下载
- 不论在线/离线：
  - 所有包与文件均集中在“中心机器/堡垒机”，再分发到各 Kubernetes 节点

**主机与连接规则（硬性规则）**
- `host.yaml` 中禁止出现 `localhost` 或 `127.0.0.1`
- 未指定机器列表时，视为本机，但必须：
  - 自动检测本机可路由的主网卡地址
  - 仍然使用 SSH 连接本机该地址执行
- 禁止使用 LocalConnector 作为生产路径，连接统一走 SSH

**离线依赖（硬性规则）**
- 任何脚本/步骤使用到的 `jq`/`yq` 或其他工具，必须具备离线安装能力
- 所有 Kubernetes 组件与依赖必须可离线分发

**目录与重构目标**
- 强调“组合优于继承”，Pipeline/Module/Task 都通过配置或注册组装
- Step 禁止直接调用 Connector，只能通过 Runner
- 重构完成后删除旧目录（以迁移清单为准，避免误删）

**参数策略**
- 禁止引入 `--source-registry` 这类不必要参数
- 镜像源应在离线阶段已确定，不在运行时强制指定

**建议目录结构参考**
```
kubexm/
├── bin/
│   └── kubexm
├── internal/
│   ├── cmd/
│   ├── pipeline/
│   ├── module/
│   ├── task/
│   ├── step/
│   ├── runner/
│   └── connector/
└── configs/
```

**执行流（自顶向下）**
1. Entrypoint: `bin/kubexm` 解析 CLI 子命令与参数
2. Pipeline: 全局流程，加载配置，顺序触发 Module
3. Module: 业务封装，组织 Task
4. Task: 逻辑任务集合，组合 Step
5. Step: 原子化、幂等的最小执行单元
6. Runner: Step 生命周期、重试、执行细节
7. Connector: SSH 传输与命令下发
