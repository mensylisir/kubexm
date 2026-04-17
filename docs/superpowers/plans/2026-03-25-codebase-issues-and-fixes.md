# 代码库问题及修复计划

## 日期: 2026-03-25

## 一、问题汇总

### 1. DeleteClusterPipeline缺失模块
**问题**: DeleteClusterPipeline没有清理hosts和OS配置的步骤

**当前DeleteClusterPipeline模块列表**:
```
preflight.NewPreflightModule(assumeYes),
kubernetes.NewWorkerCleanupModule(),
kubernetes.NewControlPlaneCleanupModule(),
cni.NewNetworkCleanupModule(),
moduleRuntime.NewRuntimeCleanupModule(),
etcd.NewEtcdCleanupModule(),
loadbalancer.NewLoadBalancerCleanupModule(),
```

**缺失模块**:
- HostsCleanupModule - 清理/etc/hosts记录
- OsCleanupModule - 恢复OS配置(swap, firewall, selinux等)

**修复方案**:
1. 创建`module/os/cleanup_module.go`使用`task/os/CleanOSNodesTask`
2. 在DeleteClusterPipeline末尾添加这两个模块

### 2. ManageHostsStep未使用
**问题**: `step/common/manage_host.go`定义了`ManageHostsStep`但未被任何Task使用

**当前状态**:
- `UpdateEtcHostsStep`（step/os/add_hosts.go）正在被使用
- `RemoveEtcHostsStep`（step/os/remove_hosts.go）在`CleanOSNodesTask`中被使用

**修复方案**:
- `ManageHostsStep`可以废弃，或用于更细粒度的hosts管理
- 当前实现已足够，暂不需要修改

### 3. module/kubernetes/目录结构问题
**问题**: 所有kubernetes相关module文件平铺在module/kubernetes/下

**当前结构**:
```
module/kubernetes/
├── controlplane_cleanup_module.go
├── controlplane_module.go
├── controlplane_upgrade_module.go
├── network_upgrade_module.go
├── worker_cleanup_module.go
├── worker_module.go
└── worker_upgrade_module.go
```

**建议结构**:
```
module/kubernetes/
├── kubeadm/
│   ├── reset_module.go
│   └── ...
├── kubelet/
│   ├── install_module.go
│   ├── remove_module.go
│   └── ...
├── controlplane/
│   ├── install_module.go
│   └── ...
└── ...
```

**修复方案**:
- 保持当前结构，作为中期重构目标
- 短期：添加注释标注哪些属于哪个组件

### 4. PreflightConnectivityModule独立性
**问题**: 连通性检查集成在PreflightModule中，用户要求作为独立模块

**当前实现**:
- `CheckHostConnectivityStep`在`PreflightChecksTask`中被调用
- `PreflightModule`包含所有前置检查任务

**建议方案**:
- 保持当前实现，连通性检查已在PreflightChecksTask中
- 如需完全独立，需创建`module/preflight/connectivity/`模块

### 5. 离线工具jq/yq依赖
**问题**: 需要确认jq、yq等工具是否已离线化

**需要检查的位置**:
- scripts/目录
- 各种install步骤
- download流程

**修复方案**:
- 检查所有使用jq/yq的地方
- 确保这些工具在离线包中可用

## 二、修复优先级

### P0 - 必须修复
1. DeleteClusterPipeline添加HostsCleanupModule和OsCleanupModule
2. 确认离线工具依赖完整

### P1 - 建议修复
3. module/kubernetes/目录结构优化（中期目标）

### P2 - 后续优化
4. 评估ManageHostsStep是否需要

## 三、修复详情

### 3.1 DeleteClusterPipeline修复

**文件**: `internal/pipeline/cluster/delete_cluster_pipeline.go`

**修改内容**:
```go
// 新增导入
"github.com/mensylisir/kubexm/internal/module/os"

// Pipeline modules修改为:
modules := []module.Module{
    preflight.NewPreflightModule(assumeYes),
    kubernetes.NewWorkerCleanupModule(),
    kubernetes.NewControlPlaneCleanupModule(),
    cni.NewNetworkCleanupModule(),
    moduleRuntime.NewRuntimeCleanupModule(),
    etcd.NewEtcdCleanupModule(),
    loadbalancer.NewLoadBalancerCleanupModule(),
    os.NewOsCleanupModule(),  // 新增：清理hosts和OS配置
}
```

**新增文件**: `internal/module/os/cleanup_module.go`
```go
package os

import (
    "fmt"
    "github.com/mensylisir/kubexm/internal/module"
    "github.com/mensylisir/kubexm/internal/plan"
    "github.com/mensylisir/kubexm/internal/runtime"
    "github.com/mensylisir/kubexm/internal/task"
    taskos "github.com/mensylisir/kubexm/internal/task/os"
)

type OsCleanupModule struct {
    module.BaseModule
}

func NewOsCleanupModule() module.Module {
    return &OsCleanupModule{
        BaseModule: module.NewBaseModule("OsCleanup", nil),
    }
}

func (m *OsCleanupModule) GetTasks() []task.Task {
    return []task.Task{
        taskos.NewCleanOSNodesTask(),
    }
}

func (m *OsCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
    // 实现略，参考其他Module
}

var _ module.Module = (*OsCleanupModule)(nil)
```

## 四、测试验证

### 4.1 删除集群测试
```bash
# 1. 创建测试集群
kubexm create cluster -f test-config.yaml

# 2. 删除集群
kubexm delete cluster -f test-config.yaml

# 3. 验证
# - /etc/hosts中的KubeXM标记被清除
# - swap重新启用
# - firewall重新启用
# - selinux重新启用
```

### 4.2 连通性检查测试
```bash
# 1. 创建集群（带连通性检查）
kubexm create cluster -f test-config.yaml

# 2. 验证日志中包含CheckHostConnectivity步骤
```
