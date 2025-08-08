package cilium

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

type DownloadCiliumChartStep struct {
	step.Base
	HelmBinaryPath string
}

type DownloadCiliumChartStepBuilder struct {
	step.Builder[DownloadCiliumChartStepBuilder, *DownloadCiliumChartStep]
}

func NewDownloadCiliumChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadCiliumChartStepBuilder {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeCilium) {
		return nil
	}

	s := &DownloadCiliumChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Cilium Helm chart to local work directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadCiliumChartStepBuilder).Init(s)
	return b
}

func (s *DownloadCiliumChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadCiliumChartStep) getChartAndPath(ctx runtime.ExecutionContext) (*helm.HelmChart, string, error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeCilium) {
		return nil, "", fmt.Errorf("Cilium is not the configured CNI plugin")
	}

	helmProvider := helm.NewHelmProvider(ctx)
	ciliumChart := helmProvider.GetChart(string(common.CNITypeCilium))
	if ciliumChart == nil {
		return nil, "", fmt.Errorf("could not get Cilium chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := ciliumChart.LocalPath(ctx.GetGlobalWorkDir())

	return ciliumChart, destFile, nil
}

func (s *DownloadCiliumChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Infof("Cilium chart package %s already exists locally. Step is complete.", destFile)
		return true, nil
	}

	return false, nil
}

func (s *DownloadCiliumChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	ciliumChart, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Warnf("Execution was not skipped by Precheck, but chart info is still unavailable. This should not happen. Error: %v", err)
		return nil
	}

	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create local destination directory %s: %w", destDir, err)
	}

	logger.Infof("Adding Helm repo: %s (%s)", ciliumChart.RepoName(), ciliumChart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", ciliumChart.RepoName(), ciliumChart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", ciliumChart.RepoName(), err, string(output))
		}
	}

	logger.Infof("Updating Helm repo: %s", ciliumChart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", ciliumChart.RepoName())
	if output, err := repoUpdateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w\nOutput: %s", ciliumChart.RepoName(), err, string(output))
	}

	logger.Infof("Pulling Cilium chart %s (version %s) to %s", ciliumChart.FullName(), ciliumChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", ciliumChart.FullName(), "--version", ciliumChart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Cilium chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Cilium Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadCiliumChartStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*DownloadCiliumChartStep)(nil)
