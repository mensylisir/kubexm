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

		// --- Docker Engine Installation ---
		dlDockerNode := plan.NodeID(fmt.Sprintf("download-docker-%s", hostName))
		p.AddNode(dlDockerNode, &plan.ExecutionNode{Step: docker.NewDownloadDockerStep(ctx, dlDockerNode.String()), Hosts: []connector.Host{host}})

		installDockerNode := plan.NodeID(fmt.Sprintf("install-docker-%s", hostName))
		p.AddNode(installDockerNode, &plan.ExecutionNode{Step: docker.NewInstallDockerStep(ctx, installDockerNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{dlDockerNode}})

		cfgDockerNode := plan.NodeID(fmt.Sprintf("configure-docker-%s", hostName))
		p.AddNode(cfgDockerNode, &plan.ExecutionNode{Step: docker.NewConfigureDockerStep(ctx, cfgDockerNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installDockerNode}})

		installDockerSvcNode := plan.NodeID(fmt.Sprintf("install-docker-svc-%s", hostName))
		p.AddNode(installDockerSvcNode, &plan.ExecutionNode{Step: docker.NewInstallDockerServiceStep(ctx, installDockerSvcNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{cfgDockerNode}})

		enableDockerNode := plan.NodeID(fmt.Sprintf("enable-docker-%s", hostName))
		p.AddNode(enableDockerNode, &plan.ExecutionNode{Step: docker.NewEnableDockerStep(ctx, enableDockerNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installDockerSvcNode}})

		startDockerNode := plan.NodeID(fmt.Sprintf("start-docker-%s", hostName))
		p.AddNode(startDockerNode, &plan.ExecutionNode{Step: docker.NewStartDockerStep(ctx, startDockerNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{enableDockerNode}})
		dockerReadyNode := startDockerNode

		// --- CNI Plugins Installation (can run in parallel with Docker install) ---
		dlCniNode := plan.NodeID(fmt.Sprintf("download-cni-for-docker-%s", hostName))
		p.AddNode(dlCniNode, &plan.ExecutionNode{Step: containerd.NewDownloadCNIStep(ctx, dlCniNode.String()), Hosts: []connector.Host{host}})

		installCniNode := plan.NodeID(fmt.Sprintf("install-cni-for-docker-%s", hostName))
		p.AddNode(installCniNode, &plan.ExecutionNode{Step: containerd.NewInstallCNIStep(ctx, installCniNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{dlCniNode}})
		cniReadyNode := installCniNode

		// --- cri-dockerd Installation ---
		dlCriDockerdNode := plan.NodeID(fmt.Sprintf("download-cri-dockerd-%s", hostName))
		p.AddNode(dlCriDockerdNode, &plan.ExecutionNode{Step: docker.NewDownloadCriDockerdStep(ctx, dlCriDockerdNode.String()), Hosts: []connector.Host{host}})

		installCriDockerdNode := plan.NodeID(fmt.Sprintf("install-cri-dockerd-%s", hostName))
		p.AddNode(installCriDockerdNode, &plan.ExecutionNode{Step: docker.NewInstallCriDockerdStep(ctx, installCriDockerdNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{dlCriDockerdNode}})

		installCriDockerdSvcNode := plan.NodeID(fmt.Sprintf("install-cri-dockerd-svc-%s", hostName))
		p.AddNode(installCriDockerdSvcNode, &plan.ExecutionNode{Step: docker.NewInstallCriDockerdServiceStep(ctx, installCriDockerdSvcNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installCriDockerdNode}})

		// cri-dockerd depends on docker and cni
		criDockerdDeps := []plan.NodeID{dockerReadyNode, cniReadyNode, installCriDockerdSvcNode}

		enableCriDockerdNode := plan.NodeID(fmt.Sprintf("enable-cri-dockerd-%s", hostName))
		p.AddNode(enableCriDockerdNode, &plan.ExecutionNode{Step: docker.NewEnableCriDockerdStep(ctx, enableCriDockerdNode.String()), Hosts: []connector.Host{host}, Dependencies: criDockerdDeps})

		startCriDockerdNode := plan.NodeID(fmt.Sprintf("start-cri-dockerd-%s", hostName))
		p.AddNode(startCriDockerdNode, &plan.ExecutionNode{Step: docker.NewStartCriDockerdStep(ctx, startCriDockerdNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{enableCriDockerdNode}})
	}

	return p, nil
}
