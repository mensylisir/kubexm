package kubevip

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubevipstep "github.com/mensylisir/kubexm/pkg/step/loadbalancer/kube-vip"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanKubeVipTask struct {
	task.Base
}

func NewCleanKubeVipTask() task.Task {
	return &CleanKubeVipTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanKubeVip",
				Description: "Clean up kube-vip static pod manifest on all master nodes",
			},
		},
	}
}

func (t *CleanKubeVipTask) Name() string {
	return t.Meta.Name
}

func (t *CleanKubeVipTask) Description() string {
	return t.Meta.Description
}

func (t *CleanKubeVipTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubeVIP) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *CleanKubeVipTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	cleanVipManifest := kubevipstep.NewCleanKubeVipManifestStepBuilder(*runtimeCtx, "CleanKubeVipManifest").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanKubeVipManifestOnAllMasters", Step: cleanVipManifest, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
