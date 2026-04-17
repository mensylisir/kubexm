package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubexmstep "github.com/mensylisir/kubexm/internal/step/pki/kubexm"
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

	runtimeCtx := ctx.ForTask(t.Name())

	masterNodes := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterNodes) == 0 {
		return nil, fmt.Errorf("no master nodes found for task %s", t.Name())
	}
	firstMaster := masterNodes[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []remotefw.Host{controlNode}

	fetchPkiStep, err := kubexmstep.NewKubxmFetchPKIStepBuilder(runtimeCtx, "FetchCorePKI").Build()
	if err != nil {
		return nil, err
	}
	checkCaStep, err := kubexmstep.NewKubexmCheckK8sCAExpirationStepBuilder(runtimeCtx, "CheckCAExpiration").Build()
	if err != nil {
		return nil, err
	}
	checkLeafStep, err := kubexmstep.NewKubexmCheckLeafCertsExpirationStepBuilder(runtimeCtx, "CheckLeafCertsExpiration").Build()
	if err != nil {
		return nil, err
	}

	fetchPkiNode := &plan.ExecutionNode{Name: "FetchCorePKIFromMaster", Step: fetchPkiStep, Hosts: []remotefw.Host{firstMaster}}
	checkCaNode := &plan.ExecutionNode{Name: "CheckCAExpirationLocally", Step: checkCaStep, Hosts: controlNodeList}
	checkLeafNode := &plan.ExecutionNode{Name: "CheckLeafCertsExpirationOnMaster", Step: checkLeafStep, Hosts: []remotefw.Host{firstMaster}}

	fetchPkiNodeID, _ := fragment.AddNode(fetchPkiNode)
	checkCaNodeID, _ := fragment.AddNode(checkCaNode)
	checkLeafNodeID, _ := fragment.AddNode(checkLeafNode)

	fragment.AddDependency(fetchPkiNodeID, checkCaNodeID)
	fragment.AddDependency(fetchPkiNodeID, checkLeafNodeID)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*CheckClusterCertificatesTask)(nil)
