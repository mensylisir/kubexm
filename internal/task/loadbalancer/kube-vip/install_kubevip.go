package kubevip

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubevipstep "github.com/mensylisir/kubexm/internal/step/loadbalancer/kube-vip"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployKubeVipTask struct {
	task.Base
}

func NewDeployKubeVipTask() task.Task {
	return &DeployKubeVipTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployKubeVip",
				Description: "Deploy kube-vip as a static pod on all master nodes for control plane HA",
			},
		},
	}
}

func (t *DeployKubeVipTask) Name() string {
	return t.Meta.Name
}

func (t *DeployKubeVipTask) Description() string {
	return t.Meta.Description
}

func (t *DeployKubeVipTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability == nil ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil ||
		!*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled {
		return false, nil
	}
	ha := cfg.Spec.ControlPlaneEndpoint.HighAvailability
	if ha.External != nil && ha.External.Enabled != nil && *ha.External.Enabled &&
		ha.External.Type == string(common.ExternalLBTypeKubeVIP) {
		return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
	}
	if ha.Internal != nil && ha.Internal.Enabled != nil && *ha.Internal.Enabled &&
		ha.Internal.Type == string(common.InternalLBTypeKubeVIP) {
		return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
	}
	return false, nil
}

func (t *DeployKubeVipTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	generateVipManifest, err := kubevipstep.NewGenerateKubeVipManifestStepBuilder(runtimeCtx, "GenerateKubeVipManifest").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeVipManifestOnAllMasters", Step: generateVipManifest, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
