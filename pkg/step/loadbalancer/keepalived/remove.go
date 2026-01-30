package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ===================================================================
// RemoveKeepalivedPackage - 卸载 Keepalived 包
// ===================================================================

type RemoveKeepalivedPackage struct {
	step.Base
}

func NewRemoveKeepalivedPackage(ctx runtime.Context, name string) *RemoveKeepalivedPackage {
	s := &RemoveKeepalivedPackage{}
	s.Base.Meta.Name = name
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove Keepalived package", name)
	s.Base.Sudo = true
	s.Base.Timeout = 2 * time.Minute
	return s
}

func (s *RemoveKeepalivedPackage) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *RemoveKeepalivedPackage) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RemoveKeepalivedPackage) Run(ctx runtime.ExecutionContext) error {
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.PackageManager.RemoveCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	return err
}

func (s *RemoveKeepalivedPackage) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

// ===================================================================
// RemoveKeepalivedConfig - 删除 Keepalived 配置
// ===================================================================

type RemoveKeepalivedConfig struct {
	step.Base
}

func NewRemoveKeepalivedConfig(ctx runtime.Context, name string) *RemoveKeepalivedConfig {
	s := &RemoveKeepalivedConfig{}
	s.Base.Meta.Name = name
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove Keepalived configuration", name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	return s
}

func (s *RemoveKeepalivedConfig) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *RemoveKeepalivedConfig) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RemoveKeepalivedConfig) Run(ctx runtime.ExecutionContext) error {
	conn, _ := ctx.GetCurrentHostConnector()
	ctx.GetRunner().Remove(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget, true, false)
	return nil
}

func (s *RemoveKeepalivedConfig) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}
