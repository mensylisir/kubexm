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
	conn, _ := ctx.GetCurrentHostConnector()
	facts, _ := ctx.GetHostFacts(ctx.GetHost())

	var cmd string
	switch facts.PackageManager.Type {
	case runner.PackageManagerApt:
		cmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "keepalived")
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		cmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "keepalived")
	default:
		result.MarkFailed(fmt.Errorf("unsupported package manager"), "unsupported package manager")
		return result, fmt.Errorf("unsupported package manager")
	}

	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	if err != nil {
		result.MarkFailed(err, "failed to install keepalived package")
		return result, err
	}
	result.MarkCompleted("Keepalived package installed successfully")
	return result, nil
}

func (s *InstallKeepalivedPackage) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*InstallKeepalivedPackage)(nil)