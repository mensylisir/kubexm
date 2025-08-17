package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	etcdstep "github.com/mensylisir/kubexm/pkg/step/pki/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for task %s: %w", t.Name(), err)
	}
	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 {
		return nil, fmt.Errorf("no etcd nodes found for task %s", t.Name())
	}

	restoreCaStep := etcdstep.NewRestoreCAFromRemoteStepBuilder(*runtimeCtx, "RestoreEtcdCAFromRemote").Build()
	checkCaStep := etcdstep.NewCheckCAExpirationStepBuilder(*runtimeCtx, "CheckEtcdCAExpiration").Build()
	checkLeafStep := etcdstep.NewCheckLeafCertsExpirationStepBuilder(*runtimeCtx, "CheckLeafCertificateExpiration").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "RestoreEtcdCAFromRemote", Step: restoreCaStep, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckEtcdCAExpiration", Step: checkCaStep, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckLeafCertificateExpiration", Step: checkLeafStep, Hosts: etcdNodes})

	fragment.AddDependency("RestoreEtcdCAFromRemote", "CheckEtcdCAExpiration")
	fragment.AddDependency("RestoreEtcdCAFromRemote", "CheckLeafCertificateExpiration")

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*CheckRenewalStatusTask)(nil)
