package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CopyEtcdConfigStep copies rendered etcd configuration to remote host.
type CopyEtcdConfigStep struct {
	step.Base
	SourceKey  string // Context key for rendered config content
	RemotePath string
	Mode       string
	Overwrite  bool
}

type CopyEtcdConfigStepBuilder struct {
	step.Builder[CopyEtcdConfigStepBuilder, *CopyEtcdConfigStep]
}

func NewCopyEtcdConfigStepBuilder(ctx runtime.ExecutionContext, instanceName, sourceKey, remotePath, mode string) *CopyEtcdConfigStepBuilder {
	s := &CopyEtcdConfigStep{
		SourceKey:  sourceKey,
		RemotePath: remotePath,
		Mode:       mode,
		Overwrite:  true,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Copy etcd config from %s to %s", instanceName, sourceKey, remotePath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(CopyEtcdConfigStepBuilder).Init(s)
}

func (s *CopyEtcdConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CopyEtcdConfigStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemotePath)
	if err != nil {
		return false, err
	}

	if exists && !s.Overwrite {
		return true, nil
	}

	return false, nil
}

func (s *CopyEtcdConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	// Get content from context
	contentRaw, ok := ctx.Import("", s.SourceKey)
	if !ok {
		result.MarkFailed(fmt.Errorf("config with key %s not found in context", s.SourceKey), "config not found")
		return result, fmt.Errorf("config with key %s not found", s.SourceKey)
	}
	content, ok := contentRaw.(string)
	if !ok {
		result.MarkFailed(fmt.Errorf("config with key %s has invalid type", s.SourceKey), "invalid type")
		return result, fmt.Errorf("config with key %s has invalid type", s.SourceKey)
	}

	logger.Infof("Copying etcd config to %s:%s", ctx.GetHost().GetName(), s.RemotePath)
	if err := ctx.GetRunner().WriteFile(ctx.GoContext(), conn, []byte(content), s.RemotePath, s.Mode, s.Overwrite); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to write config to %s", s.RemotePath))
		return result, err
	}

	logger.Infof("Etcd config copied successfully to %s", s.RemotePath)
	result.MarkCompleted(fmt.Sprintf("Etcd config copied to %s", s.RemotePath))
	return result, nil
}

func (s *CopyEtcdConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.RemotePath)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemotePath, true, false); err != nil {
		logger.Errorf("Failed to remove %s during rollback: %v", s.RemotePath, err)
	}
	return nil
}

var _ step.Step = (*CopyEtcdConfigStep)(nil)
