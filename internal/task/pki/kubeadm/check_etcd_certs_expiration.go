package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
)

type CheckEtcdCertsExpirationTask struct {
	task.Base
}

func NewCheckEtcdCertsExpirationTask() task.Task {
	return &CheckEtcdCertsExpirationTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckEtcdCertsExpiration",
				Description: "Checks the expiration of Kubeadm etcd CA certificates (ca.crt)",
			},
		},
	}
}

func (t *CheckEtcdCertsExpirationTask) Name() string {
	return t.Meta.Name
}

func (t *CheckEtcdCertsExpirationTask) Description() string {
	return t.Meta.Description
}

func (t *CheckEtcdCertsExpirationTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm) {
		return true, nil
	}
	return false, nil
}

func (t *CheckEtcdCertsExpirationTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdHosts) == 0 {
		return nil, fmt.Errorf("no etcd hosts found in context, cannot determine certificate source")
	}
	sourceHost := etcdHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	fetchPKIStep, err := kubeadm.NewKubeadmFetchFullPKIStepBuilder(runtimeCtx, "FetchEtcdPKI").Build()
	if err != nil {
		return nil, err
	}
	checkEtcdCAStep, err := kubeadm.NewKubeadmCheckEtcdCAExpirationStepBuilder(runtimeCtx, "CheckEtcdCA").Build()
	if err != nil {
		return nil, err
	}
	checkEtcdLeafCertStep, err := kubeadm.NewKubeadmCheckLeafCertsExpirationStepBuilder(runtimeCtx, "CheckEtcdLeafs").Build()
	if err != nil {
		return nil, err
	}

	fetchPKINode := &plan.ExecutionNode{Name: "FetchEtcdPKI", Step: fetchPKIStep, Hosts: []connector.Host{sourceHost}}
	checkEtcdCANode := &plan.ExecutionNode{Name: "CheckEtcdCA", Step: checkEtcdCAStep, Hosts: []connector.Host{controlNode}}
	checkEtcdLeafCertNode := &plan.ExecutionNode{Name: "CheckEtcdLeafs", Step: checkEtcdLeafCertStep, Hosts: []connector.Host{sourceHost}}

	fragment.AddNode(fetchPKINode, "FetchEtcdPKI")
	fragment.AddNode(checkEtcdCANode, "CheckEtcdCA")
	fragment.AddNode(checkEtcdLeafCertNode, "CheckEtcdLeafs")

	fragment.AddDependency("FetchEtcdPKI", "CheckEtcdCA")
	fragment.AddDependency("FetchEtcdPKI", "CheckEtcdLeafs")
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
