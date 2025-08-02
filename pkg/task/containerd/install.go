package containerd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/containerd"
	"github.comcom/mensylisir/kubexm/pkg/task"
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

	// For each host, create a chain of steps to install containerd.
	for _, host := range hosts {
		hostName := host.GetName()

		// 1. Download steps (can run in parallel)
		downloadContainerd := &plan.ExecutionNode{
			Step:  containerd.NewDownloadContainerdStep(ctx, fmt.Sprintf("DownloadContainerd-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		downloadRunc := &plan.ExecutionNode{
			Step:  containerd.NewDownloadRuncStep(ctx, fmt.Sprintf("DownloadRunc-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		downloadCni := &plan.ExecutionNode{
			Step:  containerd.NewDownloadCNIStep(ctx, fmt.Sprintf("DownloadCNI-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		downloadCrictl := &plan.ExecutionNode{
			Step:  containerd.NewDownloadCrictlStep(ctx, fmt.Sprintf("DownloadCrictl-%s", hostName)),
			Hosts: []connector.Host{host},
		}

		p.AddNode(plan.NodeID(downloadContainerd.Step.Meta().Name), downloadContainerd)
		p.AddNode(plan.NodeID(downloadRunc.Step.Meta().Name), downloadRunc)
		p.AddNode(plan.NodeID(downloadCni.Step.Meta().Name), downloadCni)
		p.AddNode(plan.NodeID(downloadCrictl.Step.Meta().Name), downloadCrictl)

		// 2. Extract and Install steps
		extractContainerd := &plan.ExecutionNode{
			Step:  containerd.NewExtractContainerdStep(ctx, fmt.Sprintf("ExtractContainerd-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		p.AddNode(plan.NodeID(extractContainerd.Step.Meta().Name), extractContainerd)
		p.AddDependency(downloadContainerd.Step.Meta().Name, extractContainerd.Step.Meta().Name)

		installRunc := &plan.ExecutionNode{
			Step:  containerd.NewInstallRuncStep(ctx, fmt.Sprintf("InstallRunc-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		p.AddNode(plan.NodeID(installRunc.Step.Meta().Name), installRunc)
		p.AddDependency(downloadRunc.Step.Meta().Name, installRunc.Step.Meta().Name)

		extractCni := &plan.ExecutionNode{
			Step:  containerd.NewExtractCNIStep(ctx, fmt.Sprintf("ExtractCNI-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		p.AddNode(plan.NodeID(extractCni.Step.Meta().Name), extractCni)
		p.AddDependency(downloadCni.Step.Meta().Name, extractCni.Step.Meta().Name)

		extractCrictl := &plan.ExecutionNode{
			Step:  containerd.NewExtractCrictlStep(ctx, fmt.Sprintf("ExtractCrictl-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		p.AddNode(plan.NodeID(extractCrictl.Step.Meta().Name), extractCrictl)
		p.AddDependency(downloadCrictl.Step.Meta().Name, extractCrictl.Step.Meta().Name)

		installContainerd := &plan.ExecutionNode{
			Step:  containerd.NewInstallContainerdStep(ctx, fmt.Sprintf("InstallContainerd-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		p.AddNode(plan.NodeID(installContainerd.Step.Meta().Name), installContainerd)
		p.AddDependency(extractContainerd.Step.Meta().Name, installContainerd.Step.Meta().Name)

		installCni := &plan.ExecutionNode{
			Step:  containerd.NewInstallCNIStep(ctx, fmt.Sprintf("InstallCNI-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		p.AddNode(plan.NodeID(installCni.Step.Meta().Name), installCni)
		p.AddDependency(extractCni.Step.Meta().Name, installCni.Step.Meta().Name)

		installCrictl := &plan.ExecutionNode{
			Step:  containerd.NewInstallCrictlStep(ctx, fmt.Sprintf("InstallCrictl-%s", hostName)),
			Hosts: []connector.Host{host},
		}
		p.AddNode(plan.NodeID(installCrictl.Step.Meta().Name), installCrictl)
		p.AddDependency(extractCrictl.Step.Meta().Name, installCrictl.Step.Meta().Name)

		// 3. Configure steps
		depsForConfig := []plan.NodeID{
			plan.NodeID(installContainerd.Step.Meta().Name),
			plan.NodeID(installRunc.Step.Meta().Name),
			plan.NodeID(installCni.Step.Meta().Name),
			plan.NodeID(installCrictl.Step.Meta().Name),
		}

		configureContainerd := &plan.ExecutionNode{
			Step:         containerd.NewConfigureContainerdStep(ctx, fmt.Sprintf("ConfigureContainerd-%s", hostName)),
			Hosts:        []connector.Host{host},
			Dependencies: depsForConfig,
		}
		p.AddNode(plan.NodeID(configureContainerd.Step.Meta().Name), configureContainerd)

		installService := &plan.ExecutionNode{
			Step:         containerd.NewInstallContainerdServiceStep(ctx, fmt.Sprintf("InstallContainerdService-%s", hostName)),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{plan.NodeID(configureContainerd.Step.Meta().Name)},
		}
		p.AddNode(plan.NodeID(installService.Step.Meta().Name), installService)

		configureCrictl := &plan.ExecutionNode{
			Step:         containerd.NewConfigureCrictlStep(ctx, fmt.Sprintf("ConfigureCrictl-%s", hostName)),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{plan.NodeID(installService.Step.Meta().Name)},
		}
		p.AddNode(plan.NodeID(configureCrictl.Step.Meta().Name), configureCrictl)

		// 4. Service management steps
		enableService := &plan.ExecutionNode{
			Step:         containerd.NewEnableContainerdStep(ctx, fmt.Sprintf("EnableContainerd-%s", hostName)),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{plan.NodeID(configureCrictl.Step.Meta().Name)},
		}
		p.AddNode(plan.NodeID(enableService.Step.Meta().Name), enableService)

		startService := &plan.ExecutionNode{
			Step:         containerd.NewStartContainerdStep(ctx, fmt.Sprintf("StartContainerd-%s", hostName)),
			Hosts:        []connector.Host{host},
			Dependencies: []plan.NodeID{plan.NodeID(enableService.Step.Meta().Name)},
		}
		p.AddNode(plan.NodeID(startService.Step.Meta().Name), startService)
	}

	return p, nil
}
