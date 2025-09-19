package packages

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	packagestep "github.com/mensylisir/kubexm/pkg/step/packages"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallPackagesTask struct {
	task.Base
}

func NewInstallPackagesTask() task.Task {
	return &InstallPackagesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallPackages",
				Description: "Install prerequisite packages on all nodes",
			},
		},
	}
}

func (t *InstallPackagesTask) Name() string {
	return t.Meta.Name
}

func (t *InstallPackagesTask) Description() string {
	return t.Meta.Description
}

func (t *InstallPackagesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *InstallPackagesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	installPackagesStep := packagestep.NewInstallPackagesStepBuilder(*runtimeCtx, "InstallPackages").Build()

	node := &plan.ExecutionNode{
		Name:  "InstallPackages",
		Step:  installPackagesStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
