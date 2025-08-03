package kubeadm

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

	genFirstMasterCfgNode := plan.NodeID("gen-first-master-cfg")
	p.AddNode(genFirstMasterCfgNode, &plan.ExecutionNode{Step: kubeadm.NewGenerateFirstMasterConfigStep(ctx, genFirstMasterCfgNode.String()), Hosts: []connector.Host{firstMaster}})

	initFirstMasterNode := plan.NodeID("init-first-master")
	p.AddNode(initFirstMasterNode, &plan.ExecutionNode{Step: kubeadm.NewInitFirstMasterStep(ctx, initFirstMasterNode.String()), Hosts: []connector.Host{firstMaster}, Dependencies: []plan.NodeID{genFirstMasterCfgNode}})

	enableKubeletNode := plan.NodeID("enable-kubelet-first-master")
	p.AddNode(enableKubeletNode, &plan.ExecutionNode{Step: kubelet.NewEnableKubeletServiceStep(ctx, enableKubeletNode.String()), Hosts: []connector.Host{firstMaster}, Dependencies: []plan.NodeID{initFirstMasterNode}})

	startKubeletNode := plan.NodeID("start-kubelet-first-master")
	p.AddNode(startKubeletNode, &plan.ExecutionNode{Step: kubelet.NewStartKubeletStep(ctx, startKubeletNode.String()), Hosts: []connector.Host{firstMaster}, Dependencies: []plan.NodeID{enableKubeletNode}})

	firstMasterReadyNode := startKubeletNode

	if len(otherMasters) > 0 {
		genOtherMasterCfgNode := plan.NodeID("gen-other-master-cfg")
		p.AddNode(genOtherMasterCfgNode, &plan.ExecutionNode{Step: kubeadm.NewGenerateOtherMasterConfigStep(ctx, genOtherMasterCfgNode.String()), Hosts: otherMasters, Dependencies: []plan.NodeID{firstMasterReadyNode}})

		joinMastersNode := plan.NodeID("join-other-masters")
		p.AddNode(joinMastersNode, &plan.ExecutionNode{Step: kubeadm.NewJoinOtherMastersStep(ctx, joinMastersNode.String()), Hosts: otherMasters, Dependencies: []plan.NodeID{genOtherMasterCfgNode}})
	}

	if len(workerNodes) > 0 {
		genWorkerCfgNode := plan.NodeID("gen-worker-cfg")
		p.AddNode(genWorkerCfgNode, &plan.ExecutionNode{Step: kubeadm.NewGenerateWorkerConfigStep(ctx, genWorkerCfgNode.String()), Hosts: workerNodes, Dependencies: []plan.NodeID{firstMasterReadyNode}})

		joinWorkersNode := plan.NodeID("join-workers")
		p.AddNode(joinWorkersNode, &plan.ExecutionNode{Step: kubeadm.NewJoinWorkersStep(ctx, joinWorkersNode.String()), Hosts: workerNodes, Dependencies: []plan.NodeID{genWorkerCfgNode}})
	}

	return p, nil
}
