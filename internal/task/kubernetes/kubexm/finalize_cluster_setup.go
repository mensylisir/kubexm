package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeconfigstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubeconfig"
	kubectlstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubectl"
	rbacstep "github.com/mensylisir/kubexm/internal/step/kubernetes/rbac"
	"github.com/mensylisir/kubexm/internal/task"
)

type FinalizeClusterSetupTask struct {
	task.Base
}

func NewFinalizeClusterSetupTask() task.Task {
	return &FinalizeClusterSetupTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "FinalizeClusterSetup",
				Description: "Apply essential RBAC rules and configure kubectl for user access",
			},
		},
	}
}

func (t *FinalizeClusterSetupTask) Name() string {
	return t.Meta.Name
}

func (t *FinalizeClusterSetupTask) Description() string {
	return t.Meta.Description
}

func (t *FinalizeClusterSetupTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *FinalizeClusterSetupTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to finalize cluster setup")
	}
	executionHost := masterHosts[0]

	allHosts := ctx.GetHostsByRole("")

	applyRBAC, err := rbacstep.NewApplyEssentialRBACStepBuilder(runtimeCtx, "ApplyEssentialRBAC").Build()
	if err != nil {
		return nil, err
	}
	installKubectl, err := kubectlstep.NewInstallKubectlStepBuilder(runtimeCtx, "InstallKubectlOnAllNodes").Build()
	if err != nil {
		return nil, err
	}
	configureKubectl, err := kubectlstep.NewConfigureKubectlStepBuilder(runtimeCtx, "ConfigureKubectlForRoot").Build()
	if err != nil {
		return nil, err
	}
	copyKubeconfig, err := kubeconfigstep.NewCopyKubeconfigStepBuilder(runtimeCtx, "CopyKubeconfigToUser").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "ApplyEssentialRBAC", Step: applyRBAC, Hosts: []remotefw.Host{executionHost}})

	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubectlOnAllNodes", Step: installKubectl, Hosts: allHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubectlForRoot", Step: configureKubectl, Hosts: allHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "CopyKubeconfigToUser", Step: copyKubeconfig, Hosts: allHosts})

	fragment.AddDependency("InstallKubectlOnAllNodes", "ApplyEssentialRBAC")

	fragment.AddDependency("InstallKubectlOnAllNodes", "ConfigureKubectlForRoot")
	fragment.AddDependency("InstallKubectlOnAllNodes", "CopyKubeconfigToUser")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
