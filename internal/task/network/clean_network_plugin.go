package network

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/network/calico"
	"github.com/mensylisir/kubexm/internal/step/network/cilium"
	"github.com/mensylisir/kubexm/internal/step/network/flannel"
	"github.com/mensylisir/kubexm/internal/step/network/hybridnet"
	"github.com/mensylisir/kubexm/internal/step/network/kubeovn"
	"github.com/mensylisir/kubexm/internal/step/network/multus"
	"github.com/mensylisir/kubexm/internal/task"
)

// CleanNetworkPluginTask uninstalls the configured CNI network plugin.
// It directly composes Steps based on the configured plugin type,
// instead of delegating to sub-Tasks, adhering to the architecture constraint
// that "Task layer only composes Steps, not other Tasks".
type CleanNetworkPluginTask struct {
	task.Base
}

func NewCleanNetworkPluginTask() task.Task {
	return &CleanNetworkPluginTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanNetworkPlugin",
				Description: "Uninstall the configured CNI network plugin",
			},
		},
	}
}

func (t *CleanNetworkPluginTask) Name() string {
	return t.Meta.Name
}

func (t *CleanNetworkPluginTask) Description() string {
	return t.Meta.Description
}

func (t *CleanNetworkPluginTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	plugin := ctx.GetClusterConfig().Spec.Network.Plugin
	return plugin != "", nil
}

func (t *CleanNetworkPluginTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	plugin := ctx.GetClusterConfig().Spec.Network.Plugin
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to clean network plugin")
	}
	masterHostList := []remotefw.Host{masterHosts[0]}

	switch plugin {
	case string(common.CNITypeCalico):
		return t.planCalico(runtimeCtx, fragment, masterHosts)
	case string(common.CNITypeFlannel):
		return t.planFlannel(runtimeCtx, fragment, masterHostList)
	case string(common.CNITypeCilium):
		return t.planCilium(runtimeCtx, fragment, masterHostList)
	case string(common.CNITypeKubeOvn):
		return t.planKubeovn(runtimeCtx, fragment, masterHostList)
	case string(common.CNITypeHybridnet):
		return t.planHybridnet(runtimeCtx, fragment, masterHostList)
	case string(common.CNITypeMultus):
		return t.planMultus(runtimeCtx, fragment, masterHostList)
	default:
		return nil, fmt.Errorf("unsupported CNI plugin '%s' for cleanup: supported plugins are %v", plugin, supportedCNIs)
	}
}

func (t *CleanNetworkPluginTask) planCalico(ctx *runtime.Context, fragment *plan.ExecutionFragment, masterHosts []remotefw.Host) (*plan.ExecutionFragment, error) {
	removeCalico, err := calico.NewCleanCalicoStepBuilder(ctx, "RemoveCalico").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create RemoveCalico step: %w", err)
	}
	removeCalicoctl, err := calico.NewRemoveCalicoctlStepBuilder(ctx, "RemoveCalicoctl").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create RemoveCalicoctl step: %w", err)
	}

	removeCalicoNode, _ := fragment.AddNode(&plan.ExecutionNode{Name: "RemoveCalico", Step: removeCalico, Hosts: masterHosts})
	removeCtlNode, _ := fragment.AddNode(&plan.ExecutionNode{Name: "RemoveCalicoctl", Step: removeCalicoctl, Hosts: masterHosts})

	fragment.AddDependency(removeCalicoNode, removeCtlNode)

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

func (t *CleanNetworkPluginTask) planFlannel(ctx *runtime.Context, fragment *plan.ExecutionFragment, masterHostList []remotefw.Host) (*plan.ExecutionFragment, error) {
	cleanFlannel, err := flannel.NewCleanFlannelNodeFilesStepBuilder(ctx, "CleanFlannelFiles").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create CleanFlannelNodeFiles step: %w", err)
	}

	_, _ = fragment.AddNode(&plan.ExecutionNode{Name: "CleanFlannelFiles", Step: cleanFlannel, Hosts: masterHostList})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

func (t *CleanNetworkPluginTask) planCilium(ctx *runtime.Context, fragment *plan.ExecutionFragment, masterHostList []remotefw.Host) (*plan.ExecutionFragment, error) {
	cleanCilium, err := cilium.NewCleanCiliumNodeStateStepBuilder(ctx, "CleanCiliumState").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create CleanCiliumNodeState step: %w", err)
	}

	_, _ = fragment.AddNode(&plan.ExecutionNode{Name: "CleanCiliumState", Step: cleanCilium, Hosts: masterHostList})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

func (t *CleanNetworkPluginTask) planKubeovn(ctx *runtime.Context, fragment *plan.ExecutionFragment, masterHostList []remotefw.Host) (*plan.ExecutionFragment, error) {
	cleanKubeovn, err := kubeovn.NewCleanKubeovnStepBuilder(ctx, "CleanKubeovn").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create CleanKubeovn step: %w", err)
	}

	_, _ = fragment.AddNode(&plan.ExecutionNode{Name: "CleanKubeovn", Step: cleanKubeovn, Hosts: masterHostList})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

func (t *CleanNetworkPluginTask) planHybridnet(ctx *runtime.Context, fragment *plan.ExecutionFragment, masterHostList []remotefw.Host) (*plan.ExecutionFragment, error) {
	cleanHybridnet, err := hybridnet.NewCleanHybridnetStepBuilder(ctx, "CleanHybridnet").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create CleanHybridnet step: %w", err)
	}

	_, _ = fragment.AddNode(&plan.ExecutionNode{Name: "CleanHybridnet", Step: cleanHybridnet, Hosts: masterHostList})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

func (t *CleanNetworkPluginTask) planMultus(ctx *runtime.Context, fragment *plan.ExecutionFragment, masterHostList []remotefw.Host) (*plan.ExecutionFragment, error) {
	cleanMultus, err := multus.NewCleanMultusStepBuilder(ctx, "CleanMultus").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create CleanMultus step: %w", err)
	}

	_, _ = fragment.AddNode(&plan.ExecutionNode{Name: "CleanMultus", Step: cleanMultus, Hosts: masterHostList})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// Ensure task.Task interface is implemented
var _ task.Task = (*CleanNetworkPluginTask)(nil)
