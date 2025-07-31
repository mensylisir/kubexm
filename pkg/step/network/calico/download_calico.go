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
)

// DownloadCalicoChartStep 负责从网络上下载 Calico Helm Chart。
type DownloadCalicoChartStep struct {
	step.Base
	HelmBinaryPath string
	ChartsDir      string
}

// DownloadCalicoChartStepBuilder 用于构建 DownloadCalicoChartStep 实例。
type DownloadCalicoChartStepBuilder struct {
	step.Builder[DownloadCalicoChartStepBuilder, *DownloadCalicoChartStep]
}

// NewDownloadCalicoChartStepBuilder 是 DownloadCalicoChartStep 的构造函数。
func NewDownloadCalicoChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadCalicoChartStepBuilder {
	s := &DownloadCalicoChartStep{
		ChartsDir: filepath.Join(ctx.GetClusterArtifactsDir(), "helm"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Calico Helm chart to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadCalicoChartStepBuilder).Init(s)
	return b
}

func (s *DownloadCalicoChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadCalicoChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// 检查 Calico 是否启用，如果未启用，则跳过此步骤
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeCalico) {
		ctx.GetLogger().Infof("Calico is not enabled, skipping download step.")
		return true, nil // isDone = true 表示跳过
	}

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, fmt.Errorf("helm command not found in PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath
	return false, nil
}

func (s *DownloadCalicoChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	calicoChart := helmProvider.GetChart(string(common.CNITypeCalico))
	if calicoChart == nil {
		return fmt.Errorf("could not get Calico chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := calicoChart.LocalPath(s.ChartsDir)
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("Calico chart package %s already exists, skipping download.", destFile)
		return nil
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

	logger.Infof("Pulling Calico chart %s version %s to %s", calicoChart.FullName(), calicoChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", calicoChart.FullName(), "--version", calicoChart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Calico chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Calico Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadCalicoChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for DownloadCalicoChartStep is a no-op.")
	return nil
}

var _ step.Step = (*DownloadCalicoChartStep)(nil)
