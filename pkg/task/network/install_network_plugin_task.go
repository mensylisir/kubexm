package network

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/network/calico"
	"github.com/mensylisir/kubexm/pkg/task/network/cilium"
	"github.com/mensylisir/kubexm/pkg/task/network/flannel"
	"github.com/mensylisir/kubexm/pkg/task/network/hybridnet"
	"github.com/mensylisir/kubexm/pkg/task/network/kubeovn"
	"github.com/mensylisir/kubexm/pkg/task/network/multus"
)

// InstallNetworkPluginTask is a dispatcher task that chooses the correct CNI installation task.
type InstallNetworkPluginTask struct {
	task.BaseTask
}

// NewInstallNetworkPluginTask creates a new InstallNetworkPluginTask.
func NewInstallNetworkPluginTask() task.Task {
	return &InstallNetworkPluginTask{
		BaseTask: task.NewBaseTask(
			"InstallNetworkPlugin",
			"Deploys the CNI network plugin to the cluster.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallNetworkPluginTask) IsRequired(ctx task.TaskContext) (bool, error) {
	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Network == nil || clusterCfg.Spec.Network.Plugin == "" {
		ctx.GetLogger().Info("No CNI plugin specified, InstallNetworkPluginTask is not required.")
		return false, nil
	}
	return true, nil
}

// Plan is a dispatcher that selects the appropriate CNI-specific task.
func (t *InstallNetworkPluginTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	pluginName := ctx.GetClusterConfig().Spec.Network.Plugin

	var cniTask task.Task

	switch pluginName {
	case common.CNITypeCalico:
		cniTask = calico.NewInstallCalicoTask()
	case common.CNITypeCilium:
		cniTask = cilium.NewInstallCiliumTask()
	case common.CNITypeFlannel:
		cniTask = flannel.NewInstallFlannelTask()
	case common.CNITypeHybridnet:
		cniTask = hybridnet.NewInstallHybridnetTask()
	case common.CNITypeKubeOVN:
		cniTask = kubeovn.NewInstallKubeOVNTask()
	case common.CNITypeMultus:
		cniTask = multus.NewInstallMultusTask()
	default:
		return nil, fmt.Errorf("unsupported CNI plugin '%s' for task %s", pluginName, t.Name())
	}

	logger.Info("Dispatching to CNI-specific installation task.", "plugin", pluginName)
	return cniTask.Plan(ctx)
}

var _ task.Task = (*InstallNetworkPluginTask)(nil)
