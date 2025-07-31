package hybridnet

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

// DownloadHybridnetChartStep is a step to download the Hybridnet Helm chart from the network.
type DownloadHybridnetChartStep struct {
	step.Base
	HelmBinaryPath string
	ChartsDir      string
}

// DownloadHybridnetChartStepBuilder is used to build DownloadHybridnetChartStep instances.
type DownloadHybridnetChartStepBuilder struct {
	step.Builder[DownloadHybridnetChartStepBuilder, *DownloadHybridnetChartStep]
}

// NewDownloadHybridnetChartStepBuilder is the constructor for DownloadHybridnetChartStep.
func NewDownloadHybridnetChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadHybridnetChartStepBuilder {
	s := &DownloadHybridnetChartStep{
		ChartsDir: filepath.Join(ctx.GetClusterArtifactsDir(), "helm"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Hybridnet Helm chart to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadHybridnetChartStepBuilder).Init(s)
	return b
}

func (s *DownloadHybridnetChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadHybridnetChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Check if Hybridnet is enabled, if not, skip this step
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeHybridnet) {
		ctx.GetLogger().Infof("Hybridnet is not enabled, skipping download step.")
		return true, nil // isDone = true means skip
	}

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, fmt.Errorf("helm command not found in PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath
	return false, nil
}

func (s *DownloadHybridnetChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	hybridnetChart := helmProvider.GetChart(string(common.CNITypeHybridnet))
	if hybridnetChart == nil {
		return fmt.Errorf("could not get Hybridnet chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := hybridnetChart.LocalPath(s.ChartsDir)
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("Hybridnet chart package %s already exists, skipping download.", destFile)
		return nil
	}

	logger.Infof("Adding Helm repo: %s (%s)", hybridnetChart.RepoName(), hybridnetChart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", hybridnetChart.RepoName(), hybridnetChart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", hybridnetChart.RepoName(), err, string(output))
		}
	}

	logger.Infof("Updating Helm repo: %s", hybridnetChart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", hybridnetChart.RepoName())
	if output, err := repoUpdateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w\nOutput: %s", hybridnetChart.RepoName(), err, string(output))
	}

	logger.Infof("Pulling Hybridnet chart %s version %s to %s", hybridnetChart.FullName(), hybridnetChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", hybridnetChart.FullName(), "--version", hybridnetChart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Hybridnet chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Hybridnet Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadHybridnetChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for DownloadHybridnetChartStep is a no-op.")
	return nil
}

var _ step.Step = (*DownloadHybridnetChartStep)(nil)
