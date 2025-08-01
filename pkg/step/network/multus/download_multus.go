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
)

// DownloadMultusChartStep is a step to download the Multus Helm chart from the network.
type DownloadMultusChartStep struct {
	step.Base
	HelmBinaryPath string
	ChartsDir      string
}

// DownloadMultusChartStepBuilder is used to build DownloadMultusChartStep instances.
type DownloadMultusChartStepBuilder struct {
	step.Builder[DownloadMultusChartStepBuilder, *DownloadMultusChartStep]
}

// NewDownloadMultusChartStepBuilder is the constructor for DownloadMultusChartStep.
func NewDownloadMultusChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadMultusChartStepBuilder {
	s := &DownloadMultusChartStep{
		ChartsDir: filepath.Join(ctx.GetClusterArtifactsDir(), "helm"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Multus Helm chart to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadMultusChartStepBuilder).Init(s)
	return b
}

func (s *DownloadMultusChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadMultusChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if !ctx.GetClusterConfig().Spec.Network.Multus.Enabled {
		ctx.GetLogger().Infof("Multus is not enabled, skipping download step.")
		return true, nil
	}

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, fmt.Errorf("helm command not found in PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath
	return false, nil
}

func (s *DownloadMultusChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	multusChart := helmProvider.GetChart(string(common.CNITypeMultus))
	if multusChart == nil {
		return fmt.Errorf("could not get Multus chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := multusChart.LocalPath(s.ChartsDir)
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("Multus chart package %s already exists, skipping download.", destFile)
		return nil
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

	logger.Infof("Pulling Multus chart %s version %s to %s", multusChart.FullName(), multusChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", multusChart.FullName(), "--version", multusChart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Multus chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Multus Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadMultusChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for DownloadMultusChartStep is a no-op.")
	return nil
}

var _ step.Step = (*DownloadMultusChartStep)(nil)
