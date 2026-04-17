package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/types"
)

// CopyConfigStep copies rendered configuration to remote host.
type CopyConfigStep struct {
	step.Base
	SourceKey  string // Context key for rendered config content
	RemotePath string
	Mode       string
	Overwrite  bool
}

type CopyConfigStepBuilder struct {
	step.Builder[CopyConfigStepBuilder, *CopyConfigStep]
}

func NewCopyConfigStepBuilder(ctx runtime.ExecutionContext, instanceName, sourceKey, remotePath, mode string) *CopyConfigStepBuilder {
	s := &CopyConfigStep{
		SourceKey:  sourceKey,
		RemotePath: remotePath,
		Mode:       mode,
		Overwrite:  true,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Copy config from %s to %s", instanceName, sourceKey, remotePath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(CopyConfigStepBuilder).Init(s)
}

func (s *CopyConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CopyConfigStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	content, err := s.getContent(ctx)
	if err != nil {
		return false, err
	}

	return helpers.CheckRemoteFileIntegrity(ctx, s.RemotePath, content, s.Sudo)
}

func (s *CopyConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	ctx.SetStepResult(result)
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	content, err := s.getContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to get config content")
		return result, err
	}

	existedBefore, err := ctx.GetRunner().Exists(ctx.GoContext(), conn, s.RemotePath)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to check existing config at %s", s.RemotePath))
		return result, err
	}
	result.SetMetadata("copy_config_existed_before", existedBefore)

	logger.Infof("Copying config to %s:%s", ctx.GetHost().GetName(), s.RemotePath)
	if err := helpers.WriteContentToRemote(ctx, conn, content, s.RemotePath, s.Mode, s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to write config to %s", s.RemotePath))
		return result, err
	}

	logger.Infof("Config copied successfully to %s", s.RemotePath)
	result.MarkCompleted(fmt.Sprintf("Config copied to %s", s.RemotePath))
	return result, nil
}

func (s *CopyConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	stepResult := ctx.GetStepResult()
	if stepResult == nil {
		logger.Warn("Rollback skipped: no step result context available.")
		return nil
	}

	existedBeforeValue, ok := stepResult.GetMetadata("copy_config_existed_before")
	if !ok {
		logger.Warn("Rollback skipped: original config state was not recorded.")
		return nil
	}

	existedBefore, ok := existedBeforeValue.(bool)
	if !ok || existedBefore {
		logger.Info("Rollback skipped to avoid deleting a potentially pre-existing config file.")
		return nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Warnf("Rollback skipped: failed to get connector: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.RemotePath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemotePath, true, false); err != nil {
		logger.Errorf("Failed to remove %s during rollback: %v", s.RemotePath, err)
		return err
	}
	return nil
}

func (s *CopyConfigStep) getContent(ctx runtime.ExecutionContext) (string, error) {
	contentRaw, ok := ctx.Import("", s.SourceKey)
	if !ok {
		return "", fmt.Errorf("config with key %s not found in context", s.SourceKey)
	}
	content, ok := contentRaw.(string)
	if !ok {
		return "", fmt.Errorf("config with key %s has invalid type", s.SourceKey)
	}
	return content, nil
}

var _ step.Step = (*CopyConfigStep)(nil)
