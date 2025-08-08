package calico

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
	"github.com/pkg/errors" // 引入 errors 包以获得更好的错误处理
)

type DownloadCalicoChartStep struct {
	step.Base
	HelmBinaryPath string
}

type DownloadCalicoChartStepBuilder struct {
	step.Builder[DownloadCalicoChartStepBuilder, *DownloadCalicoChartStep]
}

func NewDownloadCalicoChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadCalicoChartStepBuilder {
	s := &DownloadCalicoChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Calico Helm chart to local work directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadCalicoChartStepBuilder).Init(s)
	return b
}

func (s *DownloadCalicoChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadCalicoChartStep) getChartAndPath(ctx runtime.ExecutionContext) (*helm.HelmChart, string, error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeCalico) {
		return nil, "", fmt.Errorf("Calico is not the configured CNI plugin")
	}

	helmProvider := helm.NewHelmProvider(ctx)
	calicoChart := helmProvider.GetChart(string(common.CNITypeCalico))
	if calicoChart == nil {
		return nil, "", fmt.Errorf("could not get Calico chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := calicoChart.LocalPath(ctx.GetGlobalWorkDir())

	return calicoChart, destFile, nil
}

func (s *DownloadCalicoChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Infof("Calico chart package %s already exists locally. Step is complete.", destFile)
		return true, nil
	}

	return false, nil
}

func (s *DownloadCalicoChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	calicoChart, destFile, _ := s.getChartAndPath(ctx)
	if calicoChart == nil {
		logger.Warn("Execution was not skipped by Precheck, but chart info is still nil. This should not happen.")
		return nil
	}

	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create local destination directory %s: %w", destDir, err)
	}

	logger.Infof("Adding Helm repo: %s (%s)", calicoChart.RepoName(), calicoChart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", calicoChart.RepoName(), calicoChart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", calicoChart.RepoName(), err, string(output))
		}
	}

	logger.Infof("Updating Helm repo: %s", calicoChart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", calicoChart.RepoName())
	if output, err := repoUpdateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w\nOutput: %s", calicoChart.RepoName(), err, string(output))
	}

	logger.Infof("Pulling Calico chart %s (version %s) to %s", calicoChart.FullName(), calicoChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", calicoChart.FullName(), "--version", calicoChart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Calico chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Calico Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadCalicoChartStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*DownloadCalicoChartStep)(nil)
