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
	installApiSvc := apiserver.NewInstallApiServerServiceStep(ctx, "InstallApiServerService")
	p.AddNode("install-apiserver-svc", &plan.ExecutionNode{Step: installApiSvc, Hosts: masterHosts})

	enableApiSvc := apiserver.NewEnableApiServerStep(ctx, "EnableApiServer")
	p.AddNode("enable-apiserver", &plan.ExecutionNode{Step: enableApiSvc, Hosts: masterHosts, Dependencies: []plan.NodeID{"install-apiserver-svc"}})

	startApiSvc := apiserver.NewStartApiServerStep(ctx, "StartApiServer")
	p.AddNode("start-apiserver", &plan.ExecutionNode{Step: startApiSvc, Hosts: masterHosts, Dependencies: []plan.NodeID{"enable-apiserver"}})

	checkApiHealth := apiserver.NewCheckApiServerHealthStep(ctx, "CheckApiServerHealth")
	p.AddNode("check-apiserver-health", &plan.ExecutionNode{Step: checkApiHealth, Hosts: masterHosts, Dependencies: []plan.NodeID{"start-apiserver"}})
	apiServerReadyNode := plan.NodeID("check-apiserver-health")

	// --- Kube-controller-manager ---
	installCMService := controllermanager.NewInstallControllerManagerServiceStep(ctx, "InstallControllerManagerService")
	p.AddNode("install-cm-svc", &plan.ExecutionNode{Step: installCMService, Hosts: masterHosts})

	enableCM := controllermanager.NewEnableControllerManagerStep(ctx, "EnableControllerManager")
	p.AddNode("enable-cm", &plan.ExecutionNode{Step: enableCM, Hosts: masterHosts, Dependencies: []plan.NodeID{"install-cm-svc", apiServerReadyNode}})

	startCM := controllermanager.NewStartControllerManagerStep(ctx, "StartControllerManager")
	p.AddNode("start-cm", &plan.ExecutionNode{Step: startCM, Hosts: masterHosts, Dependencies: []plan.NodeID{"enable-cm"}})

	// --- Kube-scheduler ---
	installSchedulerSvc := scheduler.NewInstallSchedulerServiceStep(ctx, "InstallSchedulerService")
	p.AddNode("install-scheduler-svc", &plan.ExecutionNode{Step: installSchedulerSvc, Hosts: masterHosts})

	enableScheduler := scheduler.NewEnableSchedulerStep(ctx, "EnableScheduler")
	p.AddNode("enable-scheduler", &plan.ExecutionNode{Step: enableScheduler, Hosts: masterHosts, Dependencies: []plan.NodeID{"install-scheduler-svc", apiServerReadyNode}})

	startScheduler := scheduler.NewStartSchedulerStep(ctx, "StartScheduler")
	p.AddNode("start-scheduler", &plan.ExecutionNode{Step: startScheduler, Hosts: masterHosts, Dependencies: []plan.NodeID{"enable-scheduler"}})

	return p, nil
}
