package addon

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/addon/argocd"
	"github.com/mensylisir/kubexm/pkg/task/addon/ingressnginx"
	"github.com/mensylisir/kubexm/pkg/task/addon/longhorn"
	"github.com/mensylisir/kubexm/pkg/task/addon/nfs"
	"github.com/mensylisir/kubexm/pkg/task/addon/openebs"
)

// InstallAddonsTask is a dispatcher task that chooses the correct addon installation task.
type InstallAddonsTask struct {
	task.BaseTask
	AddonName string
}

// NewInstallAddonsTask creates a new InstallAddonsTask for a specific addon.
func NewInstallAddonsTask(addonName string) task.Task {
	return &InstallAddonsTask{
		BaseTask: task.NewBaseTask(
			fmt.Sprintf("InstallAddon-%s", addonName),
			fmt.Sprintf("Deploys addon: %s", addonName),
			nil,
			nil,
			false,
		),
		AddonName: addonName,
	}
}

func (t *InstallAddonsTask) IsRequired(ctx task.TaskContext) (bool, error) {
	// This task is required if the addon is configured in the cluster spec.
	// The calling module should iterate through the configured addons and create this task for each one.
	return true, nil
}

// Plan is a dispatcher that selects the appropriate addon-specific task.
func (t *InstallAddonsTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "addon", t.AddonName)

	var addonTask task.Task

	switch t.AddonName {
	case "ingress-nginx":
		addonTask = ingressnginx.NewInstallIngressNginxTask()
	case "openebs-local":
		addonTask = openebs.NewInstallOpenEBSTask()
	case "nfs":
		addonTask = nfs.NewInstallNFSTask()
	case "longhorn":
		addonTask = longhorn.NewInstallLonghornTask()
	case "argocd":
		addonTask = argocd.NewInstallArgoCDTask()
	default:
		logger.Warn("No specific installation task found for addon. This addon will be skipped.", "addon_name", t.AddonName)
		return task.NewEmptyFragment(), nil
	}

	logger.Info("Dispatching to addon-specific installation task.")
	return addonTask.Plan(ctx)
}

var _ task.Task = (*InstallAddonsTask)(nil)
