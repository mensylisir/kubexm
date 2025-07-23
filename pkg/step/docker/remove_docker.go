package docker

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RemoveDockerStep struct {
	step.Base
	Packages []string
	Purge    bool
}

type RemoveDockerStepBuilder struct {
	step.Builder[RemoveDockerStepBuilder, *RemoveDockerStep]
}

func NewRemoveDockerStepBuilder(ctx runtime.Context, instanceName string) *RemoveDockerStepBuilder {
	s := &RemoveDockerStep{
		Packages: []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"},
		Purge:    false,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove Docker Engine packages", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(RemoveDockerStepBuilder).Init(s)
	return b
}

func (b *RemoveDockerStepBuilder) WithPackages(packages []string) *RemoveDockerStepBuilder {
	b.Step.Packages = packages
	return b
}

func (b *RemoveDockerStepBuilder) WithPurge(purge bool) *RemoveDockerStepBuilder {
	b.Step.Purge = purge
	return b
}

func (s *RemoveDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, fmt.Errorf("failed to get host facts: %w", err)
	}

	if len(s.Packages) == 0 {
		logger.Info("No Docker packages specified to check for removal.")
		return true, nil
	}
	keyPackage := s.Packages[0]
	installed, err := runner.IsPackageInstalled(ctx.GoContext(), conn, facts, keyPackage)
	if err != nil {
		logger.Warn("Failed to check if Docker package is installed, assuming it might be present.", "package", keyPackage, "error", err)
		return false, nil
	}
	if !installed {
		logger.Info("Key Docker package already not installed.", "package", keyPackage)
		return true, nil
	}
	logger.Info("Key Docker package is installed and needs removal.", "package", keyPackage)
	return false, nil
}

func (s *RemoveDockerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to get host facts: %w", err)
	}

	if len(s.Packages) == 0 {
		logger.Info("No Docker packages specified for removal.")
		return nil
	}

	logger.Info("Removing Docker Engine packages.", "packages", strings.Join(s.Packages, ", "), "purge", s.Purge)
	if err := runner.RemovePackages(ctx.GoContext(), conn, facts, s.Packages...); err != nil {
		return fmt.Errorf("failed to remove Docker Engine packages (%s): %w", strings.Join(s.Packages, ", "), err)
	}

	logger.Info("Docker Engine packages removed successfully.")
	return nil
}

func (s *RemoveDockerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for RemoveDockerStep is not applicable (would mean reinstalling).")
	return nil
}
