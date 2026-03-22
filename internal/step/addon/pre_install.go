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

type RunAddonPreInstallHookStep struct {
	step.Base
	AddonName string
	Commands  []string
}

type RunAddonPreInstallHookStepBuilder struct {
	step.Builder[RunAddonPreInstallHookStepBuilder, *RunAddonPreInstallHookStep]
}

func NewRunAddonPreInstallHookStepBuilder(ctx runtime.ExecutionContext, addonName string) *RunAddonPreInstallHookStepBuilder {
	var targetAddon *v1alpha1.Addon
	for i := range ctx.GetClusterConfig().Spec.Addons {
		if ctx.GetClusterConfig().Spec.Addons[i].Name == addonName {
			targetAddon = &ctx.GetClusterConfig().Spec.Addons[i]
			break
		}
	}

	if targetAddon == nil ||
		(targetAddon.Enabled != nil && !*targetAddon.Enabled) ||
		len(targetAddon.PreInstall) == 0 {
		return nil
	}

	s := &RunAddonPreInstallHookStep{
		AddonName: addonName,
		Commands:  targetAddon.PreInstall,
	}

	s.Base.Meta.Name = fmt.Sprintf("RunAddonPreInstallHook-%s", addonName)
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run pre-install hooks for addon '%s'", s.Base.Meta.Name, s.AddonName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(RunAddonPreInstallHookStepBuilder).Init(s)
	return b
}

func (s *RunAddonPreInstallHookStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RunAddonPreInstallHookStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Pre-install hooks will always run if the step is reached.")
	return false, nil
}

func (s *RunAddonPreInstallHookStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector for pre-install hook")
		return result, err
	}

	for i, command := range s.Commands {
		logger.Info("Executing pre-install hook command.", "index", i+1, "command", command)
		output, err := runner.Run(ctx.GoContext(), conn, command, s.Sudo)
		if err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to execute pre-install hook command: [%s]", command))
			return result, err
		}
		logger.Debug("Command output.", "output", output)
	}

	logger.Info("Successfully executed all pre-install hooks for addon.")
	result.MarkCompleted("all pre-install hooks executed successfully")
	return result, nil
}

func (s *RunAddonPreInstallHookStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a pre-install hook step is a no-op.")
	return nil
}

var _ step.Step = (*RunAddonPreInstallHookStep)(nil)
