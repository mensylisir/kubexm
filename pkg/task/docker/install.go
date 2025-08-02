package docker

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/containerd"
	"github.com/mensylisir/kubexm/pkg/step/docker"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallDockerTask installs Docker and cri-dockerd.
type InstallDockerTask struct {
	task.Base
}

// NewInstallDockerTask creates a new task for installing Docker and cri-dockerd.
func NewInstallDockerTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallDockerTask{
		Base: task.Base{
			Name:   "InstallDocker",
			Desc:   "Install and configure Docker and cri-dockerd on all nodes",
			Hosts:  ctx.GetHosts(),
			Action: new(InstallDockerAction),
		},
	}
	return s, nil
}

type InstallDockerAction struct {
	task.Action
}

func (a *InstallDockerAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install and Configure Docker Phase")

	hosts := a.GetHosts()
	if len(hosts) == 0 {
		return p, nil
	}

	for _, host := range hosts {
		hostName := host.GetName()

		// --- Docker Engine Installation Chain ---
		downloadDocker := docker.NewDownloadDockerStep(ctx, fmt.Sprintf("DownloadDocker-%s", hostName))
		p.AddNode(plan.NodeID(downloadDocker.Meta().Name), &plan.ExecutionNode{Step: downloadDocker, Hosts: []connector.Host{host}})

		installDocker := docker.NewInstallDockerStep(ctx, fmt.Sprintf("InstallDocker-%s", hostName))
		p.AddNode(plan.NodeID(installDocker.Meta().Name), &plan.ExecutionNode{Step: installDocker, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(downloadDocker.Meta().Name)}})

		configureDocker := docker.NewConfigureDockerStep(ctx, fmt.Sprintf("ConfigureDocker-%s", hostName))
		p.AddNode(plan.NodeID(configureDocker.Meta().Name), &plan.ExecutionNode{Step: configureDocker, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(installDocker.Meta().Name)}})

		installDockerSvc := docker.NewInstallDockerServiceStep(ctx, fmt.Sprintf("InstallDockerSvc-%s", hostName))
		p.AddNode(plan.NodeID(installDockerSvc.Meta().Name), &plan.ExecutionNode{Step: installDockerSvc, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(configureDocker.Meta().Name)}})

		enableDocker := docker.NewEnableDockerStep(ctx, fmt.Sprintf("EnableDocker-%s", hostName))
		p.AddNode(plan.NodeID(enableDocker.Meta().Name), &plan.ExecutionNode{Step: enableDocker, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(installDockerSvc.Meta().Name)}})

		startDocker := docker.NewStartDockerStep(ctx, fmt.Sprintf("StartDocker-%s", hostName))
		p.AddNode(plan.NodeID(startDocker.Meta().Name), &plan.ExecutionNode{Step: startDocker, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(enableDocker.Meta().Name)}})
		dockerReadyNode := plan.NodeID(startDocker.Meta().Name)

		// --- CNI Plugins Installation Chain (in parallel) ---
		downloadCni := containerd.NewDownloadCNIStep(ctx, fmt.Sprintf("DownloadCNI-%s", hostName))
		p.AddNode(plan.NodeID(downloadCni.Meta().Name), &plan.ExecutionNode{Step: downloadCni, Hosts: []connector.Host{host}})

		extractCni := containerd.NewExtractCNIStep(ctx, fmt.Sprintf("ExtractCNI-%s", hostName))
		p.AddNode(plan.NodeID(extractCni.Meta().Name), &plan.ExecutionNode{Step: extractCni, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(downloadCni.Meta().Name)}})

		installCni := containerd.NewInstallCNIStep(ctx, fmt.Sprintf("InstallCNI-%s", hostName))
		p.AddNode(plan.NodeID(installCni.Meta().Name), &plan.ExecutionNode{Step: installCni, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(extractCni.Meta().Name)}})
		cniReadyNode := plan.NodeID(installCni.Meta().Name)

		// --- cri-dockerd Installation Chain ---
		downloadCriDockerd := docker.NewDownloadCriDockerdStep(ctx, fmt.Sprintf("DownloadCriDockerd-%s", hostName))
		p.AddNode(plan.NodeID(downloadCriDockerd.Meta().Name), &plan.ExecutionNode{Step: downloadCriDockerd, Hosts: []connector.Host{host}})

		extractCriDockerd := docker.NewExtractCriDockerdStep(ctx, fmt.Sprintf("ExtractCriDockerd-%s", hostName))
		p.AddNode(plan.NodeID(extractCriDockerd.Meta().Name), &plan.ExecutionNode{Step: extractCriDockerd, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(downloadCriDockerd.Meta().Name)}})

		installCriDockerd := docker.NewInstallCriDockerdStep(ctx, fmt.Sprintf("InstallCriDockerd-%s", hostName))
		p.AddNode(plan.NodeID(installCriDockerd.Meta().Name), &plan.ExecutionNode{Step: installCriDockerd, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(extractCriDockerd.Meta().Name)}})

		installCriDockerdSvc := docker.NewInstallCriDockerdServiceStep(ctx, fmt.Sprintf("InstallCriDockerdSvc-%s", hostName))
		p.AddNode(plan.NodeID(installCriDockerdSvc.Meta().Name), &plan.ExecutionNode{Step: installCriDockerdSvc, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(installCriDockerd.Meta().Name)}})

		// cri-dockerd needs docker to be running and CNI to be installed
		criDockerdDeps := []plan.NodeID{dockerReadyNode, cniReadyNode, plan.NodeID(installCriDockerdSvc.Meta().Name)}

		enableCriDockerd := docker.NewEnableCriDockerdStep(ctx, fmt.Sprintf("EnableCriDockerd-%s", hostName))
		p.AddNode(plan.NodeID(enableCriDockerd.Meta().Name), &plan.ExecutionNode{Step: enableCriDockerd, Hosts: []connector.Host{host}, Dependencies: criDockerdDeps})

		startCriDockerd := docker.NewStartCriDockerdStep(ctx, fmt.Sprintf("StartCriDockerd-%s", hostName))
		p.AddNode(plan.NodeID(startCriDockerd.Meta().Name), &plan.ExecutionNode{Step: startCriDockerd, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{plan.NodeID(enableCriDockerd.Meta().Name)}})
	}

	return p, nil
}
