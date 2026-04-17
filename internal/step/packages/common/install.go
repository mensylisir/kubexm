package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// InstallPackageStep installs a package.
type InstallPackageStep struct {
	step.Base
	PackageName string
}

type InstallPackageStepBuilder struct {
	step.Builder[InstallPackageStepBuilder, *InstallPackageStep]
}

func NewInstallPackageStepBuilder(ctx runtime.ExecutionContext, instanceName, packageName string) *InstallPackageStepBuilder {
	s := &InstallPackageStep{
		PackageName: packageName,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install package %s", instanceName, packageName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute
	return new(InstallPackageStepBuilder).Init(s)
}

func (s *InstallPackageStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallPackageStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *InstallPackageStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("apt-get install -y %s || yum install -y %s", s.PackageName, s.PackageName)
	logger.Infof("Running: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to install package")
		return result, err
	}

	logger.Infof("Package %s installed successfully", s.PackageName)
	result.MarkCompleted("Package installed")
	return result, nil
}

func (s *InstallPackageStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*InstallPackageStep)(nil)
