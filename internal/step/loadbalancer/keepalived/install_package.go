package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runner"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// InstallKeepalivedPackage installs Keepalived package
type InstallKeepalivedPackage struct {
	step.Base
}

type InstallKeepalivedStepBuilder struct {
	step.Builder[InstallKeepalivedStepBuilder, *InstallKeepalivedPackage]
}

func NewInstallKeepalivedStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallKeepalivedStepBuilder {
	s := &InstallKeepalivedPackage{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install Keepalived package", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallKeepalivedStepBuilder).Init(s)
	return b
}

func (s *InstallKeepalivedPackage) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *InstallKeepalivedPackage) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	r := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	path, _ := r.LookPath(ctx.GoContext(), conn, "keepalived")
	return path != "", nil
}

func (s *InstallKeepalivedPackage) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to get connector for host %s", ctx.GetHost().GetName()))
		return result, err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to get host facts")
		return result, err
	}

	var cmd string
	switch facts.PackageManager.Type {
	case runner.PackageManagerApt:
		cmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "keepalived")
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		cmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "keepalived")
	default:
		result.MarkFailed(fmt.Errorf("unsupported package manager: %s", facts.PackageManager.Type), "unsupported package manager")
		return result, fmt.Errorf("unsupported package manager: %s", facts.PackageManager.Type)
	}

	_, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &runner.ExecOptions{Sudo: s.Base.Sudo})
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to install keepalived package: %s", string(stderr)))
		return result, err
	}
	result.MarkCompleted("Keepalived package installed successfully")
	return result, nil
}

func (s *InstallKeepalivedPackage) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*InstallKeepalivedPackage)(nil)