package hybridnet

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/util/helm"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

type DownloadHybridnetChartStep struct {
	step.Base
	HelmBinaryPath string
}

type DownloadHybridnetChartStepBuilder struct {
	step.Builder[DownloadHybridnetChartStepBuilder, *DownloadHybridnetChartStep]
}

func NewDownloadHybridnetChartStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DownloadHybridnetChartStepBuilder {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeHybridnet) {
		return nil
	}

	s := &DownloadHybridnetChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Hybridnet Helm chart to local work directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadHybridnetChartStepBuilder).Init(s)
	return b
}

func (s *DownloadHybridnetChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadHybridnetChartStep) getChartAndPath(ctx runtime.ExecutionContext) (*helm.HelmChart, string, error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeHybridnet) {
		return nil, "", fmt.Errorf("Hybridnet is not the configured CNI plugin")
	}

	helmProvider := helm.NewHelmProvider(ctx)
	hybridnetChart := helmProvider.GetChart(string(common.CNITypeHybridnet))
	if hybridnetChart == nil {
		return nil, "", fmt.Errorf("could not get Hybridnet chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := hybridnetChart.LocalPath(ctx.GetGlobalWorkDir())

	return hybridnetChart, destFile, nil
}

func (s *DownloadHybridnetChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Infof("Hybridnet chart package %s already exists locally. Step is complete.", destFile)
		return true, nil
	}

	return false, nil
}

func (s *DownloadHybridnetChartStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	hybridnetChart, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Warnf("Execution was not skipped by Precheck, but chart info is still unavailable. Error: %v", err)
		result.MarkCompleted("Chart info unavailable, skipping")
		return result, nil
	}

	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to create local destination directory %s", destDir))
		return result, err
	}

	logger.Infof("Adding Helm repo: %s (%s)", hybridnetChart.RepoName(), hybridnetChart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", hybridnetChart.RepoName(), hybridnetChart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			result.MarkFailed(err, fmt.Sprintf("failed to add helm repo '%s'", hybridnetChart.RepoName()))
			return result, err
		}
	}

	logger.Infof("Updating Helm repo: %s", hybridnetChart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", hybridnetChart.RepoName())
	if _, err := repoUpdateCmd.CombinedOutput(); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to update helm repo %s", hybridnetChart.RepoName()))
		return result, err
	}

	logger.Infof("Pulling Hybridnet chart %s (version %s) to %s", hybridnetChart.FullName(), hybridnetChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", hybridnetChart.FullName(), "--version", hybridnetChart.Version, "--destination", destDir)
	if _, err := pullCmd.CombinedOutput(); err != nil {
		result.MarkFailed(err, "failed to pull Hybridnet chart")
		return result, err
	}

	logger.Info("Hybridnet Helm chart has been downloaded successfully.")
	result.MarkCompleted("Hybridnet Helm chart downloaded successfully")
	return result, nil
}

func (s *DownloadHybridnetChartStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*DownloadHybridnetChartStep)(nil)
