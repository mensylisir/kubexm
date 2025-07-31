package openebslocal

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
)

// DownloadOpenEBSChartStep is a step to download the OpenEBS Helm chart.
type DownloadOpenEBSChartStep struct {
	step.Base
	HelmBinaryPath string
	ChartsDir      string
}

// DownloadOpenEBSChartStepBuilder is used to build DownloadOpenEBSChartStep instances.
type DownloadOpenEBSChartStepBuilder struct {
	step.Builder[DownloadOpenEBSChartStepBuilder, *DownloadOpenEBSChartStep]
}

// NewDownloadOpenEBSChartStepBuilder is the constructor for DownloadOpenEBSChartStep.
func NewDownloadOpenEBSChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadOpenEBSChartStepBuilder {
	s := &DownloadOpenEBSChartStep{
		ChartsDir: filepath.Join(ctx.GetClusterArtifactsDir(), "helm"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download OpenEBS Helm chart to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadOpenEBSChartStepBuilder).Init(s)
	return b
}

func (s *DownloadOpenEBSChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadOpenEBSChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// TODO: Add a check to see if openebs is enabled in the cluster config.
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, fmt.Errorf("helm command not found in PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath
	return false, nil
}

func (s *DownloadOpenEBSChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("openebs")
	if chart == nil {
		return fmt.Errorf("could not get openebs chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := chart.LocalPath(s.ChartsDir)
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("OpenEBS chart package %s already exists, skipping download.", destFile)
		return nil
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
		return fmt.Errorf("failed to update helm repo %s: %w\nOutput: %s", chart.RepoName(), err, string(output))
	}

	logger.Infof("Pulling OpenEBS chart %s version %s to %s", chart.FullName(), chart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", chart.FullName(), "--version", chart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull OpenEBS chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("OpenEBS Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadOpenEBSChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for DownloadOpenEBSChartStep is a no-op.")
	return nil
}

var _ step.Step = (*DownloadOpenEBSChartStep)(nil)
