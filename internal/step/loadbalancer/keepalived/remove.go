package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// RemoveKeepalivedPackage removes Keepalived package
type RemoveKeepalivedPackage struct {
	step.Base
}

type RemoveKeepalivedStepBuilder struct {
	step.Builder[RemoveKeepalivedStepBuilder, *RemoveKeepalivedPackage]
}

func NewRemoveKeepalivedStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RemoveKeepalivedStepBuilder {
	s := &RemoveKeepalivedPackage{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove Keepalived package", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 2 * time.Minute

	b := new(RemoveKeepalivedStepBuilder).Init(s)
	return b
}

func (s *RemoveKeepalivedPackage) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *RemoveKeepalivedPackage) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RemoveKeepalivedPackage) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.PackageManager.RemoveCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	if err != nil {
		result.MarkFailed(err, "failed to remove keepalived package")
		return result, err
	}
	result.MarkCompleted("Keepalived package removed successfully")
	return result, nil
}

func (s *RemoveKeepalivedPackage) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RemoveKeepalivedPackage)(nil)

// CleanKeepalivedPackage cleans up Keepalived package and configuration
type CleanKeepalivedPackage struct {
	step.Base
}

type CleanKeepalivedStepBuilder struct {
	step.Builder[CleanKeepalivedStepBuilder, *CleanKeepalivedPackage]
}

func NewCleanKeepalivedStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanKeepalivedStepBuilder {
	s := &CleanKeepalivedPackage{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean Keepalived package and config", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 2 * time.Minute

	b := new(CleanKeepalivedStepBuilder).Init(s)
	return b
}

func (s *CleanKeepalivedPackage) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *CleanKeepalivedPackage) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CleanKeepalivedPackage) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()

	// Stop service
	stopCmd := fmt.Sprintf(facts.InitSystem.StopCmd, "keepalived")
	conn.Exec(ctx.GoContext(), stopCmd, nil)

	// Disable service
	disableCmd := fmt.Sprintf(facts.InitSystem.DisableCmd, "keepalived")
	conn.Exec(ctx.GoContext(), disableCmd, nil)

	// Remove package
	removeCmd := fmt.Sprintf(facts.PackageManager.RemoveCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), removeCmd, nil)
	if err != nil {
		result.MarkFailed(err, "failed to clean keepalived package")
		return result, err
	}

	// Remove config
	ctx.GetRunner().Remove(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget, true, false)

	result.MarkCompleted("Keepalived package and config cleaned successfully")
	return result, nil
}

func (s *CleanKeepalivedPackage) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CleanKeepalivedPackage)(nil)

// RemoveKeepalivedConfig removes Keepalived configuration
type RemoveKeepalivedConfig struct {
	step.Base
}

type RemoveKeepalivedConfigStepBuilder struct {
	step.Builder[RemoveKeepalivedConfigStepBuilder, *RemoveKeepalivedConfig]
}

func NewRemoveKeepalivedConfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RemoveKeepalivedConfigStepBuilder {
	s := &RemoveKeepalivedConfig{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove Keepalived configuration", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(RemoveKeepalivedConfigStepBuilder).Init(s)
	return b
}

func (s *RemoveKeepalivedConfig) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *RemoveKeepalivedConfig) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RemoveKeepalivedConfig) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	ctx.GetRunner().Remove(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget, true, false)
	result.MarkCompleted("Keepalived config removed successfully")
	return result, nil
}

func (s *RemoveKeepalivedConfig) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RemoveKeepalivedConfig)(nil)