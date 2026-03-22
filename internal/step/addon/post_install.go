package addon

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RunAddonPostInstallHookStep struct {
	step.Base
	AddonName string
	Commands  []string
}

type RunAddonPostInstallHookStepBuilder struct {
	step.Builder[RunAddonPostInstallHookStepBuilder, *RunAddonPostInstallHookStep]
}

func NewRunAddonPostInstallHookStepBuilder(ctx runtime.ExecutionContext, addonName string) *RunAddonPostInstallHookStepBuilder {
	var targetAddon *v1alpha1.Addon
	for i := range ctx.GetClusterConfig().Spec.Addons {
		if ctx.GetClusterConfig().Spec.Addons[i].Name == addonName {
			targetAddon = &ctx.GetClusterConfig().Spec.Addons[i]
			break
		}
	}

	if targetAddon == nil ||
		(targetAddon.Enabled != nil && !*targetAddon.Enabled) ||
		len(targetAddon.PostInstall) == 0 {
		return nil
	}

	s := &RunAddonPostInstallHookStep{
		AddonName: addonName,
		Commands:  targetAddon.PostInstall,
	}

	s.Base.Meta.Name = fmt.Sprintf("RunAddonPostInstallHook-%s", addonName)
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run post-install hooks for addon '%s'", s.Base.Meta.Name, s.AddonName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(RunAddonPostInstallHookStepBuilder).Init(s)
	return b
}

func (s *RunAddonPostInstallHookStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RunAddonPostInstallHookStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Post-install hooks will always run if the step is reached.")
	return false, nil
}

func (s *RunAddonPostInstallHookStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector for post-install hook")
		return result, err
	}

	for i, command := range s.Commands {
		logger.Info("Executing post-install hook command.", "index", i+1, "command", command)
		output, err := runner.Run(ctx.GoContext(), conn, command, s.Sudo)
		if err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to execute post-install hook command: [%s]", command))
			return result, err
		}
		logger.Debug("Command output.", "output", output)
	}

	logger.Info("Successfully executed all post-install hooks for addon.")
	result.MarkCompleted("all post-install hooks executed successfully")
	return result, nil
}

func (s *RunAddonPostInstallHookStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a post-install hook step is a no-op.")
	return nil
}

var _ step.Step = (*RunAddonPostInstallHookStep)(nil)
