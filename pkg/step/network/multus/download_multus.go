package multus

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/pkg/errors"
)

type DownloadMultusChartStep struct {
	step.Base
	HelmBinaryPath string
}

type DownloadMultusChartStepBuilder struct {
	step.Builder[DownloadMultusChartStepBuilder, *DownloadMultusChartStep]
}

func NewDownloadMultusChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadMultusChartStepBuilder {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Network.Multus == nil || cfg.Spec.Network.Multus.Installation.Enabled == nil || !*cfg.Spec.Network.Multus.Installation.Enabled {
		return nil
	}

	s := &DownloadMultusChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Multus Helm chart to local work directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadMultusChartStepBuilder).Init(s)
	return b
}

func (s *DownloadMultusChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadMultusChartStep) getChartAndPath(ctx runtime.ExecutionContext) (*helm.HelmChart, string, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Network.Multus == nil || cfg.Spec.Network.Multus.Installation.Enabled == nil || !*cfg.Spec.Network.Multus.Installation.Enabled {
		return nil, "", fmt.Errorf("Multus is not enabled")
	}

	helmProvider := helm.NewHelmProvider(ctx)
	multusChart := helmProvider.GetChart(string(common.CNITypeMultus))
	if multusChart == nil {
		return nil, "", fmt.Errorf("could not get Multus chart info for Kubernetes version %s", cfg.Spec.Kubernetes.Version)
	}

	destFile := multusChart.LocalPath(ctx.GetGlobalWorkDir())

	return multusChart, destFile, nil
}

func (s *DownloadMultusChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, errors.Wrap(err, "helm command not found in local PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath

	_, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Infof("Skipping step: %v", err)
		return true, nil
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("Multus chart package %s already exists locally. Step is complete.", destFile)
		return true, nil
	}

	return false, nil
}

func (s *DownloadMultusChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	multusChart, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Warnf("Execution was not skipped by Precheck, but chart info is still unavailable. Error: %v", err)
		return nil
	}

	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create local destination directory %s: %w", destDir, err)
	}

	logger.Infof("Adding Helm repo: %s (%s)", multusChart.RepoName(), multusChart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", multusChart.RepoName(), multusChart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", multusChart.RepoName(), err, string(output))
		}
	}

	logger.Infof("Updating Helm repo: %s", multusChart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", multusChart.RepoName())
	if output, err := repoUpdateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w\nOutput: %s", multusChart.RepoName(), err, string(output))
	}

	logger.Infof("Pulling Multus chart %s (version %s) to %s", multusChart.FullName(), multusChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", multusChart.FullName(), "--version", multusChart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Multus chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Multus Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadMultusChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	_, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Infof("Skipping rollback as no chart path could be determined: %v", err)
		return nil
	}

	if _, statErr := os.Stat(destFile); statErr == nil {
		logger.Warnf("Rolling back by deleting locally downloaded chart file: %s", destFile)
		if err := os.Remove(destFile); err != nil {
			logger.Errorf("Failed to remove file during rollback: %v", err)
		}
	} else {
		logger.Infof("Rollback unnecessary, file to be deleted does not exist: %s", destFile)
	}

	return nil
}

var _ step.Step = (*DownloadMultusChartStep)(nil)
