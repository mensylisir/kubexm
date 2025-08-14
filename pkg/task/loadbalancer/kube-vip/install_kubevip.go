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
	if cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled == nil || !*cfg.Spec.ControlPlaneEndpoint.HighAvailability.Enabled ||
		cfg.Spec.ControlPlaneEndpoint.HighAvailability.External.Type != string(common.ExternalLBTypeKubeVIP) {
		return false, nil
	}
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *DeployKubeVipTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	generateVipManifest := kubevipstep.NewGenerateKubeVipManifestStepBuilder(*runtimeCtx, "GenerateKubeVipManifest").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeVipManifestOnAllMasters", Step: generateVipManifest, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
