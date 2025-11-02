package repository

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type CreateAptRepoStep struct {
	step.Base
	RepoDir string
}

type CreateAptRepoStepBuilder struct {
	step.Builder[CreateAptRepoStepBuilder, *CreateAptRepoStep]
}

func NewCreateAptRepoStepBuilder(ctx runtime.Context, instanceName string) *CreateAptRepoStepBuilder {
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
	// The operation is idempotent, so we just check for the command's existence.
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
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

func (s *CreateAptRepoStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if s.RepoDir == "" {
		return errors.New("RepoDir must be specified for CreateAptRepoStep")
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RepoDir)
	if err != nil {
		return errors.Wrapf(err, "failed to check for existence of repository directory '%s'", s.RepoDir)
	}
	if !exists {
		return fmt.Errorf("repository directory '%s' does not exist", s.RepoDir)
	}

	// The commands need to be run inside the repository directory.
	scanCmd := fmt.Sprintf("cd %s && dpkg-scanpackages . /dev/null > Packages", s.RepoDir)
	gzipCmd := fmt.Sprintf("cd %s && gzip -k -f Packages", s.RepoDir)

	logger.Info("Creating/updating apt repository index...", "command", scanCmd)
	output, err := runner.Run(ctx.GoContext(), conn, scanCmd, s.Sudo)
	if err != nil {
		return errors.Wrapf(err, "failed to create apt repository index\nOutput:\n%s", output)
	}
	logger.Debug("Command output.", "output", output)

	logger.Info("Compressing apt repository index...", "command", gzipCmd)
	output, err = runner.Run(ctx.GoContext(), conn, gzipCmd, s.Sudo)
	if err != nil {
		return errors.Wrapf(err, "failed to compress apt repository index\nOutput:\n%s", output)
	}
	logger.Debug("Command output.", "output", output)

	logger.Info("Apt repository created/updated successfully.", "directory", s.RepoDir)
	return nil
}

func (s *CreateAptRepoStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for CreateAptRepoStep is a no-op. The repository metadata will not be removed.")
	return nil
}

var _ step.Step = (*CreateAptRepoStep)(nil)
