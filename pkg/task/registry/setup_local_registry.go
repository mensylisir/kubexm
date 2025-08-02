package registry

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/registry"
	"github.com/mensylisir/kubexm/pkg/task"
)

type SetupLocalRegistryTask struct {
	task.Base
}

func NewSetupLocalRegistryTask(ctx *task.TaskContext) (task.Interface, error) {
	// This task should run on the node(s) designated for the local registry.
	// This might be a specific role, or the control plane node.
	// For now, we assume it runs on the control node.
	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	s := &SetupLocalRegistryTask{
		Base: task.Base{
			Name:   "SetupLocalRegistry",
			Desc:   "Setup a local container image registry",
			Hosts:  []connector.Host{controlPlaneHost},
			Action: new(SetupLocalRegistryAction),
		},
	}
	return s, nil
}

type SetupLocalRegistryAction struct {
	task.Action
}

func (a *SetupLocalRegistryAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Setup Local Registry Phase")

	host := a.GetHosts()[0] // We assume this task runs on a single host for now.

	// The workflow for setting up a local registry would be:
	// Download -> Extract -> Generate Config -> Install Service -> Enable -> Start

	download := registry.NewDownloadRegistryStep(ctx, "DownloadRegistry")
	p.AddNode("download-registry", &plan.ExecutionNode{Step: download, Hosts: []connector.Host{host}})

	extract := registry.NewExtractRegistryStep(ctx, "ExtractRegistry")
	p.AddNode("extract-registry", &plan.ExecutionNode{Step: extract, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{"download-registry"}})

	install := registry.NewInstallRegistryStep(ctx, "InstallRegistry")
	p.AddNode("install-registry", &plan.ExecutionNode{Step: install, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{"extract-registry"}})

	genConfig := registry.NewGenerateConfigStep(ctx, "GenerateRegistryConfig")
	p.AddNode("gen-registry-config", &plan.ExecutionNode{Step: genConfig, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{"install-registry"}})

	installSvc := registry.NewInstallRegistryServiceStep(ctx, "InstallRegistryService")
	p.AddNode("install-registry-svc", &plan.ExecutionNode{Step: installSvc, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{"gen-registry-config"}})

	enable := registry.NewEnableRegistryStep(ctx, "EnableRegistry")
	p.AddNode("enable-registry", &plan.ExecutionNode{Step: enable, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{"install-registry-svc"}})

	start := registry.NewStartRegistryStep(ctx, "StartRegistry")
	p.AddNode("start-registry", &plan.ExecutionNode{Step: start, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{"enable-registry"}})

	return p, nil
}
