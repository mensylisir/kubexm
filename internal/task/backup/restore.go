package backup

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/etcd"
	k8sbackup "github.com/mensylisir/kubexm/internal/step/kubernetes/backup"
	pkikubexm "github.com/mensylisir/kubexm/internal/step/pki/kubexm"
	"github.com/mensylisir/kubexm/internal/task"
)

// ===================================================================
// Restore Tasks - restore from backup
// ===================================================================

// RestorePKITask restores PKI certificates to all control plane nodes.
type RestorePKITask struct {
	task.Base
}

func NewRestorePKITask() task.Task {
	return &RestorePKITask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RestorePKI",
				Description: "Restore PKI certificates to control plane nodes",
			},
		},
	}
}

func (t *RestorePKITask) Name() string        { return t.Meta.Name }
func (t *RestorePKITask) Description() string { return t.Meta.Description }

func (t *RestorePKITask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	return len(hosts) > 0, nil
}

func (t *RestorePKITask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		step, err := pkikubexm.NewKubexmDistributeK8sPKIStepBuilder(
			runtime.ForHost(execCtx, host), fmt.Sprintf("RestorePKI-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create PKI restore step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RestorePKI-%s", host.GetName()),
			Step:  step,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// RestoreEtcdTask restores etcd data from a snapshot on all control plane nodes.
type RestoreEtcdTask struct {
	task.Base
	SnapshotPath string
}

func NewRestoreEtcdTask(snapshotPath string) task.Task {
	return &RestoreEtcdTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RestoreEtcd",
				Description: "Restore etcd data from a snapshot on control plane nodes",
			},
		},
		SnapshotPath: snapshotPath,
	}
}

func (t *RestoreEtcdTask) Name() string        { return t.Meta.Name }
func (t *RestoreEtcdTask) Description() string { return t.Meta.Description }

func (t *RestoreEtcdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Cannot restore external etcd via kubexm tasks
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Type == string(common.EtcdDeploymentTypeExternal) {
		return false, nil
	}
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	return len(hosts) > 0, nil
}

func (t *RestoreEtcdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		snapshotPath := t.SnapshotPath
		if snapshotPath == "" {
			return nil, fmt.Errorf("snapshot path is required for etcd restore. Please provide --snapshot-path flag pointing to the backup snapshot file")
		}
		
		// Step 1: Stop etcd before restore
		stopStep, err := etcd.NewStopEtcdStepBuilder(
			runtime.ForHost(execCtx, host), fmt.Sprintf("StopEtcd-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create etcd stop step for %s: %w", host.GetName(), err)
		}
		
		// Step 2: Restore etcd data
		restoreStep, err := etcd.NewRestoreEtcdStepBuilder(
			runtime.ForHost(execCtx, host), fmt.Sprintf("RestoreEtcd-%s", host.GetName())).
			WithLocalSnapshotPath(snapshotPath).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create etcd restore step for %s: %w", host.GetName(), err)
		}
		
		// Step 3: Start etcd after restore
		startStep, err := etcd.NewStartEtcdStepBuilder(
			runtime.ForHost(execCtx, host), fmt.Sprintf("StartEtcd-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create etcd start step for %s: %w", host.GetName(), err)
		}

		stopNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("StopEtcd-%s", host.GetName()),
			Step:  stopStep,
			Hosts: []remotefw.Host{host},
		})
		restoreNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RestoreEtcd-%s", host.GetName()),
			Step:  restoreStep,
			Hosts: []remotefw.Host{host},
		})
		startNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("StartEtcd-%s", host.GetName()),
			Step:  startStep,
			Hosts: []remotefw.Host{host},
		})
		
		entryNodes = append(entryNodes, stopNodeID)
		fragment.AddDependency(stopNodeID, restoreNodeID)
		fragment.AddDependency(restoreNodeID, startNodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// RestoreK8sConfigsTask restores Kubernetes configuration from backup on all nodes.
type RestoreK8sConfigsTask struct {
	task.Base
}

func NewRestoreK8sConfigsTask() task.Task {
	return &RestoreK8sConfigsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RestoreK8sConfigs",
				Description: "Restore Kubernetes configuration from backup on control plane nodes",
			},
		},
	}
}

func (t *RestoreK8sConfigsTask) Name() string        { return t.Meta.Name }
func (t *RestoreK8sConfigsTask) Description() string { return t.Meta.Description }

func (t *RestoreK8sConfigsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	return len(hosts) > 0, nil
}

func (t *RestoreK8sConfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		step, err := k8sbackup.NewRestoreFetchedConfigsStepBuilder(
			runtime.ForHost(execCtx, host), fmt.Sprintf("RestoreK8sConfigs-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create K8s configs restore step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RestoreK8sConfigs-%s", host.GetName()),
			Step:  step,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
