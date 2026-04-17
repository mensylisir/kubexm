package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeconfigstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubeconfig"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master host found to generate admin.conf")
	}
	executionHost := []remotefw.Host{masterHosts[0]}

	generateAdminConf, err := kubeconfigstep.NewGenerateAdminKubeconfigStepBuilder(runtimeCtx, "GenerateAdminKubeconfig").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateAdminKubeconfig", Step: generateAdminConf, Hosts: executionHost})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
