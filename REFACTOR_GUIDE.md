# kubexm 架构重构完整指南

## 一、核心架构原则

### 1. 层级职责（严格分层，严禁跨层调用）

```
CLI (bin/kubexm)
  ↓
Pipeline 层：跨主机全流程定义，编排 Module
  ↓
Module 层：功能组件级封装，编排 Task
  ↓
Task 层：动作集封装，Step 的有序集合（组件级完整操作）
  ↓
Step 层：最小不可分割单位（原子化、幂等）
  ↓
Runner 层：屏蔽执行细节，Step 生命周期管理
  ↓
Connector 层：SSH 传输层封装
```

**关键规则**：
- ✅ Task 通过配置或注册的方式组装 Step
- ✅ Module 通过配置或注册的方式组装 Task
- ✅ Pipeline 通过配置或注册的方式组装 Module
- ❌ 严禁跨层调用（Step 不能直接调 Connector，必须通过 Runner）
- ❌ Task 不编排其他 Task，只做组件级完整操作

### 2. Kubernetes 安装类型支持

| 类型 | kubernetes_type 值 | 说明 |
|------|-------------------|------|
| kubeadm | `kubeadm` | 使用 kubeadm 安装 |
| kubexm | `kubexm` | 使用二进制部署 |

### 3. etcd 部署类型支持

| 类型 | etcd.type 值 | 说明 |
|------|-------------|------|
| kubeadm | `kubeadm` | 使用 kubeadm 管理 |
| kubexm | `kubexm` | 使用二进制部署 |
| exists | `exists` | 已存在 etcd，跳过安装直接配置 |

### 4. LoadBalancer 配置矩阵

| loadbalancer_mode | loadbalancer_type | kubernetes_type | 部署位置 | 部署方式 |
|-------------------|------------------|-----------------|---------|---------|
| external | kubexm_kh | - | loadbalancer 角色机器 | keepalived+haproxy |
| external | kubexm_kn | - | loadbalancer 角色机器 | keepalived+nginx |
| internal | haproxy | kubeadm | 所有 worker | 静态 pod 部署 haproxy |
| internal | haproxy | kubexm | 所有 worker | 二进制部署 haproxy |
| internal | nginx | kubeadm | 所有 worker | 静态 pod 部署 nginx |
| internal | nginx | kubexm | 所有 worker | 二进制部署 nginx |
| kube-vip | - | - | - | 使用 kube-vip |
| exists | - | - | - | 已存在 LB，跳过部署 |

## 二、路径管理规范

### 1. 下载路径规则

```
${download_base_dir}/
├── iso/
│   └── ${os_name}/${os_version}/${arch}/
│       └── ${os_name}-${os_version}-${arch}.iso
├── kubelet/
│   └── v1.24.9/amd64/kubelet
├── helm/
│   └── v3.12.0/amd64/helm
├── helm_packages/        # helm charts 自带版本号，不区分架构
│   └── prometheus-15.0.0.tgz
├── manifests/
│   └── coredns/coredns.yaml
└── ${component_name}/
    └── ${version}/${arch}/${component_name}
```

**架构判断规则**：
- 根据 `spec.arch` 判断，配置多个架构则下载多个架构
- host.yaml 中机器没配置 arch 默认 `x86`（即 amd64）
- 安装时根据 host.yaml 中机器架构分发对应版本

### 2. 多集群防覆盖

使用 `metadata.name` 作为集群名称区分路径：

```
${workdir}/
└── ${cluster_name}/
    ├── certs/
    │   ├── kubernetes/
    │   └── etcd/
    ├── kubeconfig/
    ├── manifests/
    └── hosts/
```

### 3. 证书轮转路径

```
${workdir}/rotate/
├── kubernetes/
│   ├── old/           # 旧证书
│   ├── new/           # 新根证书 ca.crt
│   └── bundle/        # bundle 后 ca.crt
└── etcd/
    ├── old/
    ├── new/
    └── bundle/
```

## 三、离线/在线模式

### 离线模式流程
```
1. 用户在有网环境执行：kubexm download --config cluster.yaml
2. 所有包下载到 ${download_base_dir}
3. 用户将整个 kubexm 目录打包到离线环境
4. 在离线环境执行：kubexm create cluster --config cluster.yaml
```

### 在线模式流程
```
1. 用户直接执行：kubexm create cluster --config cluster.yaml
2. 程序自动执行 download → create
```

**关键原则**：
- 中心机器（堡垒机）拥有所有包和文件
- 所有 kubernetes 机器上的包/配置/文件/证书都通过堡垒机分发
- download 时不校验 host.yaml（因为离线模式下 download 时还没有 host.yaml）

## 四、必需功能清单

### 1. 每台机器必须执行的步骤
- [x] 连通性检测（每个 Pipeline 前必须执行）
- [x] /etc/hosts 添加（安装时）
- [x] /etc/hosts 删除（删除集群时）
- [x] 主机名设置
- [x] Registry 域名和 hostname 写入

### 2. 所有工具离线支持
- jq, yq 等工具预编译为二进制
- 所有 k8s 组件支持离线
- OS 依赖支持离线（ISO 制作）

### 3. 不需要参数
- ❌ --source-registry：镜像源地址离线时就知道，不需要指定

## 五、Step 目录结构规范

```
internal/step/
├── common/           # 公共辅助（原 helpers/）
├── os/               # OS 配置（hosts/swap/firewall/hostname）
├── kubernetes/       # K8s 组件（按组件分目录）
│   ├── apiserver/
│   ├── controller-manager/
│   ├── scheduler/
│   ├── kubelet/
│   ├── kube-proxy/
│   ├── kubeadm/
│   ├── kubeconfig/
│   ├── kubectl/
│   └── ...
├── etcd/             # etcd 组件
├── loadbalancer/     # LB 组件
│   ├── haproxy/
│   ├── nginx/
│   ├── keepalived/
│   └── kube-vip/
├── network/          # 网络插件
│   ├── calico/
│   ├── cilium/
│   ├── flannel/
│   └── ...
├── runtime/          # 容器运行时（docker/containerd/cri-o）
├── registry/         # 镜像仓库
├── certs/            # 证书管理
├── download/         # 下载管理
├── iso/              # ISO 制作
├── images/           # 镜像推送
└── ...
```

**规则**：
- 按**组件名称**分割目录
- 同类组件放同一目录（如 kube-apiserver/ → kubernetes/apiserver/）
- 公共函数放 step/common/
- 严禁 lb/, kube_vip/, lib/ 等混乱目录

## 六、Task/Module/Pipeline 编排规则

### Task 层
- 只做**组件级原子操作**
- 不编排其他 Task
- 示例：`task::kubelet::remove` = 删除 kubelet（组合多个 Step）

### Module 层
- 编排多个 Task
- 示例：`module::cluster::remove` = 编排 kubelet remove + kubeadm reset + etcd remove

### Pipeline 层
- 编排 Module
- 每个 Pipeline 前先执行连通性检测 Module
- 参数验证

## 七、实现优先级

### P0（立即实现）
- ✅ Context cancelFn 泄漏修复
- ✅ 用户确认机制（assumeYes）
- ✅ 图验证（Validate）
- ✅ 参数验证

### P1（近期实现）
- ✅ LinkFragments 错误检查
- ✅ MergeFragment 冲突检测
- ✅ Pipeline 超时
- ✅ 重试机制

### P2（后续完善）
- [ ] 完整 Step 目录重构（600+ 文件）
- [ ] hosts 添加/删除完整实现
- [ ] 证书轮转完整实现
- [ ] ISO 制作完整实现
