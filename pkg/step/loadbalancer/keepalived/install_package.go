package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ===================================================================
// InstallKeepalivedPackage - 安装 Keepalived 包
// ===================================================================

type InstallKeepalivedPackage struct {
	step.Base
}

func NewInstallKeepalivedPackage(ctx runtime.Context, name string) *InstallKeepalivedPackage {
	s := &InstallKeepalivedPackage{}
	s.Base.Meta.Name = name
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install Keepalived package", name)
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute
	return s
}

func (s *InstallKeepalivedPackage) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *InstallKeepalivedPackage) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	r := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	path, _ := r.LookPath(ctx.GoContext(), conn, "keepalived")
	return path != "", nil
}

func (s *InstallKeepalivedPackage) Run(ctx runtime.ExecutionContext) error {
	conn, _ := ctx.GetCurrentHostConnector()
	facts, _ := ctx.GetHostFacts(ctx.GetHost())

	var cmd string
	switch facts.PackageManager.Type {
	case runner.PackageManagerApt:
		cmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "keepalived")
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		cmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "keepalived")
	default:
		return fmt.Errorf("unsupported package manager")
	}

	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	return err
}

func (s *InstallKeepalivedPackage) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}
