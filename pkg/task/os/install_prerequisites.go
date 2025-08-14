package os

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	packagesstep "github.com/mensylisir/kubexm/pkg/step/packages"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallPrerequisitesTask struct {
	task.Base
}

func NewInstallPrerequisitesTask() task.Task {
	return &InstallPrerequisitesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallPrerequisites",
				Description: "Install prerequisite packages on all nodes based on online/offline mode",
			},
		},
	}
}

func (t *InstallPrerequisitesTask) Name() string {
	return t.Meta.Name
}

func (t *InstallPrerequisitesTask) Description() string {
	return t.Meta.Description
}

func (t *InstallPrerequisitesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *InstallPrerequisitesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	if ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Offline mode detected. Planning steps for offline package installation.")

		distribute := packagesstep.NewDistributePackagesStepBuilder(*runtimeCtx, "DistributeOfflinePackages").Build()
		extract := packagesstep.NewExtractPackagesStepBuilder(*runtimeCtx, "ExtractOfflinePackages").Build()
		installOffline := packagesstep.NewInstallOfflinePackagesStepBuilder(*runtimeCtx, "InstallOfflinePackages").Build()

		fragment.AddNode(&plan.ExecutionNode{Name: "DistributeOfflinePackages", Step: distribute, Hosts: allHosts})
		fragment.AddNode(&plan.ExecutionNode{Name: "ExtractOfflinePackages", Step: extract, Hosts: allHosts})
		fragment.AddNode(&plan.ExecutionNode{Name: "InstallOfflinePackages", Step: installOffline, Hosts: allHosts})

		fragment.AddDependency("DistributeOfflinePackages", "ExtractOfflinePackages")
		fragment.AddDependency("ExtractOfflinePackages", "InstallOfflinePackages")

	} else {
		ctx.GetLogger().Info("Online mode detected. Planning step for online package installation.")

		installOnline := packagesstep.NewInstallPackagesStepBuilder(*runtimeCtx, "InstallOnlinePackages").Build()

		fragment.AddNode(&plan.ExecutionNode{Name: "InstallOnlinePackages", Step: installOnline, Hosts: allHosts})
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
