package network

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/task"
	"github.com/mensylisir/kubexm/internal/task/network/calico"
	"github.com/mensylisir/kubexm/internal/task/network/flannel"
)

type InstallNetworkPluginTask struct {
	task.Base
}

func NewInstallNetworkPluginTask() task.Task {
	return &InstallNetworkPluginTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallNetworkPlugin",
				Description: "Install the configured CNI network plugin",
			},
		},
	}
}

func (t *InstallNetworkPluginTask) Name() string {
	return t.Meta.Name
}

func (t *InstallNetworkPluginTask) Description() string {
	return t.Meta.Description
}

func (t *InstallNetworkPluginTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	plugin := ctx.GetClusterConfig().Spec.Network.Plugin
	return plugin != "", nil
}

func (t *InstallNetworkPluginTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	plugin := ctx.GetClusterConfig().Spec.Network.Plugin

	var subTask task.Task
	switch plugin {
	case string(common.CNITypeCalico):
		subTask = calico.NewDeployCalicoTask()
	case string(common.CNITypeFlannel):
		subTask = flannel.NewDeployFlannelTask()
	default:
		ctx.GetLogger().Infof("No supported network plugin configured or plugin '%s' not implemented yet.", plugin)
		return plan.NewEmptyFragment(t.Name()), nil
	}

	return subTask.Plan(ctx)
}
