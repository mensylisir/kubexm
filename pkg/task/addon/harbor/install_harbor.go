package harbor

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/harbor"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallHarborTask struct {
	task.Base
}

func NewInstallHarborTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallHarborTask{
		Base: task.Base{
			Name:   "InstallHarbor",
			Desc:   "Install Harbor private registry addon",
			Hosts:  ctx.GetHosts(), // This might run on a specific node
			Action: new(InstallHarborAction),
		},
	}
	return s, nil
}

type InstallHarborAction struct {
	task.Action
}

func (a *InstallHarborAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install Harbor Phase")

	// Harbor installation is complex and might run on a dedicated host.
	// For now, we assume it's run from the control node.
	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	// Simplified workflow
	download := harbor.NewDownloadHarborStep(ctx, "DownloadHarbor")
	p.AddNode("download-harbor", &plan.ExecutionNode{Step: download, Hosts: []connector.Host{controlPlaneHost}})

	extract := harbor.NewExtractHarborStep(ctx, "ExtractHarbor")
	p.AddNode("extract-harbor", &plan.ExecutionNode{Step: extract, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"download-harbor"}})

	genCerts := harbor.NewGenerateCertsStep(ctx, "GenerateHarborCerts")
	p.AddNode("gen-harbor-certs", &plan.ExecutionNode{Step: genCerts, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"extract-harbor"}})

	genConfig := harbor.NewGenerateConfigStep(ctx, "GenerateHarborConfig")
	p.AddNode("gen-harbor-config", &plan.ExecutionNode{Step: genConfig, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"gen-harbor-certs"}})

	install := harbor.NewInstallHarborStep(ctx, "InstallHarbor")
	p.AddNode("install-harbor", &plan.ExecutionNode{Step: install, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"gen-harbor-config"}})

	start := harbor.NewStartHarborStep(ctx, "StartHarbor")
	p.AddNode("start-harbor", &plan.ExecutionNode{Step: start, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"install-harbor"}})

	return p, nil
}
