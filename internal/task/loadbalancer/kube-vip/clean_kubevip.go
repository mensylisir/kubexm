package kubevip

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubevipstep "github.com/mensylisir/kubexm/internal/step/loadbalancer/kube-vip"
	"github.com/mensylisir/kubexm/internal/task"
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
	ha := cfg.Spec.ControlPlaneEndpoint.HighAvailability
	if ha == nil || ha.Enabled == nil || !*ha.Enabled {
		return false, nil
	}
	// Check external kube-vip
	if ha.External != nil && ha.External.Type == string(common.ExternalLBTypeKubeVIP) {
		return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
	}
	// Check internal kube-vip
	if ha.Internal != nil && ha.Internal.Enabled != nil && *ha.Internal.Enabled &&
		ha.Internal.Type == string(common.InternalLBTypeKubeVIP) {
		return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
	}
	return false, nil
}

func (t *CleanKubeVipTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	cleanVipManifest, err := kubevipstep.NewCleanKubeVipManifestStepBuilder(runtimeCtx, "CleanKubeVipManifest").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "CleanKubeVipManifestOnAllMasters", Step: cleanVipManifest, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
