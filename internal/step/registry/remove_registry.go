package registry

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RemoveRegistryArtifactsStep struct {
	step.Base
}

type RemoveRegistryArtifactsStepBuilder struct {
	step.Builder[RemoveRegistryArtifactsStepBuilder, *RemoveRegistryArtifactsStep]
}

func NewRemoveRegistryArtifactsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RemoveRegistryArtifactsStepBuilder {
	s := &RemoveRegistryArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove registry binary, config and service files", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 2 * time.Minute

	b := new(RemoveRegistryArtifactsStepBuilder).Init(s)
	return b
}

func (s *RemoveRegistryArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveRegistryArtifactsStep) filesToRemove() []string {
	return []string{
		filepath.Join(common.DefaultBinDir, "registry"),
		"/etc/docker/registry",
		registryServicePath,
	}
}

func (s *RemoveRegistryArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	allRemoved := true
	for _, path := range s.filesToRemove() {
		if exists, _ := runner.Exists(ctx.GoContext(), conn, path); exists {
			allRemoved = false
			break
		}
	}
	return allRemoved, nil
}

func (s *RemoveRegistryArtifactsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	for _, path := range s.filesToRemove() {
		_ = runner.Remove(ctx.GoContext(), conn, path, s.Sudo, true)
	}

	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo)
	result.MarkCompleted("registry artifacts removed successfully")
	return result, nil
}

func (s *RemoveRegistryArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for an artifacts removal step is not supported.")
	return nil
}

var _ step.Step = (*RemoveRegistryArtifactsStep)(nil)
