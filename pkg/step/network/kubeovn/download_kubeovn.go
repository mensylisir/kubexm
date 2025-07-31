package kubeovn

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

// DownloadKubeovnChartStep is a step to download the Kube-OVN Helm chart from the network.
type DownloadKubeovnChartStep struct {
	step.Base
	HelmBinaryPath string
	ChartsDir      string
}

// DownloadKubeovnChartStepBuilder is used to build DownloadKubeovnChartStep instances.
type DownloadKubeovnChartStepBuilder struct {
	step.Builder[DownloadKubeovnChartStepBuilder, *DownloadKubeovnChartStep]
}

// NewDownloadKubeovnChartStepBuilder is the constructor for DownloadKubeovnChartStep.
func NewDownloadKubeovnChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadKubeovnChartStepBuilder {
	s := &DownloadKubeovnChartStep{
		ChartsDir: filepath.Join(ctx.GetClusterArtifactsDir(), "helm"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Kube-OVN Helm chart to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadKubeovnChartStepBuilder).Init(s)
	return b
}

func (s *DownloadKubeovnChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadKubeovnChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeKubeOVN) {
		ctx.GetLogger().Infof("Kube-OVN is not enabled, skipping download step.")
		return true, nil
	}

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, fmt.Errorf("helm command not found in PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath
	return false, nil
}

func (s *DownloadKubeovnChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	kubeovnChart := helmProvider.GetChart(string(common.CNITypeKubeOVN))
	if kubeovnChart == nil {
		return fmt.Errorf("could not get Kube-OVN chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := kubeovnChart.LocalPath(s.ChartsDir)
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("Kube-OVN chart package %s already exists, skipping download.", destFile)
		return nil
	}

	logger.Infof("Adding Helm repo: %s (%s)", kubeovnChart.RepoName(), kubeovnChart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", kubeovnChart.RepoName(), kubeovnChart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", kubeovnChart.RepoName(), err, string(output))
		}
	}

	logger.Infof("Updating Helm repo: %s", kubeovnChart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", kubeovnChart.RepoName())
	if output, err := repoUpdateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w\nOutput: %s", kubeovnChart.RepoName(), err, string(output))
	}

	logger.Infof("Pulling Kube-OVN chart %s version %s to %s", kubeovnChart.FullName(), kubeovnChart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", kubeovnChart.FullName(), "--version", kubeovnChart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Kube-OVN chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Kube-OVN Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadKubeovnChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for DownloadKubeovnChartStep is a no-op.")
	return nil
}

var _ step.Step = (*DownloadKubeovnChartStep)(nil)
