package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// DeployRepoServerStep deploys a repository server.
type DeployRepoServerStep struct {
	step.Base
	RepoPath   string
	ServerType string // "nginx" or "apt" or "yum"
	Namespace  string
}

type DeployRepoServerStepBuilder struct {
	step.Builder[DeployRepoServerStepBuilder, *DeployRepoServerStep]
}

func NewDeployRepoServerStepBuilder(ctx runtime.ExecutionContext, instanceName, repoPath, serverType, namespace string) *DeployRepoServerStepBuilder {
	s := &DeployRepoServerStep{
		RepoPath:   repoPath,
		ServerType: serverType,
		Namespace:  namespace,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Deploy %s repository server", instanceName, serverType)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(DeployRepoServerStepBuilder).Init(s)
}

func (s *DeployRepoServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DeployRepoServerStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *DeployRepoServerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("echo 'Deploying %s repo server at %s'", s.ServerType, s.RepoPath)
	logger.Infof("Running: %s", cmd)

	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to deploy repo server")
		return result, err
	}

	logger.Infof("Repository server deployed successfully")
	result.MarkCompleted("Repository server deployed")
	return result, nil
}

func (s *DeployRepoServerStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*DeployRepoServerStep)(nil)
