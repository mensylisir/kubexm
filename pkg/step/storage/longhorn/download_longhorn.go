package longhorn

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

type DownloadLonghornChartStep struct {
	step.Base
	HelmBinaryPath string
}

type DownloadLonghornChartStepBuilder struct {
	step.Builder[DownloadLonghornChartStepBuilder, *DownloadLonghornChartStep]
}

func NewDownloadLonghornChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadLonghornChartStepBuilder {
	s := &DownloadLonghornChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Download Longhorn Helm chart to the local machine's work directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadLonghornChartStepBuilder).Init(s)
	return b
}

func (s *DownloadLonghornChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func getChart(ctx runtime.ExecutionContext) (*helm.HelmChart, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("longhorn")
	if chart == nil {
		return nil, fmt.Errorf("longhorn chart is not enabled or no compatible version found in BOM for K8s version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}
	return chart, nil
}

func (s *DownloadLonghornChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, errors.Wrap(err, "helm command not found in local PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath

	chart, err := getChart(ctx)
	if err != nil {
		ctx.GetLogger().Infof("Skipping Longhorn chart download: %v", err)
		return true, nil
	}

	destFile := chart.LocalPath(ctx.GetGlobalWorkDir())
	if _, err := os.Stat(destFile); err == nil {
		ctx.GetLogger().Infof("Longhorn chart package %s already exists locally.", destFile)
		return true, nil
	}

	return false, nil
}

func (s *DownloadLonghornChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	chart, err := getChart(ctx)
	if err != nil {
		logger.Warnf("Could not execute step (was supposed to be skipped by Precheck): %v", err)
		return nil
	}

	destFile := chart.LocalPath(ctx.GetGlobalWorkDir())
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create local destination directory %s: %w", destDir, err)
	}

	logger.Infof("Adding Helm repo: %s (%s)", chart.RepoName(), chart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", chart.RepoName(), chart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", chart.RepoName(), err, string(output))
		}
	}

	logger.Infof("Updating Helm repo: %s", chart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", chart.RepoName())
	if output, err := repoUpdateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update helm repo '%s': %w\nOutput: %s", chart.RepoName(), err, string(output))
	}

	logger.Infof("Pulling Longhorn chart %s version %s to %s", chart.FullName(), chart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", chart.FullName(), "--version", chart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Longhorn chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Longhorn Helm chart has been downloaded successfully to the local machine.")
	return nil
}

func (s *DownloadLonghornChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	if chart, err := getChart(ctx); err == nil {
		destFile := chart.LocalPath(ctx.GetGlobalWorkDir())
		if _, statErr := os.Stat(destFile); statErr == nil {
			logger.Infof("Rolling back by deleting locally downloaded chart file: %s", destFile)
			os.Remove(destFile)
		}
	}
	return nil
}

var _ step.Step = (*DownloadLonghornChartStep)(nil)
