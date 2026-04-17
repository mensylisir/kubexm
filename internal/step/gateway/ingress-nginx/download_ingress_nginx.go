package ingressnginx

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
	"github.com/mensylisir/kubexm/internal/util/helm"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

type DownloadIngressNginxChartStep struct {
	step.Base
	HelmBinaryPath string
}

type DownloadIngressNginxChartStepBuilder struct {
	step.Builder[DownloadIngressNginxChartStepBuilder, *DownloadIngressNginxChartStep]
}

func NewDownloadIngressNginxChartStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DownloadIngressNginxChartStepBuilder {
	s := &DownloadIngressNginxChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download Ingress-Nginx Helm chart to local work directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadIngressNginxChartStepBuilder).Init(s)
	return b
}

func (s *DownloadIngressNginxChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadIngressNginxChartStep) getChartAndPath(ctx runtime.ExecutionContext) (*helm.HelmChart, string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("ingress-nginx")
	if chart == nil {
		return nil, "", fmt.Errorf("could not get Ingress-Nginx chart info for Kubernetes version %s", ctx.GetClusterConfig().Spec.Kubernetes.Version)
	}

	destFile := chart.LocalPath(ctx.GetGlobalWorkDir())

	return chart, destFile, nil
}

func (s *DownloadIngressNginxChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, errors.Wrap(err, "helm command not found in local PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath

	_, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Infof("Skipping step: %v", err)
		return true, nil
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("Ingress-Nginx chart package %s already exists locally. Step is complete.", destFile)
		return true, nil
	}

	return false, nil
}

func (s *DownloadIngressNginxChartStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	chart, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Warnf("Execution was not skipped by Precheck, but chart info is still unavailable. Error: %v", err)
		result.MarkCompleted("chart info unavailable, skipping")
		return result, nil
	}

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

	logger.Infof("Pulling Ingress-Nginx chart %s (version %s) to %s", chart.FullName(), chart.Version, destDir)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", chart.FullName(), "--version", chart.Version, "--destination", destDir)
	if _, err := pullCmd.CombinedOutput(); err != nil {
		result.MarkFailed(err, "failed to pull Ingress-Nginx chart")
		return result, err
	}

	logger.Info("Ingress-Nginx Helm chart has been downloaded successfully.")
	result.MarkCompleted("Ingress-Nginx Helm chart downloaded successfully")
	return result, nil
}

func (s *DownloadIngressNginxChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	_, destFile, err := s.getChartAndPath(ctx)
	if err != nil {
		logger.Infof("Skipping rollback as no chart path could be determined: %v", err)
		return nil
	}

	if _, statErr := os.Stat(destFile); statErr == nil {
		logger.Warnf("Rolling back by deleting locally downloaded chart file: %s", destFile)
		if err := os.Remove(destFile); err != nil {
			logger.Errorf("Failed to remove file during rollback: %v", err)
		}
	} else {
		logger.Infof("Rollback unnecessary, file to be deleted does not exist: %s", destFile)
	}

	return nil
}

var _ step.Step = (*DownloadIngressNginxChartStep)(nil)
