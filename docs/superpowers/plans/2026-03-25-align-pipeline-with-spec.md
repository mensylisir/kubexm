# Align Pipeline Implementation with Spec

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor Pipeline implementations to match the pipeline layering design specification.

**Architecture:** The refactoring involves:
1. Breaking monolithic UpgradeModule into separate ControlPlaneUpgradeModule, WorkerUpgradeModule, and NetworkUpgradeModule
2. Adding missing modules to DeleteClusterPipeline (PreflightModule, WorkerCleanupModule, LoadBalancerCleanupModule)
3. Fixing AddNodesPipeline to conditionally run EtcdModule only for new etcd nodes
4. Standardizing module naming conventions
5. Fixing pre-existing bugs in existing code (e.g., `GetControlPlaneNodes()` doesn't exist)

**Tech Stack:** Go, KubeXM internal packages (module/, task/, step/, pipeline/)

---

## File Structure

### Files to Create

- `internal/module/kubernetes/controlplane_upgrade_module.go` - New module for control plane upgrade
- `internal/module/kubernetes/worker_upgrade_module.go` - New module for worker upgrade
- `internal/module/kubernetes/network_upgrade_module.go` - New module for network upgrade (placeholder)
- `internal/module/kubernetes/worker_cleanup_module.go` - New module for worker cleanup (drain + reset)
- `internal/module/loadbalancer/loadbalancer_cleanup_module.go` - New module for load balancer cleanup
- `internal/task/kubernetes/kubeadm/drain_node.go` - New task for draining worker nodes
- `internal/task/kubernetes/kubeadm/reset_node.go` - New task for resetting worker nodes

### Files to Modify

- `internal/task/kubernetes/kubeadm/upgrade_controlplane.go:51` - Fix bug: `GetControlPlaneNodes()` → `GetHostsByRole(common.RoleMaster)`
- `internal/pipeline/cluster/delete_cluster_pipeline.go:25-43` - Add missing modules to DeleteClusterPipeline
- `internal/pipeline/cluster/upgrade_cluster_pipeline.go:24-41` - Replace single UpgradeModule with three separate modules
- `internal/pipeline/cluster/add_nodes_pipeline.go:27-48` - Fix EtcdModule conditional logic
- `internal/module/kubernetes/upgrade_module.go` - DELETE this file (functionality split into new modules)
- `internal/module/cni/cni_cleanup_module.go` - Rename to network_cleanup_module.go and update module name
- `internal/task/loadbalancer/loadbalancer_tasks.go` - Add `GetLoadBalancerCleanupTasks()` function

### Files to Reference

- `docs/superpowers/specs/2026-03-25-pipeline-layering-design.md` - Specification document
- `internal/module/kubernetes/upgrade_module.go` - Current monolithic upgrade implementation to split
- `internal/task/kubernetes/kubeadm/upgrade_controlplane.go` - Existing upgrade controlplane task (has bug)
- `internal/task/kubernetes/kubeadm/upgrade_workers.go` - Existing upgrade workers task
- `internal/task/kubernetes/kubeadm/clean_kubernetes.go` - Existing kubernetes cleanup task
- `internal/step/kubernetes/perform/drain_node.go` - DrainNodeStep location (in perform package, not kubeadm)
- `internal/task/loadbalancer/loadbalancer_tasks.go` - Reference for creating GetLoadBalancerCleanupTasks

---

## CRITICAL: Pre-existing Bug Fix

**Before any other work, fix this bug in existing code:**

In `internal/task/kubernetes/kubeadm/upgrade_controlplane.go:51`, the code uses:
```go
controlPlaneNodes, err := ctx.GetControlPlaneNodes()
```

But `ctx.GetControlPlaneNodes()` does NOT exist in `runtime.TaskContext`. The correct method is:
```go
controlPlaneNodes := ctx.GetHostsByRole(common.RoleMaster)
```

---

## Task 1: Refactor UpgradeClusterPipeline - Split UpgradeModule

**Goal:** Replace single `UpgradeModule` with three separate modules: ControlPlaneUpgradeModule, WorkerUpgradeModule, NetworkUpgradeModule

### Subtask 1.1: Fix Pre-existing Bug in upgrade_controlplane.go

- [ ] **Step 1: Fix GetControlPlaneNodes bug**

Find line 51 in `internal/task/kubernetes/kubeadm/upgrade_controlplane.go`:
```go
controlPlaneNodes, err := ctx.GetControlPlaneNodes()
```

Replace with:
```go
controlPlaneNodes := ctx.GetHostsByRole(common.RoleMaster)
```

Also fix the error check on line 52-54 since `GetHostsByRole` doesn't return an error.

### Subtask 1.2: Create ControlPlaneUpgradeModule

- [ ] **Step 2: Create controlplane_upgrade_module.go**

```go
// internal/module/kubernetes/controlplane_upgrade_module.go
package kubernetes

import (
    "fmt"

    "github.com/mensylisir/kubexm/internal/module"
    "github.com/mensylisir/kubexm/internal/plan"
    "github.com/mensylisir/kubexm/internal/runtime"
    "github.com/mensylisir/kubexm/internal/task"
    taskKubeadm "github.com/mensylisir/kubexm/internal/task/kubernetes/kubeadm"
)

type ControlPlaneUpgradeModule struct {
    module.BaseModule
    targetVersion string
}

func NewControlPlaneUpgradeModule(targetVersion string) module.Module {
    return &ControlPlaneUpgradeModule{
        BaseModule:    module.NewBaseModule("ControlPlaneUpgrade", nil),
        targetVersion: targetVersion,
    }
}

func (m *ControlPlaneUpgradeModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
    logger := ctx.GetLogger().With("module", m.Name(), "target_version", m.targetVersion)
    moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

    taskCtx, ok := ctx.(runtime.TaskContext)
    if !ok {
        return nil, fmt.Errorf("context does not implement runtime.TaskContext")
    }

    // Reuse existing UpgradeControlPlaneTask
    upgradeControlPlaneTask := taskKubeadm.NewUpgradeControlPlaneTask(m.targetVersion)
    required, err := upgradeControlPlaneTask.IsRequired(taskCtx)
    if err != nil {
        return nil, fmt.Errorf("failed to check if upgrade control plane is required: %w", err)
    }
    if required {
        cpFrag, err := upgradeControlPlaneTask.Plan(taskCtx)
        if err != nil {
            return nil, fmt.Errorf("failed to plan upgrade control plane: %w", err)
        }
        if err := moduleFragment.MergeFragment(cpFrag); err != nil {
            return nil, fmt.Errorf("failed to merge control plane fragment: %w", err)
        }
    }

    moduleFragment.CalculateEntryAndExitNodes()
    logger.Info("ControlPlaneUpgrade module planning complete", "total_nodes", len(moduleFragment.Nodes))
    return moduleFragment, nil
}

var _ module.Module = (*ControlPlaneUpgradeModule)(nil)
```

### Subtask 1.3: Create WorkerUpgradeModule

- [ ] **Step 3: Create worker_upgrade_module.go**

```go
// internal/module/kubernetes/worker_upgrade_module.go
package kubernetes

import (
    "fmt"

    "github.com/mensylisir/kubexm/internal/module"
    "github.com/mensylisir/kubexm/internal/plan"
    "github.com/mensylisir/kubexm/internal/runtime"
    "github.com/mensylisir/kubexm/internal/task"
    taskKubeadm "github.com/mensylisir/kubexm/internal/task/kubernetes/kubeadm"
)

type WorkerUpgradeModule struct {
    module.BaseModule
    targetVersion string
}

func NewWorkerUpgradeModule(targetVersion string) module.Module {
    return &WorkerUpgradeModule{
        BaseModule:    module.NewBaseModule("WorkerUpgrade", nil),
        targetVersion: targetVersion,
    }
}

func (m *WorkerUpgradeModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
    logger := ctx.GetLogger().With("module", m.Name(), "target_version", m.targetVersion)
    moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

    taskCtx, ok := ctx.(runtime.TaskContext)
    if !ok {
        return nil, fmt.Errorf("context does not implement runtime.TaskContext")
    }

    upgradeWorkerTask := taskKubeadm.NewUpgradeWorkersTask(m.targetVersion)
    workerRequired, err := upgradeWorkerTask.IsRequired(taskCtx)
    if err != nil {
        return nil, fmt.Errorf("failed to check if worker upgrade is required: %w", err)
    }
    if workerRequired {
        workerFrag, err := upgradeWorkerTask.Plan(taskCtx)
        if err != nil {
            return nil, fmt.Errorf("failed to plan worker upgrade: %w", err)
        }
        if err := moduleFragment.MergeFragment(workerFrag); err != nil {
            return nil, fmt.Errorf("failed to merge worker fragment: %w", err)
        }
    }

    moduleFragment.CalculateEntryAndExitNodes()
    logger.Info("WorkerUpgrade module planning complete", "total_nodes", len(moduleFragment.Nodes))
    return moduleFragment, nil
}

var _ module.Module = (*WorkerUpgradeModule)(nil)
```

### Subtask 1.4: Create NetworkUpgradeModule (Placeholder)

- [ ] **Step 4: Create network_upgrade_module.go**

```go
// internal/module/kubernetes/network_upgrade_module.go
package kubernetes

import (
    "fmt"

    "github.com/mensylisir/kubexm/internal/module"
    "github.com/mensylisir/kubexm/internal/plan"
    "github.com/mensylisir/kubexm/internal/runtime"
)

type NetworkUpgradeModule struct {
    module.BaseModule
}

func NewNetworkUpgradeModule() module.Module {
    return &NetworkUpgradeModule{
        BaseModule: module.NewBaseModule("NetworkUpgrade", nil),
    }
}

func (m *NetworkUpgradeModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
    logger := ctx.GetLogger().With("module", m.Name())
    // Network upgrade is currently not implemented - return empty fragment
    logger.Info("NetworkUpgrade module is not yet implemented, skipping")
    return plan.NewEmptyFragment(m.Name()), nil
}

var _ module.Module = (*NetworkUpgradeModule)(nil)
```

### Subtask 1.5: Update UpgradeClusterPipeline

- [ ] **Step 5: Modify upgrade_cluster_pipeline.go**

Find this section in `NewUpgradeClusterPipeline()`:
```go
modules := []module.Module{
    preflight.NewPreflightModule(assumeYes),
    kubernetes.NewUpgradeModule(targetVersion),
}
```

Replace with:
```go
modules := []module.Module{
    preflight.NewPreflightModule(assumeYes),
    kubernetes.NewControlPlaneUpgradeModule(targetVersion),
    kubernetes.NewWorkerUpgradeModule(targetVersion),
    kubernetes.NewNetworkUpgradeModule(),
}
```

### Subtask 1.6: Delete Old UpgradeModule

- [ ] **Step 6: Delete upgrade_module.go**

```bash
rm internal/module/kubernetes/upgrade_module.go
```

---

## Task 2: Refactor DeleteClusterPipeline - Add Missing Modules

**Goal:** Add PreflightModule, WorkerCleanupModule, and LoadBalancerCleanupModule to DeleteClusterPipeline

### Subtask 2.1: Create DrainNodeTask

- [ ] **Step 1: Create drain_node.go**

```go
// internal/task/kubernetes/kubeadm/drain_node.go
package kubeadm

import (
    "fmt"

    "github.com/mensylisir/kubexm/internal/common"
    "github.com/mensylisir/kubexm/internal/connector"
    "github.com/mensylisir/kubexm/internal/plan"
    "github.com/mensylisir/kubexm/internal/runtime"
    "github.com/mensylisir/kubexm/internal/spec"
    "github.com/mensylisir/kubexm/internal/step/kubernetes/perform"
    "github.com/mensylisir/kubexm/internal/task"
)

type DrainNodeTask struct {
    task.Base
}

func NewDrainNodeTask() task.Task {
    return &DrainNodeTask{
        Base: task.Base{
            Meta: spec.TaskMeta{
                Name:        "DrainNode",
                Description: "Drains worker nodes before removal from cluster",
            },
        },
    }
}

func (t *DrainNodeTask) Name() string { return t.Meta.Name }
func (t *DrainNodeTask) Description() string { return t.Meta.Description }

func (t *DrainNodeTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
    return true, nil
}

func (t *DrainNodeTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
    fragment := plan.NewExecutionFragment(t.Name())
    runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

    // Get worker hosts
    workerHosts := ctx.GetHostsByRole(common.RoleWorker)
    if len(workerHosts) == 0 {
        ctx.GetLogger().Info("No worker hosts found, skipping drain")
        return fragment, nil
    }

    // Get control node - drain runs from control node
    controlNode, err := ctx.GetControlNode()
    if err != nil {
        return nil, fmt.Errorf("failed to get control node: %w", err)
    }

    // Create drain step for each worker node
    // DrainNodeStep runs from control node and targets a specific worker
    for _, worker := range workerHosts {
        drainStep, err := perform.NewDrainNodeStepBuilder(runtimeCtx, fmt.Sprintf("Drain%s", worker.GetName()), worker.GetName()).Build()
        if err != nil {
            return nil, fmt.Errorf("failed to create drain step for %s: %w", worker.GetName(), err)
        }
        fragment.AddNode(&plan.ExecutionNode{
            Name:  fmt.Sprintf("Drain%s", worker.GetName()),
            Step:  drainStep,
            Hosts: []connector.Host{controlNode}, // Runs from control node
        })
    }

    fragment.CalculateEntryAndExitNodes()
    return fragment, nil
}

var _ task.Task = (*DrainNodeTask)(nil)
```

### Subtask 2.2: Create ResetNodeTask

- [ ] **Step 2: Create reset_node.go**

```go
// internal/task/kubernetes/kubeadm/reset_node.go
package kubeadm

import (
    "github.com/mensylisir/kubexm/internal/common"
    "github.com/mensylisir/kubexm/internal/connector"
    "github.com/mensylisir/kubexm/internal/plan"
    "github.com/mensylisir/kubexm/internal/runtime"
    "github.com/mensylisir/kubexm/internal/spec"
    "github.com/mensylisir/kubexm/internal/step/kubernetes/kubeadm"
    "github.com/mensylisir/kubexm/internal/task"
)

type ResetNodeTask struct {
    task.Base
}

func NewResetNodeTask() task.Task {
    return &ResetNodeTask{
        Base: task.Base{
            Meta: spec.TaskMeta{
                Name:        "ResetNode",
                Description: "Resets kubeadm on worker nodes (kubeadm reset)",
            },
        },
    }
}

func (t *ResetNodeTask) Name() string { return t.Meta.Name }
func (t *ResetNodeTask) Description() string { return t.Meta.Description }

func (t *ResetNodeTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
    return true, nil
}

func (t *ResetNodeTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
    fragment := plan.NewExecutionFragment(t.Name())
    runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

    workerHosts := ctx.GetHostsByRole(common.RoleWorker)
    if len(workerHosts) == 0 {
        return fragment, nil
    }

    resetStep, err := kubeadm.NewKubeadmResetStepBuilder(runtimeCtx, "KubeadmReset").Build()
    if err != nil {
        return nil, err
    }

    // Reset runs on each worker node itself
    fragment.AddNode(&plan.ExecutionNode{Name: "KubeadmReset", Step: resetStep, Hosts: workerHosts})
    fragment.CalculateEntryAndExitNodes()
    return fragment, nil
}

var _ task.Task = (*ResetNodeTask)(nil)
```

### Subtask 2.3: Create WorkerCleanupModule

- [ ] **Step 3: Create worker_cleanup_module.go**

```go
// internal/module/kubernetes/worker_cleanup_module.go
package kubernetes

import (
    "fmt"

    "github.com/mensylisir/kubexm/internal/module"
    "github.com/mensylisir/kubexm/internal/plan"
    "github.com/mensylisir/kubexm/internal/runtime"
    "github.com/mensylisir/kubexm/internal/task"
    taskKubeadm "github.com/mensylisir/kubexm/internal/task/kubernetes/kubeadm"
)

type WorkerCleanupModule struct {
    module.BaseModule
}

func NewWorkerCleanupModule() module.Module {
    return &WorkerCleanupModule{
        BaseModule: module.NewBaseModule("WorkerCleanup", nil),
    }
}

func (m *WorkerCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
    logger := ctx.GetLogger().With("module", m.Name())
    moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

    taskCtx, ok := ctx.(runtime.TaskContext)
    if !ok {
        return nil, fmt.Errorf("context does not implement runtime.TaskContext")
    }

    // Drain node task (runs from control node, targets workers)
    drainTask := taskKubeadm.NewDrainNodeTask()
    drainRequired, err := drainTask.IsRequired(taskCtx)
    if err != nil {
        return nil, fmt.Errorf("failed to check if drain task is required: %w", err)
    }
    if drainRequired {
        drainFrag, err := drainTask.Plan(taskCtx)
        if err != nil {
            return nil, fmt.Errorf("failed to plan drain task: %w", err)
        }
        if err := moduleFragment.MergeFragment(drainFrag); err != nil {
            return nil, fmt.Errorf("failed to merge drain fragment: %w", err)
        }
    }

    // Reset node task (runs on worker nodes)
    resetTask := taskKubeadm.NewResetNodeTask()
    resetRequired, err := resetTask.IsRequired(taskCtx)
    if err != nil {
        return nil, fmt.Errorf("failed to check if reset task is required: %w", err)
    }
    if resetRequired {
        resetFrag, err := resetTask.Plan(taskCtx)
        if err != nil {
            return nil, fmt.Errorf("failed to plan reset task: %w", err)
        }
        if err := moduleFragment.MergeFragment(resetFrag); err != nil {
            return nil, fmt.Errorf("failed to merge reset fragment: %w", err)
        }
    }

    moduleFragment.CalculateEntryAndExitNodes()
    logger.Info("WorkerCleanup module planning complete", "total_nodes", len(moduleFragment.Nodes))
    return moduleFragment, nil
}

var _ module.Module = (*WorkerCleanupModule)(nil)
```

### Subtask 2.4: Add GetLoadBalancerCleanupTasks to loadbalancer_tasks.go

- [ ] **Step 4: Add GetLoadBalancerCleanupTasks function**

In `internal/task/loadbalancer/loadbalancer_tasks.go`, add a new function:

```go
// GetLoadBalancerCleanupTasks returns cleanup tasks for load balancer
// Based on the same logic as GetLoadBalancerTasks but for cleanup
func GetLoadBalancerCleanupTasks(ctx runtime.TaskContext) []task.Task {
    cfg := ctx.GetClusterConfig()
    var tasks []task.Task

    // Check if HA is enabled
    if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
        cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
        !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled {
        return tasks
    }

    // External LoadBalancer cleanup
    if cfg.Spec.ControlPlaneEndpoint.HighAvailability.External != nil &&
        cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled != nil &&
        *cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Enabled {

        externalType := cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type

        switch externalType {
        case string(common.ExternalLBTypeKubexmKH), string(common.ExternalLBTypeKubexmKN):
            // Keepalived cleanup
            tasks = append(tasks, NewCleanKeepalivedTask())
            // HAProxy/Nginx cleanup based on type
            if externalType == string(common.ExternalLBTypeKubexmKH) {
                tasks = append(tasks, haproxy.NewCleanHAProxyTask())
            } else {
                tasks = append(tasks, nginx.NewCleanNginxTask())
            }
        case string(common.ExternalLBTypeKubeVIP):
            tasks = append(tasks, kubevip.NewCleanKubeVipTask())
        }
    }

    // Internal LoadBalancer cleanup
    if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal != nil &&
        cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled != nil &&
        *cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Enabled {

        internalType := cfg.Spec.ControlPlaneEndpoint.HighAvailability.Internal.Type

        switch internalType {
        case string(common.InternalLBTypeHAProxy):
            tasks = append(tasks, haproxy.NewCleanHAProxyTask())
        case string(common.InternalLBTypeNginx):
            tasks = append(tasks, nginx.NewCleanNginxTask())
        case string(common.InternalLBTypeKubeVIP):
            tasks = append(tasks, kubevip.NewCleanKubeVipTask())
        }
    }

    return tasks
}
```

**Note:** This assumes `CleanKeepalivedTask`, `CleanHAProxyTask`, `CleanNginxTask`, and `CleanKubeVipTask` exist. Check and create if missing.

### Subtask 2.5: Create LoadBalancerCleanupModule

- [ ] **Step 5: Create loadbalancer_cleanup_module.go**

```go
// internal/module/loadbalancer/loadbalancer_cleanup_module.go
package loadbalancer

import (
    "fmt"

    "github.com/mensylisir/kubexm/internal/module"
    "github.com/mensylisir/kubexm/internal/plan"
    "github.com/mensylisir/kubexm/internal/runtime"
)

type LoadBalancerCleanupModule struct {
    module.BaseModule
}

func NewLoadBalancerCleanupModule() module.Module {
    return &LoadBalancerCleanupModule{
        BaseModule: module.NewBaseModule("LoadBalancerCleanup", nil),
    }
}

func (m *LoadBalancerCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
    logger := ctx.GetLogger().With("module", m.Name())
    moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

    taskCtx, ok := ctx.(runtime.TaskContext)
    if !ok {
        return nil, fmt.Errorf("context does not implement runtime.TaskContext")
    }

    cleanupTasks := GetLoadBalancerCleanupTasks(taskCtx)

    var previousTaskExitNodes []plan.NodeID
    for _, cleanupTask := range cleanupTasks {
        taskFrag, err := cleanupTask.Plan(taskCtx)
        if err != nil {
            return nil, fmt.Errorf("failed to plan load balancer cleanup task %s: %w", cleanupTask.Name(), err)
        }
        if taskFrag.IsEmpty() {
            continue
        }
        if err := moduleFragment.MergeFragment(taskFrag); err != nil {
            return nil, err
        }
        if len(previousTaskExitNodes) > 0 {
            plan.LinkFragments(moduleFragment, previousTaskExitNodes, taskFrag.EntryNodes)
        }
        previousTaskExitNodes = taskFrag.ExitNodes
    }

    if len(moduleFragment.Nodes) == 0 {
        logger.Info("LoadBalancerCleanup module returned empty fragment")
        return plan.NewEmptyFragment(m.Name()), nil
    }

    moduleFragment.CalculateEntryAndExitNodes()
    logger.Info("LoadBalancerCleanup module planning complete", "total_nodes", len(moduleFragment.Nodes))
    return moduleFragment, nil
}

var _ module.Module = (*LoadBalancerCleanupModule)(nil)
```

### Subtask 2.6: Update DeleteClusterPipeline Module List

- [ ] **Step 6: Modify delete_cluster_pipeline.go**

Add imports for `preflight` and `loadbalancer` packages:
```go
import (
    "github.com/mensylisir/kubexm/internal/module"
    "github.com/mensylisir/kubexm/internal/module/cni"
    "github.com/mensylisir/kubexm/internal/module/etcd"
    "github.com/mensylisir/kubexm/internal/module/kubernetes"
    "github.com/mensylisir/kubexm/internal/module/loadbalancer"
    "github.com/mensylisir/kubexm/internal/module/preflight"
    moduleRuntime "github.com/mensylisir/kubexm/internal/module/runtime"
    "github.com/mensylisir/kubexm/internal/pipeline"
    "github.com/mensylisir/kubexm/internal/plan"
    runtime2 "github.com/mensylisir/kubexm/internal/runtime"
)
```

Find the modules list in `NewDeleteClusterPipeline()`:
```go
modules := []module.Module{
    kubernetes.NewControlPlaneCleanupModule(),
    cni.NewCNICleanupModule(),
    moduleRuntime.NewRuntimeCleanupModule(),
    etcd.NewEtcdCleanupModule(),
}
```

Replace with:
```go
modules := []module.Module{
    preflight.NewPreflightModule(assumeYes),
    kubernetes.NewWorkerCleanupModule(),
    kubernetes.NewControlPlaneCleanupModule(),
    cni.NewCNICleanupModule(),
    moduleRuntime.NewRuntimeCleanupModule(),
    etcd.NewEtcdCleanupModule(),
    loadbalancer.NewLoadBalancerCleanupModule(),
}
```

---

## Task 3: Fix AddNodesPipeline - Conditional EtcdModule

**Goal:** Make EtcdModule only run for nodes that are designated as etcd nodes

### Subtask 3.1: Analyze AddNodesPipeline Logic

- [ ] **Step 1: Review add_nodes_pipeline.go**

Check how EtcdModule is currently added. The spec says it should only run for new etcd nodes.

Current implementation in `internal/pipeline/cluster/add_nodes_pipeline.go` unconditionally includes `etcd.NewEtcdModule()`.

### Subtask 3.2: Implement Conditional EtcdModule Logic

- [ ] **Step 2: Add conditional logic for EtcdModule**

This requires:
1. Determining which nodes being added are etcd nodes
2. Only including EtcdModule if there are new etcd nodes

For now, implement a simple conditional:
```go
// Check if any of the new nodes have etcd role
// If adding new etcd nodes, include EtcdModule
// This is a placeholder - full implementation requires comparing
// current etcd members with cluster config
etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
if len(etcdHosts) > 0 {
    modules = append(modules, etcd.NewEtcdModule())
}
```

**Note:** Full implementation may require querying existing etcd cluster members via etcdctl and comparing with configured nodes.

---

## Task 4: Standardize Module Naming

**Goal:** Rename `CNICleanupModule` to `NetworkCleanupModule` for consistency

### Subtask 4.1: Rename cni_cleanup_module.go

- [ ] **Step 1: Rename file**

```bash
mv internal/module/cni/cni_cleanup_module.go internal/module/cni/network_cleanup_module.go
```

- [ ] **Step 2: Update module name in file**

In `internal/module/cni/network_cleanup_module.go`, update:
- Struct name from `CNICleanupModule` to `NetworkCleanupModule`
- Constructor from `NewCNICleanupModule` to `NewNetworkCleanupModule`
- `Name()` method to return `"NetworkCleanup"`

- [ ] **Step 3: Update references**

Search for `cni.NewCNICleanupModule` and replace with `cni.NewNetworkCleanupModule` in:
- `internal/pipeline/cluster/delete_cluster_pipeline.go`

---

## Verification Steps

After completing all tasks:

1. **Format code:**
   ```bash
   gofmt -w internal/module/kubernetes/ internal/module/loadbalancer/ internal/module/cni/ internal/pipeline/cluster/ internal/task/kubernetes/kubeadm/ internal/task/loadbalancer/
   ```

2. **Check for compile errors:**
   ```bash
   go build ./...
   ```

3. **Verify no broken imports:**
   ```bash
   grep -r "NewUpgradeModule" internal/
   ```
   Should return no results (module was deleted).

4. **Verify GetControlPlaneNodes is fixed:**
   ```bash
   grep -r "GetControlPlaneNodes" internal/
   ```
   Should return no results.

5. **Review the changes:**
   ```bash
   git diff --stat
   ```
