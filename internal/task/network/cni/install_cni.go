package cnitask

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/network/cni"
	"github.com/mensylisir/kubexm/internal/task"
)

type InstallCNIPluginsTask struct {
	task.Base
}

func NewInstallCNIPluginsTask() task.Task {
	return &InstallCNIPluginsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallCNIPlugins",
				Description: "Extract and install CNI plugin binaries on all nodes (assets prepared in Preflight)",
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

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return nil, fmt.Errorf("no master or worker hosts found to install CNI plugins")
	}

	extractCNI, err := cni.NewExtractCNIPluginsStepBuilder(runtimeCtx, "ExtractCNI").Build()
	if err != nil {
		return nil, err
	}
	installCNI, err := cni.NewInstallCNIPluginsStepBuilder(runtimeCtx, "InstallCNI").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCNI", Step: extractCNI, Hosts: []remotefw.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCNI", Step: installCNI, Hosts: deployHosts})

	fragment.AddDependency("ExtractCNI", "InstallCNI")

	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
