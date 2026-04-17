package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CopyAddonManifestStep copies addon manifest content to remote host.
type CopyAddonManifestStep struct {
	step.Base
	SourceKey  string // Context key for rendered manifest content
	RemotePath string
	Mode       string
	Overwrite  bool
}

type CopyAddonManifestStepBuilder struct {
	step.Builder[CopyAddonManifestStepBuilder, *CopyAddonManifestStep]
}

func NewCopyAddonManifestStepBuilder(ctx runtime.ExecutionContext, instanceName, sourceKey, remotePath, mode string) *CopyAddonManifestStepBuilder {
	s := &CopyAddonManifestStep{
		SourceKey:  sourceKey,
		RemotePath: remotePath,
		Mode:       mode,
		Overwrite:  true,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Copy addon manifest from %s to %s", instanceName, sourceKey, remotePath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(CopyAddonManifestStepBuilder).Init(s)
}

func (s *CopyAddonManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CopyAddonManifestStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
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

func (s *CopyAddonManifestStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	contentRaw, ok := ctx.Import("", s.SourceKey)
	if !ok {
		result.MarkFailed(fmt.Errorf("manifest with key %s not found in context", s.SourceKey), "manifest not found")
		return result, fmt.Errorf("manifest with key %s not found", s.SourceKey)
	}
	content, ok := contentRaw.(string)
	if !ok {
		result.MarkFailed(fmt.Errorf("manifest with key %s has invalid type", s.SourceKey), "invalid type")
		return result, fmt.Errorf("manifest with key %s has invalid type", s.SourceKey)
	}

	logger.Infof("Copying addon manifest to %s:%s", ctx.GetHost().GetName(), s.RemotePath)
	runner := ctx.GetRunner()
	if err := runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.RemotePath, s.Mode, s.Overwrite); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to write manifest to %s", s.RemotePath))
		return result, err
	}

	logger.Infof("Addon manifest copied successfully to %s", s.RemotePath)
	result.MarkCompleted(fmt.Sprintf("Addon manifest copied to %s", s.RemotePath))
	return result, nil
}

func (s *CopyAddonManifestStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*CopyAddonManifestStep)(nil)
