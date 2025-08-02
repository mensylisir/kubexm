package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
)

type BootstrapClusterWithKubeadmTask struct {
	task.Base
}

func NewBootstrapClusterWithKubeadmTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &BootstrapClusterWithKubeadmTask{
		Base: task.Base{
			Name:   "BootstrapClusterWithKubeadm",
			Desc:   "Bootstrap the Kubernetes cluster using kubeadm init and join",
			Hosts:  ctx.GetHosts(),
			Action: new(BootstrapClusterWithKubeadmAction),
		},
	}
	return s, nil
}

type BootstrapClusterWithKubeadmAction struct {
	task.Action
}

func (a *BootstrapClusterWithKubeadmAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Bootstrap Cluster with Kubeadm Phase")

	masterNodes := ctx.GetHostsByRole(common.RoleMaster)
	workerNodes := ctx.GetHostsByRole(common.RoleWorker)

	if len(masterNodes) == 0 {
		return nil, fmt.Errorf("at least one master node is required for this task")
	}

	firstMaster := masterNodes[0]
	otherMasters := masterNodes[1:]

	// 1. Initialize the first master
	genFirstMasterCfg := kubeadm.NewGenerateFirstMasterConfigStep(ctx, "GenerateFirstMasterConfig")
	p.AddNode("gen-first-master-cfg", &plan.ExecutionNode{Step: genFirstMasterCfg, Hosts: []connector.Host{firstMaster}})

	initFirstMaster := kubeadm.NewInitFirstMasterStep(ctx, "InitFirstMaster")
	p.AddNode("init-first-master", &plan.ExecutionNode{Step: initFirstMaster, Hosts: []connector.Host{firstMaster}, Dependencies: []plan.NodeID{"gen-first-master-cfg"}})

	// Kubelet must be started after init
	enableKubeletFirstMaster := kubelet.NewEnableKubeletServiceStep(ctx, "EnableKubeletOnFirstMaster")
	p.AddNode("enable-kubelet-first-master", &plan.ExecutionNode{Step: enableKubeletFirstMaster, Hosts: []connector.Host{firstMaster}, Dependencies: []plan.NodeID{"init-first-master"}})

	startKubeletFirstMaster := kubelet.NewStartKubeletStep(ctx, "StartKubeletOnFirstMaster")
	p.AddNode("start-kubelet-first-master", &plan.ExecutionNode{Step: startKubeletFirstMaster, Hosts: []connector.Host{firstMaster}, Dependencies: []plan.NodeID{"enable-kubelet-first-master"}})

	firstMasterReadyNode := plan.NodeID("start-kubelet-first-master")

	// 2. Join other masters (if any)
	if len(otherMasters) > 0 {
		genOtherMasterCfg := kubeadm.NewGenerateOtherMasterConfigStep(ctx, "GenerateOtherMasterConfig")
		p.AddNode("gen-other-master-cfg", &plan.ExecutionNode{Step: genOtherMasterCfg, Hosts: otherMasters, Dependencies: []plan.NodeID{firstMasterReadyNode}})

		joinMasters := kubeadm.NewJoinOtherMastersStep(ctx, "JoinOtherMasters")
		p.AddNode("join-other-masters", &plan.ExecutionNode{Step: joinMasters, Hosts: otherMasters, Dependencies: []plan.NodeID{"gen-other-master-cfg"}})
	}

	// 3. Join worker nodes (if any)
	if len(workerNodes) > 0 {
		genWorkerCfg := kubeadm.NewGenerateWorkerConfigStep(ctx, "GenerateWorkerConfig")
		p.AddNode("gen-worker-cfg", &plan.ExecutionNode{Step: genWorkerCfg, Hosts: workerNodes, Dependencies: []plan.NodeID{firstMasterReadyNode}})

		joinWorkers := kubeadm.NewJoinWorkersStep(ctx, "JoinWorkers")
		p.AddNode("join-workers", &plan.ExecutionNode{Step: joinWorkers, Hosts: workerNodes, Dependencies: []plan.NodeID{"gen-worker-cfg"}})
	}

	return p, nil
}
