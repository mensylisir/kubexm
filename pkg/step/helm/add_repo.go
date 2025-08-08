package helm

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type AddRepoStep struct {
	step.Base
	RepoName string
	RepoURL  string
}

type AddRepoStepBuilder struct {
	step.Builder[AddRepoStepBuilder, *AddRepoStep]
}

func NewAddRepoStepBuilder(ctx runtime.Context, instanceName string) *AddRepoStepBuilder {
	s := &AddRepoStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Add or update a Helm repository on a remote host", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(AddRepoStepBuilder).Init(s)
	return b
}

func (b *AddRepoStepBuilder) WithRepoName(repoName string) *AddRepoStepBuilder {
	b.Step.RepoName = repoName
	return b
}

func (b *AddRepoStepBuilder) WithRepoURL(repoURL string) *AddRepoStepBuilder {
	b.Step.RepoURL = repoURL
	return b
}

func (s *AddRepoStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *AddRepoStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if s.RepoName == "" || s.RepoURL == "" {
		return false, errors.New("RepoName and RepoURL must be provided")
	}

	if _, err := runner.LookPath(ctx.GoContext(), conn, "helm"); err != nil {
		return false, errors.Wrap(err, "helm command not found on remote host")
	}

	listCmd := "helm repo list"
	output, err := runner.Run(ctx.GoContext(), conn, listCmd, s.Sudo)
	if err != nil {
		logger.Warnf("Failed to list helm repos, assuming repo needs to be added: %v", err)
		return false, nil
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			repoName := fields[0]
			repoURL := fields[1]
			if repoName == s.RepoName {
				if repoURL == s.RepoURL {
					logger.Infof("Helm repo '%s' with URL '%s' already exists. Step is complete.", s.RepoName, s.RepoURL)
					return true, nil
				} else {
					logger.Warnf("Helm repo '%s' exists but with a different URL ('%s'). It will be updated.", s.RepoName, repoURL)
					return false, nil
				}
			}
		}
	}

	logger.Infof("Helm repo '%s' does not exist. It will be added.", s.RepoName)
	return false, nil
}

func (s *AddRepoStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	addCmd := fmt.Sprintf("helm repo add %s %s --force-update", s.RepoName, s.RepoURL)

	logger.Infof("Adding or updating helm repo '%s' from '%s'", s.RepoName, s.RepoURL)
	output, err := runner.Run(ctx.GoContext(), conn, addCmd, s.Sudo)
	if err != nil {
		return errors.Wrapf(err, "failed to add helm repo '%s'\nOutput: %s", s.RepoName, output)
	}

	logger.Info("Successfully added or updated helm repo.")
	logger.Debugf("Helm command output:\n%s", output)
	return nil
}

func (s *AddRepoStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	removeCmd := fmt.Sprintf("helm repo remove %s", s.RepoName)

	logger.Warnf("Rolling back by removing helm repo '%s'", s.RepoName)
	if _, err := runner.Run(ctx.GoContext(), conn, removeCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove helm repo during rollback (this may be expected if add failed): %v", err)
	} else {
		logger.Info("Successfully executed helm repo remove.")
	}

	return nil
}
