package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-apiserver"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	"github.com/mensylisir/kubexm/pkg/task"
)

type BootstrapControlPlaneWithBinariesTask struct {
	task.Base
}

func NewBootstrapControlPlaneWithBinariesTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &BootstrapControlPlaneWithBinariesTask{
		Base: task.Base{
			Name:   "BootstrapControlPlaneWithBinaries",
			Desc:   "Configure and start all control plane components for a binary installation",
			Hosts:  ctx.GetHostsByRole(common.RoleMaster),
			Action: new(BootstrapControlPlaneWithBinariesAction),
		},
	}
	return s, nil
}

type BootstrapControlPlaneWithBinariesAction struct {
	task.Action
}

func (a *BootstrapControlPlaneWithBinariesAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Bootstrap Control Plane (Binary) Phase")

	masterHosts := a.GetHosts()

	// --- Kube-apiserver ---
	installApiSvcNode := plan.NodeID("install-apiserver-svc")
	p.AddNode(installApiSvcNode, &plan.ExecutionNode{Step: apiserver.NewInstallApiServerServiceStep(ctx, installApiSvcNode.String()), Hosts: masterHosts})

	enableApiSvcNode := plan.NodeID("enable-apiserver")
	p.AddNode(enableApiSvcNode, &plan.ExecutionNode{Step: apiserver.NewEnableApiServerStep(ctx, enableApiSvcNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{installApiSvcNode}})

	startApiSvcNode := plan.NodeID("start-apiserver")
	p.AddNode(startApiSvcNode, &plan.ExecutionNode{Step: apiserver.NewStartApiServerStep(ctx, startApiSvcNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{enableApiSvcNode}})

	checkApiHealthNode := plan.NodeID("check-apiserver-health")
	p.AddNode(checkApiHealthNode, &plan.ExecutionNode{Step: apiserver.NewCheckApiServerHealthStep(ctx, checkApiHealthNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{startApiSvcNode}})

	// --- Kube-controller-manager ---
	installCMServiceNode := plan.NodeID("install-cm-svc")
	p.AddNode(installCMServiceNode, &plan.ExecutionNode{Step: controllermanager.NewInstallControllerManagerServiceStep(ctx, installCMServiceNode.String()), Hosts: masterHosts})

	enableCMNode := plan.NodeID("enable-cm")
	p.AddNode(enableCMNode, &plan.ExecutionNode{Step: controllermanager.NewEnableControllerManagerStep(ctx, enableCMNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{installCMServiceNode, checkApiHealthNode}})

	startCMNode := plan.NodeID("start-cm")
	p.AddNode(startCMNode, &plan.ExecutionNode{Step: controllermanager.NewStartControllerManagerStep(ctx, startCMNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{enableCMNode}})

	// --- Kube-scheduler ---
	installSchedulerSvcNode := plan.NodeID("install-scheduler-svc")
	p.AddNode(installSchedulerSvcNode, &plan.ExecutionNode{Step: scheduler.NewInstallSchedulerServiceStep(ctx, installSchedulerSvcNode.String()), Hosts: masterHosts})

	enableSchedulerNode := plan.NodeID("enable-scheduler")
	p.AddNode(enableSchedulerNode, &plan.ExecutionNode{Step: scheduler.NewEnableSchedulerStep(ctx, enableSchedulerNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{installSchedulerSvcNode, checkApiHealthNode}})

	startSchedulerNode := plan.NodeID("start-scheduler")
	p.AddNode(startSchedulerNode, &plan.ExecutionNode{Step: scheduler.NewStartSchedulerStep(ctx, startSchedulerNode.String()), Hosts: masterHosts, Dependencies: []plan.NodeID{enableSchedulerNode}})

	return p, nil
}
