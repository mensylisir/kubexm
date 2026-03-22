package nfs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

const NfsChartName = "nfs-subdir-external-provisioner"

type DownloadNFSProvisionerChartStep struct {
	step.Base
	HelmBinaryPath string
}

type DownloadNFSProvisionerChartStepBuilder struct {
	step.Builder[DownloadNFSProvisionerChartStepBuilder, *DownloadNFSProvisionerChartStep]
}

func NewDownloadNFSProvisionerChartStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DownloadNFSProvisionerChartStepBuilder {
	s := &DownloadNFSProvisionerChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Download NFS Provisioner Helm chart to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadNFSProvisionerChartStepBuilder).Init(s)
	return b
}

func (s *DownloadNFSProvisionerChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func getChart(ctx runtime.ExecutionContext) (*helm.HelmChart, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(NfsChartName)
	if chart == nil {
		return nil, fmt.Errorf("NFS Provisioner chart is not enabled or no compatible version found in BOM for K8s version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}
	return chart, nil
}

func (s *DownloadNFSProvisionerChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, errors.Wrap(err, "helm command not found in local PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath

	chart, err := getChart(ctx)
	if err != nil {
		ctx.GetLogger().Infof("Skipping NFS Provisioner chart download: %v", err)
		return true, nil
	}

	destFile := chart.LocalPath(ctx.GetGlobalWorkDir())
	if _, err := os.Stat(destFile); err == nil {
		ctx.GetLogger().Infof("NFS Provisioner chart package %s already exists locally.", destFile)
		return true, nil
	}

	return false, nil
}

func (s *DownloadNFSProvisionerChartStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	chart, err := getChart(ctx)
	if err != nil {
		logger.Warnf("Could not execute step (was supposed to be skipped by Precheck): %v", err)
		result.MarkCompleted("step skipped - chart not available")
		return result, nil
	}

	destFile := chart.LocalPath(ctx.GetGlobalWorkDir())
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		result.MarkFailed(err, "failed to create local destination directory")
		return result, err
	}

	logger.Infof("Adding Helm repo: %s (%s)", chart.RepoName(), chart.RepoURL())
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", chart.RepoName(), chart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			result.MarkFailed(err, "failed to add helm repo")
			return result, err
		}
	}

	logger.Infof("Updating Helm repo: %s", chart.RepoName())
	repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update", chart.RepoName())
	if _, err := repoUpdateCmd.CombinedOutput(); err != nil {
		result.MarkFailed(err, "failed to update helm repo")
		return result, err
	}

	logger.Infof("Pulling NFS Provisioner chart %s version %s to %s", chart.FullName(), chart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", chart.FullName(), "--version", chart.Version, "--destination", destDir)
	if _, err := pullCmd.CombinedOutput(); err != nil {
		result.MarkFailed(err, "failed to pull NFS Provisioner chart")
		return result, err
	}

	logger.Info("NFS Provisioner Helm chart has been downloaded successfully.")
	result.MarkCompleted("NFS Provisioner chart downloaded successfully")
	return result, nil
}

func (s *DownloadNFSProvisionerChartStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*DownloadNFSProvisionerChartStep)(nil)
