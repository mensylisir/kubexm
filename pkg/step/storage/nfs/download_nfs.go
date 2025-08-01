package nfs

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

// DownloadNFSProvisionerChartStep is a step to download the NFS Subdir External Provisioner Helm chart.
type DownloadNFSProvisionerChartStep struct {
	step.Base
	HelmBinaryPath string
	ChartsDir      string
}

// DownloadNFSProvisionerChartStepBuilder is used to build DownloadNFSProvisionerChartStep instances.
type DownloadNFSProvisionerChartStepBuilder struct {
	step.Builder[DownloadNFSProvisionerChartStepBuilder, *DownloadNFSProvisionerChartStep]
}

// NewDownloadNFSProvisionerChartStepBuilder is the constructor.
func NewDownloadNFSProvisionerChartStepBuilder(ctx runtime.Context, instanceName string) *DownloadNFSProvisionerChartStepBuilder {
	s := &DownloadNFSProvisionerChartStep{
		ChartsDir: filepath.Join(ctx.GetClusterArtifactsDir(), "helm"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download NFS Provisioner Helm chart to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadNFSProvisionerChartStepBuilder).Init(s)
	return b
}

func (s *DownloadNFSProvisionerChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadNFSProvisionerChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// TODO: Add a check to see if nfs provisioner is enabled in the cluster config.
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, fmt.Errorf("helm command not found in PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath
	return false, nil
}

func (s *DownloadNFSProvisionerChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("nfs-subdir-external-provisioner")
	if chart == nil {
		return fmt.Errorf("could not get nfs-subdir-external-provisioner chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := chart.LocalPath(s.ChartsDir)
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("NFS Provisioner chart package %s already exists, skipping download.", destFile)
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

	logger.Infof("Pulling NFS Provisioner chart %s version %s to %s", chart.FullName(), chart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", chart.FullName(), "--version", chart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull NFS Provisioner chart: %w\nOutput: %s", err, string(output))
	}

	logger.Info("NFS Provisioner Helm chart has been downloaded successfully.")
	return nil
}

func (s *DownloadNFSProvisionerChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for DownloadNFSProvisionerChartStep is a no-op.")
	return nil
}

var _ step.Step = (*DownloadNFSProvisionerChartStep)(nil)
