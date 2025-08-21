package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/health"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-apiserver"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	kubexmstep "github.com/mensylisir/kubexm/pkg/step/pki/kubexm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type RolloutK8sCABundleTask struct {
	task.Base
}

func NewRolloutK8sCABundleTask() task.Task {
	return &RolloutK8sCABundleTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RolloutK8sCABundle",
				Description: "Distributes the K8s CA bundle to all masters and performs a rolling restart of the control plane",
			},
		},
	}
}

func (t *RolloutK8sCABundleTask) Name() string {
	return t.Meta.Name
}

func (t *RolloutK8sCABundleTask) Description() string {
	return t.Meta.Description
}

func (t *RolloutK8sCABundleTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	var caRequiresRenewal bool
	if val, ok := ctx.GetModuleCache().Get(common.CacheKubexmK8sCACertRenew); ok {
		if renew, isBool := val.(bool); isBool && renew {
			caRequiresRenewal = renew
		}
	}
	return caRequiresRenewal, nil
}

func (t *RolloutK8sCABundleTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found for task %s", t.Name())
	}

	var lastNodeExitPoint plan.NodeID = ""

	for _, host := range masterHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		backupStep := kubexmstep.NewKubexmBackupRemotePKIStepBuilder(*runtimeCtx, fmt.Sprintf("BackupPKIFor%s", hostName)).Build()
		distributeBundleStep := kubexmstep.NewKubexmDistributeK8sPKIStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeBundleFor%s", hostName)).Build()

		restartApiServerStep := kube_apiserver.NewRestartKubeApiServerStepBuilder(*runtimeCtx, fmt.Sprintf("RestartApiServerFor%s", hostName)).Build()
		verifyApiServerStep := health.NewVerifyAPIServerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyApiServer-%s", hostName)).Build()

		restartCMStep := kube_controller_manager.NewRestartKubeControllerManagerStepBuilder(*runtimeCtx, fmt.Sprintf("RestartControllerManagerFor%s", hostName)).Build()
		verifyCMStep := health.NewVerifyControllerManagerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyControllerManagerFor%s", hostName)).Build()

		restartSchedulerStep := kube_scheduler.NewRestartKubeSchedulerStepBuilder(*runtimeCtx, fmt.Sprintf("RestartSchedulerFor%s", hostName)).Build()
		verifySchedulerStep := health.NewVerifySchedulerHealthStepBuilder(*runtimeCtx, fmt.Sprintf("VerifySchedulerFor%s", hostName)).Build()

		backupNode := &plan.ExecutionNode{Name: fmt.Sprintf("BackupPKIFor%s", hostName), Step: backupStep, Hosts: hostList}
		distributeBundleNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeBundleFor%s", hostName), Step: distributeBundleStep, Hosts: hostList}

		restartApiServerNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartApiServerFor%s", hostName), Step: restartApiServerStep, Hosts: hostList}
		verifyApiServerNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyApiServerFor%s", hostName), Step: verifyApiServerStep, Hosts: hostList}

		restartCMNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartControllerManagerFor%s", hostName), Step: restartCMStep, Hosts: hostList}
		verifyCMNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyControllerManagerFor%s", hostName), Step: verifyCMStep, Hosts: hostList}

		restartSchedulerNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartSchedulerFor%s", hostName), Step: restartSchedulerStep, Hosts: hostList}
		verifySchedulerNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifySchedulerFor%s", hostName), Step: verifySchedulerStep, Hosts: hostList}

		backupID, _ := fragment.AddNode(backupNode)
		distributeBundleID, _ := fragment.AddNode(distributeBundleNode)

		restartApiServerID, _ := fragment.AddNode(restartApiServerNode)
		verifyApiServerID, _ := fragment.AddNode(verifyApiServerNode)

		restartCMID, _ := fragment.AddNode(restartCMNode)
		verifyCMID, _ := fragment.AddNode(verifyCMNode)

		restartSchedulerID, _ := fragment.AddNode(restartSchedulerNode)
		verifySchedulerID, _ := fragment.AddNode(verifySchedulerNode)

		fragment.AddDependency(backupID, distributeBundleID)
		fragment.AddDependency(distributeBundleID, restartApiServerID)
		fragment.AddDependency(restartApiServerID, verifyApiServerID)
		fragment.AddDependency(verifyApiServerID, restartCMID)
		fragment.AddDependency(restartCMID, verifyCMID)
		fragment.AddDependency(verifyCMID, restartSchedulerID)
		fragment.AddDependency(restartSchedulerID, verifySchedulerID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, backupID)
		}

		lastNodeExitPoint = verifySchedulerID
	}

	verifyClusterStep := health.NewCheckClusterHealthStepBuilder(*runtimeCtx, "VerifyClusterHealthAfterBundleRollout").Build()
	verifyClusterNode := &plan.ExecutionNode{
		Name:  "VerifyClusterHealthAfterBundleRollout",
		Step:  verifyClusterStep,
		Hosts: []connector.Host{masterHosts[0]},
	}
	verifyClusterID, _ := fragment.AddNode(verifyClusterNode)

	if lastNodeExitPoint != "" {
		fragment.AddDependency(lastNodeExitPoint, verifyClusterID)
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*RolloutK8sCABundleTask)(nil)
