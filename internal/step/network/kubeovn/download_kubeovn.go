package kubeovn

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

type DownloadKubeovnChartStep struct {
	step.Base
	HelmBinaryPath string
}

type DownloadKubeovnChartStepBuilder struct {
	step.Builder[DownloadKubeovnChartStepBuilder, *DownloadKubeovnChartStep]
}

func NewDownloadKubeovnChartStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DownloadKubeovnChartStepBuilder {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeKubeOvn) {
		return nil
	}

	s := &DownloadKubeovnChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Kube-OVN Helm chart to local work directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadKubeovnChartStepBuilder).Init(s)
	return b
}

func (s *DownloadKubeovnChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadKubeovnChartStep) getChartAndPath(ctx runtime.ExecutionContext) (*helm.HelmChart, string, error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeKubeOvn) {
		return nil, "", fmt.Errorf("Kube-OVN is not the configured CNI plugin")
	}

	helmProvider := helm.NewHelmProvider(ctx)
	kubeovnChart := helmProvider.GetChart(string(common.CNITypeKubeOvn))
	if kubeovnChart == nil {
		return nil, "", fmt.Errorf("could not get Kube-OVN chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := kubeovnChart.LocalPath(ctx.GetGlobalWorkDir())

	return kubeovnChart, destFile, nil
}

func (s *DownloadKubeovnChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Infof("Kube-OVN chart package %s already exists locally. Step is complete.", destFile)
		return true, nil
	}

	return false, nil
}

func (s *DownloadKubeovnChartStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	kubeovnChart, destFile, err := s.getChartAndPath(ctx)
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

	logger.Infof("Adding Helm repo: %s (%s)", kubeovnChart.RepoName(), kubeovnChart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", kubeovnChart.RepoName(), kubeovnChart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			result.MarkFailed(err, fmt.Sprintf("failed to add helm repo '%s'", kubeovnChart.RepoName()))
			return result, err
		}
	}

	logger.Infof("Updating Helm repo: %s", kubeovnChart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", kubeovnChart.RepoName())
	if _, err := repoUpdateCmd.CombinedOutput(); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to update helm repo %s", kubeovnChart.RepoName()))
		return result, err
	}

	logger.Infof("Pulling Kube-OVN chart %s (version %s) to %s", kubeovnChart.FullName(), kubeovnChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", kubeovnChart.FullName(), "--version", kubeovnChart.Version, "--destination", destDir)
	if _, err := pullCmd.CombinedOutput(); err != nil {
		result.MarkFailed(err, "failed to pull Kube-OVN chart")
		return result, err
	}

	logger.Info("Kube-OVN Helm chart has been downloaded successfully.")
	result.MarkCompleted("Kube-OVN Helm chart downloaded successfully")
	return result, nil
}

func (s *DownloadKubeovnChartStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*DownloadKubeovnChartStep)(nil)
