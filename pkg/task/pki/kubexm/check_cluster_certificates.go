package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubexmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubexm"
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
				Description: "Fetches core Kubernetes PKI files and checks the expiration of CA and leaf certificates",
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
	return true, nil
}

func (t *CheckClusterCertificatesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterNodes := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterNodes) == 0 {
		return nil, fmt.Errorf("no master nodes found for task %s", t.Name())
	}
	firstMaster := masterNodes[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []connector.Host{controlNode}

	fetchPkiStep := kubexmstep.NewKubxmFetchPKIStepBuilder(*runtimeCtx, "FetchCorePKI").Build()
	checkCaStep := kubexmstep.NewKubexmCheckK8sCAExpirationStepBuilder(*runtimeCtx, "CheckCAExpiration").Build()
	checkLeafStep := kubexmstep.NewKubexmCheckLeafCertsExpirationStepBuilder(*runtimeCtx, "CheckLeafCertsExpiration").Build()

	fetchPkiNode := &plan.ExecutionNode{Name: "FetchCorePKIFromMaster", Step: fetchPkiStep, Hosts: []connector.Host{firstMaster}}
	checkCaNode := &plan.ExecutionNode{Name: "CheckCAExpirationLocally", Step: checkCaStep, Hosts: controlNodeList}
	checkLeafNode := &plan.ExecutionNode{Name: "CheckLeafCertsExpirationOnMaster", Step: checkLeafStep, Hosts: []connector.Host{firstMaster}}

	fetchPkiNodeID, _ := fragment.AddNode(fetchPkiNode)
	checkCaNodeID, _ := fragment.AddNode(checkCaNode)
	checkLeafNodeID, _ := fragment.AddNode(checkLeafNode)

	fragment.AddDependency(fetchPkiNodeID, checkCaNodeID)
	fragment.AddDependency(fetchPkiNodeID, checkLeafNodeID)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*CheckClusterCertificatesTask)(nil)
