package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeconfigstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeconfig"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateAdminConfigTask struct {
	task.Base
}

func NewGenerateAdminKubeconfigTask() task.Task {
	return &GenerateAdminConfigTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateAdminConfig",
				Description: "Generate the admin.conf kubeconfig for cluster administration",
			},
		},
	}
}

func (t *GenerateAdminConfigTask) Name() string                                     { return t.Meta.Name }
func (t *GenerateAdminConfigTask) Description() string                              { return t.Meta.Description }
func (t *GenerateAdminConfigTask) IsRequired(ctx runtime.TaskContext) (bool, error) { return true, nil }

func (t *GenerateAdminConfigTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master host found to generate admin.conf")
	}
	executionHost := []connector.Host{masterHosts[0]}

	generateAdminConf := kubeconfigstep.NewGenerateAdminKubeconfigStepBuilder(*runtimeCtx, "GenerateAdminKubeconfig").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateAdminKubeconfig", Step: generateAdminConf, Hosts: executionHost})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
