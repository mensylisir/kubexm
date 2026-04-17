package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/pki/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	masterNodes := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterNodes) == 0 {
		return nil, fmt.Errorf("no master nodes found for task %s", t.Name())
	}
	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	fetchPkiStep, err := kubeadm.NewKubeadmFetchFullPKIStepBuilder(runtimeCtx, "FetchK8sPKI").Build()
	if err != nil {
		return nil, err
	}
	checkCaStep, err := kubeadm.NewKubeadmCheckK8sCAExpirationStepBuilder(runtimeCtx, "CheckK8sCAExpiration").Build()
	if err != nil {
		return nil, err
	}
	checkLeafStep, err := kubeadm.NewKubeadmCheckLeafCertsExpirationStepBuilder(runtimeCtx, "CheckK8sLeafCertsExpiration").Build()
	if err != nil {
		return nil, err
	}

	fetchPkiNode := &plan.ExecutionNode{Name: "FetchKubernetesPKI", Step: fetchPkiStep, Hosts: []remotefw.Host{controlNode}}

	checkCaNode := &plan.ExecutionNode{Name: "CheckKubernetesCAExpiration", Step: checkCaStep, Hosts: []remotefw.Host{controlNode}}

	checkLeafNode := &plan.ExecutionNode{Name: "CheckKubernetesLeafCertsExpiration", Step: checkLeafStep, Hosts: []remotefw.Host{masterNodes[0]}}

	fetchPkiNodeID, _ := fragment.AddNode(fetchPkiNode)
	checkCaNodeID, _ := fragment.AddNode(checkCaNode)
	checkLeafNodeID, _ := fragment.AddNode(checkLeafNode)

	fragment.AddDependency(fetchPkiNodeID, checkCaNodeID)
	fragment.AddDependency(fetchPkiNodeID, checkLeafNodeID)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*CheckClusterCertificatesTask)(nil)
