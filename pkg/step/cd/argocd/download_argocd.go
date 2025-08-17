package argocd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/pkg/errors"
)

type DownloadArgoCDChartStep struct {
	step.Base
	HelmBinaryPath string
}

type DownloadArgoCDChartStepBuilder struct {
	step.Builder[DownloadArgoCDChartStepBuilder, *DownloadArgoCDChartStep]
}

func NewDownloadArgoCDChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadArgoCDChartStepBuilder {
	s := &DownloadArgoCDChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Argo CD Helm chart to local work directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadArgoCDChartStepBuilder).Init(s)
	return b
}

func (s *DownloadArgoCDChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadArgoCDChartStep) getChartAndPath(ctx runtime.ExecutionContext) (*helm.HelmChart, string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("argocd")
	if chart == nil {
		return nil, "", fmt.Errorf("could not get Argo CD chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := chart.LocalPath(ctx.GetGlobalWorkDir())

	return chart, destFile, nil
}

func (s *DownloadArgoCDChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, errors.Wrap(err, "helm command not found in local PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath

	_, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Info("Skipping step.", "reason", err)
		return true, nil
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Info("Argo CD chart package already exists locally. Step is complete.", "path", destFile)
		return true, nil
	}

	return false, nil
}

func (s *DownloadArgoCDChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	chart, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Warn("Execution was not skipped by Precheck, but chart info is still unavailable.", "error", err)
		return nil
	}

	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create local destination directory %s: %w", destDir, err)
	}

	logger.Info("Adding Helm repo.", "name", chart.RepoName(), "url", chart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", chart.RepoName(), chart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", chart.RepoName(), err, string(output))
		}
	}

	logger.Info("Updating Helm repo.", "name", chart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", chart.RepoName())
	if output, err := repoUpdateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w\nOutput: %s", chart.RepoName(), err, string(output))
	}

	logger.Info("Pulling Argo CD chart.", "chart", chart.FullName(), "version", chart.Version, "destination", destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", chart.FullName(), "--version", chart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Argo CD chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Argo CD Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadArgoCDChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	_, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Info("Skipping rollback as no chart path could be determined.", "error", err)
		return nil
	}

	if _, statErr := os.Stat(destFile); statErr == nil {
		logger.Warn("Rolling back by deleting locally downloaded chart file.", "path", destFile)
		if err := os.Remove(destFile); err != nil {
			logger.Error(err, "Failed to remove file during rollback.", "path", destFile)
		}
	} else {
		logger.Info("Rollback unnecessary, file to be deleted does not exist.", "path", destFile)
	}

	return nil
}

var _ step.Step = (*DownloadArgoCDChartStep)(nil)
