package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	etcdstep "github.com/mensylisir/kubexm/internal/step/pki/etcd"
	"github.com/mensylisir/kubexm/internal/task"
)

type CheckRenewalStatusTask struct {
	task.Base
}

func NewCheckRenewalStatusTask() task.Task {
	return &CheckRenewalStatusTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckRenewalStatus",
				Description: "Ensures local CA exists and checks the expiration of all etcd certificates",
			},
		},
	}
}

func (t *CheckRenewalStatusTask) Name() string {
	return t.Meta.Name
}

func (t *CheckRenewalStatusTask) Description() string {
	return t.Meta.Description
}

func (t *CheckRenewalStatusTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CheckRenewalStatusTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())
	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for task %s: %w", t.Name(), err)
	}
	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 {
		return nil, fmt.Errorf("no etcd nodes found for task %s", t.Name())
	}

	restoreCaStep, err := etcdstep.NewRestoreCAFromRemoteStepBuilder(runtimeCtx, "RestoreEtcdCAFromRemote").Build()
	if err != nil {
		return nil, err
	}
	checkCaStep, err := etcdstep.NewCheckCAExpirationStepBuilder(runtimeCtx, "CheckEtcdCAExpiration").Build()
	if err != nil {
		return nil, err
	}
	checkLeafStep, err := etcdstep.NewCheckLeafCertsExpirationStepBuilder(runtimeCtx, "CheckLeafCertificateExpiration").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "RestoreEtcdCAFromRemote", Step: restoreCaStep, Hosts: []remotefw.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckEtcdCAExpiration", Step: checkCaStep, Hosts: []remotefw.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckLeafCertificateExpiration", Step: checkLeafStep, Hosts: etcdNodes})

	fragment.AddDependency("RestoreEtcdCAFromRemote", "CheckEtcdCAExpiration")
	fragment.AddDependency("RestoreEtcdCAFromRemote", "CheckLeafCertificateExpiration")

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*CheckRenewalStatusTask)(nil)
