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
)

// DownloadCiliumChartStep 负责从网络上下载 Cilium Helm Chart。
type DownloadCiliumChartStep struct {
	step.Base
	HelmBinaryPath string
	ChartsDir      string
}

// DownloadCiliumChartStepBuilder 用于构建 DownloadCiliumChartStep 实例。
type DownloadCiliumChartStepBuilder struct {
	step.Builder[DownloadCiliumChartStepBuilder, *DownloadCiliumChartStep]
}

// NewDownloadCiliumChartStepBuilder 是 DownloadCiliumChartStep 的构造函数。
func NewDownloadCiliumChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadCiliumChartStepBuilder {
	s := &DownloadCiliumChartStep{
		ChartsDir: filepath.Join(ctx.GetClusterArtifactsDir(), "helm"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Cilium Helm chart to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadCiliumChartStepBuilder).Init(s)
	return b
}

func (s *DownloadCiliumChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadCiliumChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeCilium) {
		ctx.GetLogger().Infof("Cilium is not enabled, skipping download step.")
		return true, nil
	}

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, fmt.Errorf("helm command not found in PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath
	return false, nil
}

func (s *DownloadCiliumChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	ciliumChart := helmProvider.GetChart(string(common.CNITypeCilium))
	if ciliumChart == nil {
		return fmt.Errorf("could not get Cilium chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := ciliumChart.LocalPath(s.ChartsDir)
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("Cilium chart package %s already exists, skipping download.", destFile)
		return nil
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

	logger.Infof("Pulling Cilium chart %s version %s to %s", ciliumChart.FullName(), ciliumChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", ciliumChart.FullName(), "--version", ciliumChart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Cilium chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Cilium Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadCiliumChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for DownloadCiliumChartStep is a no-op.")
	return nil
}

var _ step.Step = (*DownloadCiliumChartStep)(nil)
