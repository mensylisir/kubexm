package backup

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/etcd"
	pkicommon "github.com/mensylisir/kubexm/internal/step/pki/common"
	kubernetesbackup "github.com/mensylisir/kubexm/internal/step/kubernetes/backup"
	"github.com/mensylisir/kubexm/internal/task"
)

// ===================================================================
// Backup Tasks - wraps existing backup step implementations
// ===================================================================

// BackupPKITask backs up PKI certificates and keys from all control plane nodes.
type BackupPKITask struct {
	task.Base
}

func NewBackupPKITask() task.Task {
	return &BackupPKITask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "BackupPKI",
				Description: "Backup PKI certificates and keys from control plane nodes",
			},
		},
	}
}

func (t *BackupPKITask) Name() string        { return t.Meta.Name }
func (t *BackupPKITask) Description() string { return t.Meta.Description }

func (t *BackupPKITask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	return len(hosts) > 0, nil
}

func (t *BackupPKITask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(hosts) == 0 {
		return fragment, nil
	}

	// Back up from first control plane node
	host := hosts[0]
	backupDir := fmt.Sprintf("%s/backups/%s/%s/pki", ctx.GetGlobalWorkDir(), ctx.GetClusterConfig().Name, time.Now().Format("20060102-150405"))
	
	// Determine PKI source path based on deployment type
	pkiSourcePath := "/etc/kubernetes/pki" // kubeadm default
	if ctx.GetClusterConfig().Spec.Kubernetes != nil &&
		ctx.GetClusterConfig().Spec.Kubernetes.Type == string(common.KubernetesDeploymentTypeKubexm) {
		pkiSourcePath = "/etc/kubexm/pki" // kubexm uses different PKI path
	}
	
	step, err := pkicommon.NewBackupPKIStepBuilder(
		runtime.ForHost(execCtx, host), "BackupPKI", pkiSourcePath, backupDir).Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create PKI backup step: %w", err)
	}
	nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  "BackupPKI",
		Step:  step,
		Hosts: []remotefw.Host{host},
	})
	fragment.EntryNodes = []plan.NodeID{nodeID}
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// BackupEtcdTask backs up etcd data from the first control plane node.
type BackupEtcdTask struct {
	task.Base
}

func NewBackupEtcdTask() task.Task {
	return &BackupEtcdTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "BackupEtcd",
				Description: "Backup etcd data snapshot from control plane nodes",
			},
		},
	}
}

func (t *BackupEtcdTask) Name() string        { return t.Meta.Name }
func (t *BackupEtcdTask) Description() string { return t.Meta.Description }

func (t *BackupEtcdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	return len(hosts) > 0, nil
}

func (t *BackupEtcdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(hosts) == 0 {
		return fragment, nil
	}

	host := hosts[0]
	step, err := etcd.NewBackupEtcdStepBuilder(
		runtime.ForHost(execCtx, host), "BackupEtcd").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd backup step: %w", err)
	}
	nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  "BackupEtcd",
		Step:  step,
		Hosts: []remotefw.Host{host},
	})
	fragment.EntryNodes = []plan.NodeID{nodeID}
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// BackupK8sConfigsTask backs up Kubernetes configuration from all nodes.
type BackupK8sConfigsTask struct {
	task.Base
}

func NewBackupK8sConfigsTask() task.Task {
	return &BackupK8sConfigsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "BackupK8sConfigs",
				Description: "Backup Kubernetes configuration from all control plane nodes",
			},
		},
	}
}

func (t *BackupK8sConfigsTask) Name() string        { return t.Meta.Name }
func (t *BackupK8sConfigsTask) Description() string { return t.Meta.Description }

func (t *BackupK8sConfigsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	return len(hosts) > 0, nil
}

func (t *BackupK8sConfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		step, err := kubernetesbackup.NewBinaryFetchConfigsStepBuilder(
			runtime.ForHost(execCtx, host), fmt.Sprintf("BackupK8sConfigs-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create K8s config backup step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("BackupK8sConfigs-%s", host.GetName()),
			Step:  step,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
