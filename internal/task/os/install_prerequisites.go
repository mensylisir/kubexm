package os

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	packagesstep "github.com/mensylisir/kubexm/internal/step/packages"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	if ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Offline mode detected. Planning steps for offline package installation.")

		distribute, err := packagesstep.NewDistributePackagesStepBuilder(runtimeCtx, "DistributeOfflinePackages").Build()
		if err != nil {
			return nil, err
		}
		extract, err := packagesstep.NewExtractPackagesStepBuilder(runtimeCtx, "ExtractOfflinePackages").Build()
		if err != nil {
			return nil, err
		}
		installOffline, err := packagesstep.NewInstallOfflinePackagesStepBuilder(runtimeCtx, "InstallOfflinePackages").Build()
		if err != nil {
			return nil, err
		}

		fragment.AddNode(&plan.ExecutionNode{Name: "DistributeOfflinePackages", Step: distribute, Hosts: allHosts})
		fragment.AddNode(&plan.ExecutionNode{Name: "ExtractOfflinePackages", Step: extract, Hosts: allHosts})
		fragment.AddNode(&plan.ExecutionNode{Name: "InstallOfflinePackages", Step: installOffline, Hosts: allHosts})

		fragment.AddDependency("DistributeOfflinePackages", "ExtractOfflinePackages")
		fragment.AddDependency("ExtractOfflinePackages", "InstallOfflinePackages")

	} else {
		ctx.GetLogger().Info("Online mode detected. Planning step for online package installation.")

		installOnline, err := packagesstep.NewInstallPackagesStepBuilder(runtimeCtx, "InstallOnlinePackages").Build()
		if err != nil {
			return nil, err
		}

		fragment.AddNode(&plan.ExecutionNode{Name: "InstallOnlinePackages", Step: installOnline, Hosts: allHosts})
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
