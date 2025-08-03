package containerd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/containerd"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallContainerdTask installs and configures containerd.
type InstallContainerdTask struct {
	task.Base
}

// NewInstallContainerdTask creates a new task for installing and configuring containerd.
func NewInstallContainerdTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallContainerdTask{
		Base: task.Base{
			Name:   "InstallContainerd",
			Desc:   "Install and configure containerd on all nodes",
			Hosts:  ctx.GetHosts(),
			Action: new(InstallContainerdAction),
		},
	}
	return s, nil
}

type InstallContainerdAction struct {
	task.Action
}

func (a *InstallContainerdAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install and Configure Containerd Phase")

	hosts := a.GetHosts()
	if len(hosts) == 0 {
		return p, nil
	}

	for _, host := range hosts {
		hostName := host.GetName()

		// --- Parallel Downloads ---
		dlContainerdNode := plan.NodeID(fmt.Sprintf("download-containerd-%s", hostName))
		p.AddNode(dlContainerdNode, &plan.ExecutionNode{
			Step:  containerd.NewDownloadContainerdStep(ctx, dlContainerdNode.String()),
			Hosts: []connector.Host{host},
		})

		dlRuncNode := plan.NodeID(fmt.Sprintf("download-runc-%s", hostName))
		p.AddNode(dlRuncNode, &plan.ExecutionNode{
			Step:  containerd.NewDownloadRuncStep(ctx, dlRuncNode.String()),
			Hosts: []connector.Host{host},
		})

		dlCniNode := plan.NodeID(fmt.Sprintf("download-cni-%s", hostName))
		p.AddNode(dlCniNode, &plan.ExecutionNode{
			Step:  containerd.NewDownloadCNIStep(ctx, dlCniNode.String()),
			Hosts: []connector.Host{host},
		})

		dlCrictlNode := plan.NodeID(fmt.Sprintf("download-crictl-%s", hostName))
		p.AddNode(dlCrictlNode, &plan.ExecutionNode{
			Step:  containerd.NewDownloadCrictlStep(ctx, dlCrictlNode.String()),
			Hosts: []connector.Host{host},
		})

		// --- Parallel Installations ---
		extractContainerdNode := plan.NodeID(fmt.Sprintf("extract-containerd-%s", hostName))
		p.AddNode(extractContainerdNode, &plan.ExecutionNode{
			Step:         containerd.NewExtractContainerdStep(ctx, extractContainerdNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{dlContainerdNode},
		})
		installContainerdNode := plan.NodeID(fmt.Sprintf("install-containerd-%s", hostName))
		p.AddNode(installContainerdNode, &plan.ExecutionNode{
			Step:         containerd.NewInstallContainerdStep(ctx, installContainerdNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{extractContainerdNode},
		})

		installRuncNode := plan.NodeID(fmt.Sprintf("install-runc-%s", hostName))
		p.AddNode(installRuncNode, &plan.ExecutionNode{
			Step:         containerd.NewInstallRuncStep(ctx, installRuncNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{dlRuncNode},
		})

		extractCniNode := plan.NodeID(fmt.Sprintf("extract-cni-%s", hostName))
		p.AddNode(extractCniNode, &plan.ExecutionNode{
			Step:         containerd.NewExtractCNIStep(ctx, extractCniNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{dlCniNode},
		})
		installCniNode := plan.NodeID(fmt.Sprintf("install-cni-%s", hostName))
		p.AddNode(installCniNode, &plan.ExecutionNode{
			Step:         containerd.NewInstallCNIStep(ctx, installCniNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{extractCniNode},
		})

		extractCrictlNode := plan.NodeID(fmt.Sprintf("extract-crictl-%s", hostName))
		p.AddNode(extractCrictlNode, &plan.ExecutionNode{
			Step:         containerd.NewExtractCrictlStep(ctx, extractCrictlNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{dlCrictlNode},
		})
		installCrictlNode := plan.NodeID(fmt.Sprintf("install-crictl-%s", hostName))
		p.AddNode(installCrictlNode, &plan.ExecutionNode{
			Step:         containerd.NewInstallCrictlStep(ctx, installCrictlNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{extractCrictlNode},
		})

		// --- Configuration ---
		binariesReadyDeps := []plan.NodeID{installContainerdNode, installRuncNode, installCniNode, installCrictlNode}

		configureContainerdNode := plan.NodeID(fmt.Sprintf("configure-containerd-%s", hostName))
		p.AddNode(configureContainerdNode, &plan.ExecutionNode{
			Step:         containerd.NewConfigureContainerdStep(ctx, configureContainerdNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: binariesReadyDeps,
		})

		installServiceNode := plan.NodeID(fmt.Sprintf("install-containerd-svc-%s", hostName))
		p.AddNode(installServiceNode, &plan.ExecutionNode{
			Step:         containerd.NewInstallContainerdServiceStep(ctx, installServiceNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{configureContainerdNode},
		})

		configureCrictlNode := plan.NodeID(fmt.Sprintf("configure-crictl-%s", hostName))
		p.AddNode(configureCrictlNode, &plan.ExecutionNode{
			Step:         containerd.NewConfigureCrictlStep(ctx, configureCrictlNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{installServiceNode},
		})

		// --- Service Management ---
		enableServiceNode := plan.NodeID(fmt.Sprintf("enable-containerd-%s", hostName))
		p.AddNode(enableServiceNode, &plan.ExecutionNode{
			Step:         containerd.NewEnableContainerdStep(ctx, enableServiceNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{configureCrictlNode},
		})

		startServiceNode := plan.NodeID(fmt.Sprintf("start-containerd-%s", hostName))
		p.AddNode(startServiceNode, &plan.ExecutionNode{
			Step:         containerd.NewStartContainerdStep(ctx, startServiceNode.String()),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{enableServiceNode},
		})
	}

	return p, nil
}
