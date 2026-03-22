package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	rn "github.com/mensylisir/kubexm/internal/runner"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CleanNginxStep struct {
	step.Base
}

type CleanNginxStepBuilder struct {
	step.Builder[CleanNginxStepBuilder, *CleanNginxStep]
}

func NewCleanNginxStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanNginxStepBuilder {
	s := &CleanNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean NGINX configuration and uninstall package", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	b := new(CleanNginxStepBuilder).Init(s)
	return b
}

func (s *CleanNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanNginxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *CleanNginxStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	remoteConfigPath := common.DefaultNginxConfigFilePath
	logger.Infof("Removing NGINX config file: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove config file %s (may not exist): %v", remoteConfigPath, err)
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to gather facts before installation")
		return result, err
	}

	if facts.PackageManager == nil || facts.PackageManager.Type == rn.PackageManagerUnknown {
		result.MarkFailed(err, "could not determine a valid package manager")
		return result, err
	}
	pkgManager := facts.PackageManager

	logger.Infof("Uninstalling nginx package...")
	packageName := "nginx"
	removeCMD := fmt.Sprintf(pkgManager.RemoveCmd, packageName)
	if _, err := runner.Run(ctx.GoContext(), conn, removeCMD, s.Sudo); err != nil {
		result.MarkFailed(err, "failed to remove nginx")
		return result, err
	}

	logger.Infof("NGINX cleanup finished on %s.", ctx.GetHost().GetName())
	result.MarkCompleted("NGINX cleanup completed successfully")
	return result, nil
}

func (s *CleanNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*CleanNginxStep)(nil)
