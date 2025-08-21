package cni

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/network/cni"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallCNIPluginsTask struct {
	task.Base
}

func NewInstallCNIPluginsTask() task.Task {
	return &InstallCNIPluginsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallCNIPlugins",
				Description: "Download, extract and install CNI plugin binaries on all nodes",
			},
		},
	}
}

func (t *InstallCNIPluginsTask) Name() string {
	return t.Meta.Name
}

func (t *InstallCNIPluginsTask) Description() string {
	return t.Meta.Description
}

func (t *InstallCNIPluginsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *InstallCNIPluginsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return nil, fmt.Errorf("no master or worker hosts found to install CNI plugins")
	}

	extractCNI := cni.NewExtractCNIPluginsStepBuilder(*runtimeCtx, "ExtractCNI").Build()
	installCNI := cni.NewInstallCNIPluginsStepBuilder(*runtimeCtx, "InstallCNI").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCNI", Step: extractCNI, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCNI", Step: installCNI, Hosts: deployHosts})

	fragment.AddDependency("ExtractCNI", "InstallCNI")

	isOffline := ctx.IsOfflineMode()
	if !isOffline {
		ctx.GetLogger().Info("Online mode detected. Adding download step for CNI plugins.")
		downloadCNI := cni.NewDownloadCNIPluginsStepBuilder(*runtimeCtx, "DownloadCNI").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadCNI", Step: downloadCNI, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadCNI", "ExtractCNI")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download step for CNI plugins.")
	}

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
