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

const keepalivedServiceName = "keepalived"

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
	installed, err := isKeepalivedInstalled(ctx)
	if err != nil {
		return false, err
	}
	return !installed, nil
}

func (s *RemoveKeepalivedPackage) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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

	installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, keepalivedServiceName)
	if err != nil {
		result.MarkFailed(err, "failed to determine keepalived package state")
		return result, err
	}
	if !installed {
		result.MarkCompleted("Keepalived package already absent")
		return result, nil
	}

	if err := runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, keepalivedServiceName); err != nil {
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
	packageInstalled, err := isKeepalivedInstalled(ctx)
	if err != nil {
		return false, err
	}
	configExists, err := keepalivedConfigExists(ctx)
	if err != nil {
		return false, err
	}
	serviceActive, err := isKeepalivedServiceActive(ctx)
	if err != nil {
		return false, err
	}
	serviceEnabled, err := isKeepalivedServiceEnabled(ctx)
	if err != nil {
		return false, err
	}
	return !packageInstalled && !configExists && !serviceActive && !serviceEnabled, nil
}

func (s *CleanKeepalivedPackage) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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

	if active, err := runnerSvc.IsServiceActive(ctx.GoContext(), conn, facts, keepalivedServiceName); err == nil && active {
		if err := runnerSvc.StopService(ctx.GoContext(), conn, facts, keepalivedServiceName); err != nil {
			result.MarkFailed(err, "failed to stop keepalived service during cleanup")
			return result, err
		}
	} else if err != nil {
		result.MarkFailed(err, "failed to determine keepalived service status during cleanup")
		return result, err
	}

	if enabled, err := runnerSvc.IsServiceEnabled(ctx.GoContext(), conn, facts, keepalivedServiceName); err == nil && enabled {
		if err := runnerSvc.DisableService(ctx.GoContext(), conn, facts, keepalivedServiceName); err != nil {
			result.MarkFailed(err, "failed to disable keepalived service during cleanup")
			return result, err
		}
	} else if err != nil {
		result.MarkFailed(err, "failed to determine keepalived service enablement during cleanup")
		return result, err
	}

	if installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, keepalivedServiceName); err == nil && installed {
		if err := runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, keepalivedServiceName); err != nil {
			result.MarkFailed(err, "failed to remove keepalived package")
			return result, err
		}
	} else if err != nil {
		result.MarkFailed(err, "failed to determine keepalived package state during cleanup")
		return result, err
	}

	configExists, err := runnerSvc.Exists(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget)
	if err != nil {
		result.MarkFailed(err, "failed to check keepalived config during cleanup")
		return result, err
	}
	if configExists {
		if err := runnerSvc.Remove(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget, true, false); err != nil {
			result.MarkFailed(err, "failed to remove keepalived config during cleanup")
			return result, err
		}
	}

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
	configExists, err := keepalivedConfigExists(ctx)
	if err != nil {
		return false, err
	}
	return !configExists, nil
}

func (s *RemoveKeepalivedConfig) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to get connector for host %s", ctx.GetHost().GetName()))
		return result, err
	}
	configExists, err := runnerSvc.Exists(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget)
	if err != nil {
		result.MarkFailed(err, "failed to check keepalived config")
		return result, err
	}
	if !configExists {
		result.MarkCompleted("Keepalived config already absent")
		return result, nil
	}
	if err := runnerSvc.Remove(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget, true, false); err != nil {
		result.MarkFailed(err, "failed to remove keepalived config")
		return result, err
	}
	result.MarkCompleted("Keepalived config removed successfully")
	return result, nil
}

func (s *RemoveKeepalivedConfig) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RemoveKeepalivedConfig)(nil)

func isKeepalivedInstalled(ctx runtime.ExecutionContext) (bool, error) {
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, err
	}
	return runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, keepalivedServiceName)
}

func keepalivedConfigExists(ctx runtime.ExecutionContext) (bool, error) {
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	return runnerSvc.Exists(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget)
}

func isKeepalivedServiceActive(ctx runtime.ExecutionContext) (bool, error) {
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, err
	}
	return runnerSvc.IsServiceActive(ctx.GoContext(), conn, facts, keepalivedServiceName)
}

func isKeepalivedServiceEnabled(ctx runtime.ExecutionContext) (bool, error) {
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, err
	}
	return runnerSvc.IsServiceEnabled(ctx.GoContext(), conn, facts, keepalivedServiceName)
}
