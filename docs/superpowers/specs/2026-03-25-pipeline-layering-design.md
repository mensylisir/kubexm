# Pipeline 分层设计规范

## 概述

KubeXM 采用 **Pipeline → Module → Task → Step** 四层编排结构，每一层都有明确的职责边界和组合原则。

## 分层原则

### Step（步骤层）- 最小原子单元

**职责：** 执行不可再分的原子动作。

**特征：**
- 幂等性：多次执行结果一致
- 原子性：一个 Step 不能依赖同一个 Task 内的其他 Step
- 可组合性：Step 可以自由组合成不同的 Task

**Step 类型分类：**

| 类型 | 示例 | 说明 |
|------|------|------|
| 下载类 | `DownloadContainerdStep` | 从源下载二进制/镜像 |
| 提取类 | `ExtractContainerdStep` | 解压压缩包 |
| 安装类 | `InstallContainerdStep` | 分发二进制到目标路径 |
| 配置类 | `ConfigureContainerdStep` | 渲染配置文件并写入 |
| 服务类 | `StartContainerdStep`, `EnableContainerdStep` | Systemd 服务管理 |
| 健康检查类 | `CheckHealthStep` | 验证组件状态 |
| 清理类 | `CleanContainerdStep` | 移除组件 |

### Task（任务层）- 组件生命周期

**职责：** 管理一个组件的完整生命周期。

**特征：**
- 自包含：一个 Task 包含组件从安装到运行所需的全部 Step
- 可跳过：通过 `IsRequired()` 判断是否需要执行
- 可独立运行：可以单独执行某个组件的安装/配置/清理

**Task 命名规范：** `{Component}{Action}Task`
- 示例：`DeployContainerdTask`, `CleanEtcdTask`, `JoinWorkerTask`

**Task 内 Step 编排顺序：**

```
Download → Extract → Install → Configure → Start → HealthCheck
                         ↓
                   (可选)InstallService → Enable
```

### Module（模块层）- 功能边界

**职责：** 处理特定功能域的完整生命周期。

**特征：**
- 组合多个相关的 Task
- 提供条件选择（根据配置选择不同的实现）

**Module 命名规范：** `{Domain}Module`
- 示例：`RuntimeModule`, `EtcdModule`, `NetworkModule`

### Pipeline（流水线层）- 业务编排

**职责：** 编排多个 Module 形成完整的业务流程。

**特征：**
- 定义执行顺序
- 管理全局状态（集群版本、配置等）
- 处理跨 Module 的依赖

## Pipeline 定义

### CreateClusterPipeline

```
CreateClusterPipeline
├── PreflightModule
│   ├── GreetingTask
│   ├── ConfirmTask
│   ├── ExtractBundleTask
│   ├── InstallPrerequisitesTask
│   ├── PreflightChecksTask
│   └── VerifyArtifactsTask
├── OsModule
│   ├── ConfigureHostnameTask
│   ├── DisableSwapTask
│   ├── DisableFirewallTask
│   ├── ConfigureKernelTask
│   └── InstallBasePackagesTask
├── RuntimeModule
│   ├── DeployContainerdTask
│   ├── DeployDockerTask
│   └── DeployCriOTask
├── LoadBalancerModule
│   ├── DeployKeepalivedTask
│   ├── DeployHaproxyTask
│   └── DeployKubeVipTask
├── EtcdModule
│   ├── GenerateEtcdPKITask
│   └── DeployEtcdClusterTask
├── ControlPlaneModule
│   ├── DeployKubeletTask
│   ├── InitControlPlaneTask
│   └── JoinControlPlaneTask
├── NetworkModule
│   ├── DeployCalicoTask
│   ├── DeployFlannelTask
│   ├── DeployCiliumTask
│   ├── DeployKubeOVNTask
│   └── DeployMultusTask
├── WorkerModule
│   └── JoinWorkerTask
└── AddonsModule
    └── InstallAddonsTask
```

### DeleteClusterPipeline

```
DeleteClusterPipeline
├── PreflightModule
├── WorkerCleanupModule
│   ├── DrainNodeTask
│   └── ResetNodeTask
├── ControlPlaneCleanupModule
│   └── CleanupControlPlaneTask
├── NetworkCleanupModule
│   └── CleanNetworkTask
├── RuntimeCleanupModule
│   ├── CleanContainerdTask
│   ├── CleanDockerTask
│   └── CleanCriOTask
├── EtcdCleanupModule
│   └── CleanEtcdTask
└── LoadBalancerCleanupModule
    ├── CleanKeepalivedTask
    ├── CleanHaproxyTask
    └── CleanKubeVipTask
```

### AddNodesPipeline

```
AddNodesPipeline
├── PreflightModule
├── OsModule
├── RuntimeModule
├── EtcdModule (仅新加入的 etcd 节点)
└── WorkerModule
    └── JoinWorkerTask
```

### UpgradeClusterPipeline

```
UpgradeClusterPipeline
├── PreflightModule
├── ControlPlaneUpgradeModule
│   ├── UpgradeKubeadmTask
│   ├── UpgradeKubeletTask
│   └── RestartControlPlaneTask
├── WorkerUpgradeModule
│   └── UpgradeWorkerTask
└── NetworkUpgradeModule (如需)
```

### UpgradeEtcdPipeline

```
UpgradeEtcdPipeline
├── PreflightModule
├── EtcdBackupModule
│   └── BackupEtcdTask
├── EtcdUpgradeModule
│   ├── UpgradeEtcdBinaryTask
│   ├── RestartEtcdTask
│   └── VerifyEtcdTask
└── PostUpgradeModule
```

### RenewPKIPipeline

```
RenewPKIPipeline
├── PreflightModule
├── PKIRenewalModule
│   ├── RenewCACertTask
│   ├── RenewEtcdCertTask
│   ├── RenewK8sCertTask
│   └── DistributeCertsTask
└── VerificationModule
```

## Step 设计原则

### 原子性要求

每个 Step 必须独立完成一个最小动作：

**正确示范：**
```
InstallContainerdStep      → 只安装二进制
ConfigureContainerdStep     → 只配置 config.toml
InstallContainerdServiceStep → 只安装 systemd unit
StartContainerdStep         → 只启动服务
EnableContainerdStep       → 只设置开机自启
```

**错误示范（耦合）：**
```
InstallAndConfigureContainerdStep  → 安装 + 配置混在一起
StartAndEnableContainerdStep       → 启动 + 设置自启混在一起
```

### 可组合性

修改配置后应能只执行必要的 Step：

```go
// 安装时：完整流程
task.Plan() → Download → Extract → Install → Configure → Start → Enable

// 修改配置时：只需
configureStep.Run()
restartStep.Run()
```

### Precheck 幂等性

每个 Step 必须实现幂等的 `Precheck()`：

```go
func (s *ConfigureContainerdStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
    // 检查配置是否已存在且内容一致
    // 返回 (true, nil) 表示已完成，无需执行
    // 返回 (false, nil) 表示需要执行
    // 返回 (_, error) 表示检查失败
}
```

## Task 设计原则

### IsRequired 判断

每个 Task 必须实现 `IsRequired()`，根据配置决定是否需要执行：

```go
func (t *DeployContainerdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
    return ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type == common.RuntimeTypeContainerd, nil
}
```

### Plan 方法

Task 的 `Plan()` 方法负责：
1. 创建 ExecutionFragment
2. 根据条件筛选需要的 Step
3. 设置 Step 的执行 Hosts
4. 定义 Step 之间的依赖关系

```go
func (t *DeployContainerdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
    fragment := plan.NewExecutionFragment(t.Name())

    // 添加 Step nodes
    installStep := containerd.NewInstallContainerdStepBuilder(ctx, "InstallContainerd").Build()
    configureStep := containerd.NewConfigureContainerdStepBuilder(ctx, "ConfigureContainerd").Build()

    fragment.AddNode(&plan.ExecutionNode{Name: "Install", Step: installStep, Hosts: deployHosts})
    fragment.AddNode(&plan.ExecutionNode{Name: "Configure", Step: configureStep, Hosts: deployHosts})

    // 定义依赖
    fragment.AddDependency("Install", "Configure")

    fragment.CalculateEntryAndExitNodes()
    return fragment, nil
}
```

## Module 设计原则

### 条件选择

Module 根据配置选择不同的 Task 实现：

```go
func (m *RuntimeModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
    runtimeType := ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type

    switch runtimeType {
    case common.RuntimeTypeContainerd:
        return m.planContainerd(ctx)
    case common.RuntimeTypeDocker:
        return m.planDocker(ctx)
    case common.RuntimeTypeCriO:
        return m.planCriO(ctx)
    default:
        return plan.NewEmptyFragment(m.Name()), nil
    }
}
```

### 空片段处理

当没有需要执行的 Task 时，返回空片段：

```go
if len(fragment.Nodes) == 0 {
    return plan.NewEmptyFragment(m.Name()), nil
}
```

## 附录：现有 Step 一览

### Containerd Step

| Step | 类型 | 说明 |
|------|------|------|
| `DownloadContainerdStep` | 下载 | 下载 containerd 压缩包 |
| `DownloadRuncStep` | 下载 | 下载 runc 压缩包 |
| `DownloadCrictlStep` | 下载 | 下载 crictl 工具 |
| `DownloadCNIPluginsStep` | 下载 | 下载 CNI 插件 |
| `ExtractContainerdStep` | 提取 | 解压 containerd |
| `ExtractRuncStep` | 提取 | 解压 runc |
| `ExtractCrictlStep` | 提取 | 解压 crictl |
| `ExtractCNIPluginsStep` | 提取 | 解压 CNI 插件 |
| `InstallContainerdStep` | 安装 | 安装 containerd 二进制 |
| `InstallRuncStep` | 安装 | 安装 runc 二进制 |
| `InstallCrictlStep` | 安装 | 安装 crictl 工具 |
| `InstallCNIPluginsStep` | 安装 | 安装 CNI 插件 |
| `ConfigureContainerdStep` | 配置 | 配置 config.toml |
| `ConfigureContainerdDropinStep` | 配置 | 配置 dropin 文件 |
| `ConfigureCrictlStep` | 配置 | 配置 crictl.yaml |
| `InstallContainerdServiceStep` | 服务 | 安装 systemd unit |
| `StartContainerdStep` | 服务 | 启动 containerd |
| `EnableContainerdStep` | 服务 | 设置开机自启 |
| `RestartContainerdStep` | 服务 | 重启 containerd |
| `StopContainerdStep` | 服务 | 停止 containerd |
| `DisableContainerdStep` | 服务 | 禁用 containerd |
| `CleanContainerdStep` | 清理 | 清理 containerd |

### OS Step

| Step | 类型 | 说明 |
|------|------|------|
| `SetHostnameStep` | 配置 | 设置主机名 |
| `AddHostsStep` | 配置 | 添加 /etc/hosts 条目 |
| `DisableSwapStep` | 配置 | 禁用 swap |
| `EnableSwapStep` | 配置 | 启用 swap |
| `DisableFirewallStep` | 配置 | 关闭防火墙 |
| `EnableFirewallStep` | 配置 | 开启防火墙 |
| `DisableSelinuxStep` | 配置 | 禁用 SELinux |
| `EnableSelinuxStep` | 配置 | 启用 SELinux |
| `AddSysctlStep` | 配置 | 设置 sysctl 参数 |
| `RemoveSysctlStep` | 配置 | 移除 sysctl 参数 |
| `AddSecurityStep` | 配置 | 设置安全限制 |
| `RemoveSecurityStep` | 配置 | 移除安全限制 |
| `AddModulesStep` | 配置 | 加载内核模块 |
| `RemoveModulesStep` | 配置 | 卸载内核模块 |
| `ConfigureTimezoneStep` | 配置 | 设置时区 |

### Etcd Step

| Step | 类型 | 说明 |
|------|------|------|
| `GenerateEtcdCAStep` | PKI | 生成 ETCD CA |
| `GenerateEtcdCertStep` | PKI | 生成 ETCD 证书 |
| `DistributeEtcdCertsStep` | PKI | 分发 ETCD 证书 |
| `ExtractEtcdStep` | 提取 | 解压 etcd |
| `InstallEtcdStep` | 安装 | 安装 etcd 二进制 |
| `ConfigureEtcdStep` | 配置 | 配置 etcd.yml |
| `InstallEtcdServiceStep` | 服务 | 安装 systemd unit |
| `StartEtcdStep` | 服务 | 启动 etcd |
| `StopEtcdStep` | 服务 | 停止 etcd |
| `RestartEtcdStep` | 服务 | 重启 etcd |
| `EnableEtcdStep` | 服务 | 设置开机自启 |
| `DisableEtcdStep` | 服务 | 禁用 etcd |
| `CheckEtcdHealthStep` | 健康检查 | 检查 etcd 健康状态 |
| `CheckClusterHealthStep` | 健康检查 | 检查集群健康状态 |
| `WaitClusterHealthyStep` | 健康检查 | 等待集群健康 |
| `AddMemberStep` | 运维 | 添加集群成员 |
| `RemoveMemberStep` | 运维 | 移除集群成员 |
| `BackupEtcdStep` | 备份 | 备份 etcd 数据 |
| `BackupDataStep` | 备份 | 备份数据 |
| `RestoreEtcdStep` | 恢复 | 恢复 etcd |
| `DefragEtcdStep` | 运维 | 整理 etcd 碎片 |
| `CleanEtcdStep` | 清理 | 清理 etcd |

### Kubernetes Step

| Step | 类型 | 说明 |
|------|------|------|
| `DownloadKubeadmStep` | 下载 | 下载 kubeadm |
| `DownloadKubeletStep` | 下载 | 下载 kubelet |
| `DownloadKubectlStep` | 下载 | 下载 kubectl |
| `ExtractKubeadmStep` | 提取 | 解压 kubeadm |
| `ExtractKubeletStep` | 提取 | 解压 kubelet |
| `ExtractKubectlStep` | 提取 | 解压 kubectl |
| `InstallKubeadmStep` | 安装 | 安装 kubeadm |
| `InstallKubeletStep` | 安装 | 安装 kubelet |
| `InstallKubectlStep` | 安装 | 安装 kubectl |
| `GenerateKubeletConfigStep` | 配置 | 生成 kubelet 配置 |
| `GenerateKubeProxyConfigStep` | 配置 | 生成 kube-proxy 配置 |
| `InstallKubeletServiceStep` | 服务 | 安装 kubelet systemd unit |
| `StartKubeletStep` | 服务 | 启动 kubelet |
| `StopKubeletStep` | 服务 | 停止 kubelet |
| `EnableKubeletStep` | 服务 | 设置 kubelet 开机自启 |
| `CleanKubeletStep` | 清理 | 清理 kubelet |
| `BootstrapFirstMasterStep` | 初始化 | 第一个 master 执行 kubeadm init |
| `JoinMasterStep` | 加入 | master 加入集群 |
| `JoinWorkerStep` | 加入 | worker 加入集群 |
| `CopyKubeconfigStep` | 配置 | 复制 kubeconfig |
