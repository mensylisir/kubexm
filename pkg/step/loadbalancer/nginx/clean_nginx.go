package nginx

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	rn "github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"time"
)

type CleanNginxStep struct {
	step.Base
}

type CleanNginxStepBuilder struct {
	step.Builder[CleanNginxStepBuilder, *CleanNginxStep]
}

func NewCleanNginxStepBuilder(ctx runtime.Context, instanceName string) *CleanNginxStepBuilder {
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

func (s *CleanNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	remoteConfigPath := common.DefaultNginxConfigFilePath
	logger.Infof("Removing NGINX config file: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove config file %s (may not exist): %v", remoteConfigPath, err)
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to gather facts before installation: %w", err)
	}

	if facts.PackageManager == nil || facts.PackageManager.Type == rn.PackageManagerUnknown {
		return fmt.Errorf("could not determine a valid package manager for host %s", ctx.GetHost().GetName())
	}
	pkgManager := facts.PackageManager

	logger.Infof("Uninstalling nginx package...")
	packageName := "nginx"
	removeCMD := fmt.Sprintf(pkgManager.RemoveCmd, packageName)
	if _, err := runner.Run(ctx.GoContext(), conn, removeCMD, s.Sudo); err != nil {
		return fmt.Errorf("failed to remove %s: %w", packageName, err)
	}

	logger.Infof("NGINX cleanup finished on %s.", ctx.GetHost().GetName())
	return nil
}

func (s *CleanNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*CleanNginxStep)(nil)
