package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanKubernetesTask struct {
	task.Base
}

func NewCleanKubernetesTask() task.Task {
	return &CleanKubernetesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanKubernetes",
				Description: "Reset kubeadm and remove all Kubernetes components",
			},
		},
	}
}

func (t *CleanKubernetesTask) Name() string {
	return t.Meta.Name
}

func (t *CleanKubernetesTask) Description() string {
	return t.Meta.Description
}

func (t *CleanKubernetesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanKubernetesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	allHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(allHosts) == 0 {
		ctx.GetLogger().Info("No master or worker hosts found, skipping Kubernetes cleanup task.")
		return fragment, nil
	}

	kubeadmReset := kubeadm.NewKubeadmResetStepBuilder(*runtimeCtx, "KubeadmReset").Build()

	node := &plan.ExecutionNode{Name: "KubeadmReset", Step: kubeadmReset, Hosts: allHosts}

	fragment.AddNode(node)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
