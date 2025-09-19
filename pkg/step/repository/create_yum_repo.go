package repository

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type CreateYumRepoStep struct {
	step.Base
	RepoDir string
}

type CreateYumRepoStepBuilder struct {
	step.Builder[CreateYumRepoStepBuilder, *CreateYumRepoStep]
}

func NewCreateYumRepoStepBuilder(ctx runtime.Context, instanceName string) *CreateYumRepoStepBuilder {
	s := &CreateYumRepoStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Create or update a Yum repository", instanceName)
	s.Base.Sudo = true // createrepo_c might need to write to a protected directory
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute
	return new(CreateYumRepoStepBuilder).Init(s)
}

func (b *CreateYumRepoStepBuilder) WithRepoDir(repoDir string) *CreateYumRepoStepBuilder {
	b.Step.RepoDir = repoDir
	return b
}

func (s *CreateYumRepoStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CreateYumRepoStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// The createrepo_c command with --update is idempotent, so we can skip a complex precheck
	// and just run the command. The command itself will determine if work needs to be done.
	// We will just check if the command exists.
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := runner.LookPath(ctx.GoContext(), conn, "createrepo_c"); err != nil {
		logger.Warn("`createrepo_c` command not found, falling back to `createrepo`")
		if _, err := runner.LookPath(ctx.GoContext(), conn, "createrepo"); err != nil {
			return false, errors.Wrap(err, "`createrepo_c` and `createrepo` commands not found on remote host")
		}
	}
	return false, nil
}

func (s *CreateYumRepoStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if s.RepoDir == "" {
		return errors.New("RepoDir must be specified for CreateYumRepoStep")
	}

	// Check if the directory exists
	exists, err := runner.Exists(ctx.GoContext(), conn, s.RepoDir)
	if err != nil {
		return errors.Wrapf(err, "failed to check for existence of repository directory '%s'", s.RepoDir)
	}
	if !exists {
		return fmt.Errorf("repository directory '%s' does not exist", s.RepoDir)
	}

	cmdName := "createrepo_c"
	if _, err := runner.LookPath(ctx.GoContext(), conn, cmdName); err != nil {
		cmdName = "createrepo"
	}

	// Use --update to make the operation idempotent
	cmd := fmt.Sprintf("%s --update %s", cmdName, s.RepoDir)

	logger.Info("Creating/updating yum repository...", "command", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return errors.Wrapf(err, "failed to create yum repository\nOutput:\n%s", output)
	}
	logger.Debug("Command output.", "output", output)

	logger.Info("Yum repository created/updated successfully.", "directory", s.RepoDir)
	return nil
}

func (s *CreateYumRepoStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for CreateYumRepoStep is a no-op. The repository metadata will not be removed.")
	return nil
}

var _ step.Step = (*CreateYumRepoStep)(nil)
