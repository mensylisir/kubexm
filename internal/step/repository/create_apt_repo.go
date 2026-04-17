package repository

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

type CreateAptRepoStep struct {
	step.Base
	RepoDir string
}

type CreateAptRepoStepBuilder struct {
	step.Builder[CreateAptRepoStepBuilder, *CreateAptRepoStep]
}

func NewCreateAptRepoStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CreateAptRepoStepBuilder {
	s := &CreateAptRepoStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Create or update an Apt repository", instanceName)
	s.Base.Sudo = true // dpkg-scanpackages might be run in a protected directory
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute
	return new(CreateAptRepoStepBuilder).Init(s)
}

func (b *CreateAptRepoStepBuilder) WithRepoDir(repoDir string) *CreateAptRepoStepBuilder {
	b.Step.RepoDir = repoDir
	return b
}

func (s *CreateAptRepoStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CreateAptRepoStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := runner.LookPath(ctx.GoContext(), conn, "dpkg-scanpackages"); err != nil {
		return false, errors.Wrap(err, "`dpkg-scanpackages` command not found on remote host")
	}
	return false, nil
}

func (s *CreateAptRepoStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	if s.RepoDir == "" {
		err := errors.New("RepoDir must be specified for CreateAptRepoStep")
		result.MarkFailed(err, "RepoDir must be specified")
		return result, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RepoDir)
	if err != nil {
		err = errors.Wrapf(err, "failed to check for existence of repository directory '%s'", s.RepoDir)
		result.MarkFailed(err, "failed to check repository directory existence")
		return result, err
	}
	if !exists {
		err = fmt.Errorf("repository directory '%s' does not exist", s.RepoDir)
		result.MarkFailed(err, "repository directory does not exist")
		return result, err
	}

	// The commands need to be run inside the repository directory.
	scanCmd := fmt.Sprintf("cd %s && dpkg-scanpackages . /dev/null > Packages", s.RepoDir)
	gzipCmd := fmt.Sprintf("cd %s && gzip -k -f Packages", s.RepoDir)

	logger.Info("Creating/updating apt repository index...", "command", scanCmd)
	runResult, err := runner.Run(ctx.GoContext(), conn, scanCmd, s.Sudo)
	if err != nil {
		err = errors.Wrapf(err, "failed to create apt repository index\nOutput:\n%s", runResult.Stdout)
		result.MarkFailed(err, "failed to create apt repository index")
		return result, err
	}
	logger.Debug("Command output.", "output", runResult.Stdout)

	logger.Info("Compressing apt repository index...", "command", gzipCmd)
	runResult, err = runner.Run(ctx.GoContext(), conn, gzipCmd, s.Sudo)
	if err != nil {
		err = errors.Wrapf(err, "failed to compress apt repository index\nOutput:\n%s", runResult.Stdout)
		result.MarkFailed(err, "failed to compress apt repository index")
		return result, err
	}
	logger.Debug("Command output.", "output", runResult.Stdout)

	logger.Info("Apt repository created/updated successfully.", "directory", s.RepoDir)
	result.MarkCompleted("Apt repository created/updated successfully")
	return result, nil
}

func (s *CreateAptRepoStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for CreateAptRepoStep is a no-op. The repository metadata will not be removed.")
	return nil
}

var _ step.Step = (*CreateAptRepoStep)(nil)
