package helm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
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
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Add helm repository", s.Base.Meta.Name)
	s.Base.Sudo = true
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

	cmd := "helm"
	args := []string{"repo", "list"}
	stdout, _, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")), s.Sudo)
	if err != nil {
		return false, fmt.Errorf("failed to list helm repos: %w", err)
	}

	if strings.Contains(stdout, s.RepoName) {
		logger.Infof("Helm repo %s already exists. Step is done.", s.RepoName)
		return true, nil
	}

	return false, nil
}

func (s *AddRepoStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cmd := "helm"
	args := []string{"repo", "add", s.RepoName, s.RepoURL}

	logger.Infof("Adding helm repo %s from %s", s.RepoName, s.RepoURL)
	_, _, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")), s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to add helm repo: %w", err)
	}

	return nil
}

func (s *AddRepoStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	cmd := "helm"
	args := []string{"repo", "remove", s.RepoName}

	logger.Warnf("Rolling back by removing helm repo %s", s.RepoName)
	_, _, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")), s.Sudo)
	if err != nil {
		logger.Errorf("Failed to remove helm repo during rollback: %v", err)
	}

	return nil
}
