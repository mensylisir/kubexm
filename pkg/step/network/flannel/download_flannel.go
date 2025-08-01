package flannel

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

// DownloadFlannelChartStep is a step to download the Flannel Helm chart from the network.
type DownloadFlannelChartStep struct {
	step.Base
	HelmBinaryPath string
	ChartsDir      string
}

// DownloadFlannelChartStepBuilder is used to build DownloadFlannelChartStep instances.
type DownloadFlannelChartStepBuilder struct {
	step.Builder[DownloadFlannelChartStepBuilder, *DownloadFlannelChartStep]
}

// NewDownloadFlannelChartStepBuilder is the constructor for DownloadFlannelChartStep.
func NewDownloadFlannelChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadFlannelChartStepBuilder {
	s := &DownloadFlannelChartStep{
		ChartsDir: filepath.Join(ctx.GetClusterArtifactsDir(), "helm"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Flannel Helm chart to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadFlannelChartStepBuilder).Init(s)
	return b
}

func (s *DownloadFlannelChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadFlannelChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Check if Flannel is enabled, if not, skip this step
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeFlannel) {
		ctx.GetLogger().Infof("Flannel is not enabled, skipping download step.")
		return true, nil // isDone = true means skip
	}

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, fmt.Errorf("helm command not found in PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath
	return false, nil
}

func (s *DownloadFlannelChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	flannelChart := helmProvider.GetChart(string(common.CNITypeFlannel))
	if flannelChart == nil {
		return fmt.Errorf("could not get Flannel chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := flannelChart.LocalPath(s.ChartsDir)
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("Flannel chart package %s already exists, skipping download.", destFile)
		return nil
	}

	logger.Infof("Adding Helm repo: %s (%s)", flannelChart.RepoName(), flannelChart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", flannelChart.RepoName(), flannelChart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", flannelChart.RepoName(), err, string(output))
		}
	}

	logger.Infof("Updating Helm repo: %s", flannelChart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", flannelChart.RepoName())
	if output, err := repoUpdateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w\nOutput: %s", flannelChart.RepoName(), err, string(output))
	}

	logger.Infof("Pulling Flannel chart %s version %s to %s", flannelChart.FullName(), flannelChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", flannelChart.FullName(), "--version", flannelChart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Flannel chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Flannel Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadFlannelChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for DownloadFlannelChartStep is a no-op.")
	return nil
}

var _ step.Step = (*DownloadFlannelChartStep)(nil)
