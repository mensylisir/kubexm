package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CheckClusterCertificatesTask struct {
	task.Base
}

func NewCheckClusterCertificatesTask() task.Task {
	return &CheckClusterCertificatesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckClusterCertificates",
				Description: "Fetches Kubernetes PKI and checks the expiration of CA and leaf certificates",
			},
		},
	}
}

func (t *CheckClusterCertificatesTask) Name() string {
	return t.Meta.Name
}

func (t *CheckClusterCertificatesTask) Description() string {
	return t.Meta.Description
}

func (t *CheckClusterCertificatesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Kubernetes.Type == string(common.KubernetesDeploymentTypeKubeadm) {
		return true, nil
	}
	return false, nil
}

func (t *CheckClusterCertificatesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext's base context is not of type *runtime.Context")
	}

	masterNodes := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterNodes) == 0 {
		return nil, fmt.Errorf("no master nodes found for task %s", t.Name())
	}
	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	fetchPkiStep := kubeadm.NewKubeadmFetchFullPKIStepBuilder(*runtimeCtx, "FetchK8sPKI").Build()
	checkCaStep := kubeadm.NewKubeadmCheckK8sCAExpirationStepBuilder(*runtimeCtx, "CheckK8sCAExpiration").Build()
	checkLeafStep := kubeadm.NewKubeadmCheckLeafCertsExpirationStepBuilder(*runtimeCtx, "CheckK8sLeafCertsExpiration").Build()

	fetchPkiNode := &plan.ExecutionNode{Name: "FetchKubernetesPKI", Step: fetchPkiStep, Hosts: []connector.Host{controlNode}}

	checkCaNode := &plan.ExecutionNode{Name: "CheckKubernetesCAExpiration", Step: checkCaStep, Hosts: []connector.Host{controlNode}}

	checkLeafNode := &plan.ExecutionNode{Name: "CheckKubernetesLeafCertsExpiration", Step: checkLeafStep, Hosts: []connector.Host{masterNodes[0]}}

	fetchPkiNodeID, _ := fragment.AddNode(fetchPkiNode)
	checkCaNodeID, _ := fragment.AddNode(checkCaNode)
	checkLeafNodeID, _ := fragment.AddNode(checkLeafNode)

	fragment.AddDependency(fetchPkiNodeID, checkCaNodeID)
	fragment.AddDependency(fetchPkiNodeID, checkLeafNodeID)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*CheckClusterCertificatesTask)(nil)
