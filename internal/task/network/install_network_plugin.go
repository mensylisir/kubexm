package network

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/task"
	"github.com/mensylisir/kubexm/internal/task/network/calico"
	"github.com/mensylisir/kubexm/internal/task/network/cilium"
	"github.com/mensylisir/kubexm/internal/task/network/flannel"
	"github.com/mensylisir/kubexm/internal/task/network/hybridnet"
	"github.com/mensylisir/kubexm/internal/task/network/kubeovn"
	"github.com/mensylisir/kubexm/internal/task/network/multus"
)

// supportedCNIs lists all CNI types that have install tasks implemented.
var supportedCNIs = []string{
	string(common.CNITypeCalico),
	string(common.CNITypeFlannel),
	string(common.CNITypeCilium),
	string(common.CNITypeKubeOvn),
	string(common.CNITypeHybridnet),
	string(common.CNITypeMultus),
}

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
	case string(common.CNITypeCilium):
		subTask = cilium.NewDeployCiliumTask()
	case string(common.CNITypeKubeOvn):
		subTask = kubeovn.NewDeployKubeovnTask()
	case string(common.CNITypeHybridnet):
		subTask = hybridnet.NewDeployHybridnetTask()
	case string(common.CNITypeMultus):
		subTask = multus.NewDeployMultusTask()
	default:
		return nil, fmt.Errorf("unsupported CNI plugin '%s': supported plugins are %v", plugin, supportedCNIs)
	}

	return subTask.Plan(ctx)
}
