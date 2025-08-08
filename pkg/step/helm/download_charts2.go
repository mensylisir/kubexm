package helm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/pkg/errors"
)

type DownloadHelmCharts2Step struct {
	step.Base
	HelmBinaryPath string
	Concurrency    int
}

type DownloadHelmCharts2StepBuilder struct {
	step.Builder[DownloadHelmCharts2StepBuilder, *DownloadHelmCharts2Step]
}

func NewDownloadHelmCharts2StepBuilder(ctx runtime.Context, instanceName string) *DownloadHelmCharts2StepBuilder {
	s := &DownloadHelmCharts2Step{
		Concurrency: 5,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download all required Helm charts to local work directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 30 * time.Minute

	b := new(DownloadHelmCharts2StepBuilder).Init(s)
	return b
}

func (b *DownloadHelmCharts2StepBuilder) WithConcurrency(c int) *DownloadHelmCharts2StepBuilder {
	if c > 0 {
		b.Step.Concurrency = c
	}
	return b
}

func (s *DownloadHelmCharts2Step) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadHelmCharts2Step) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, errors.Wrap(err, "helm command not found in local PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath

	helmProvider := helm.NewHelmProvider(ctx)
	chartsToDownload := helmProvider.GetCharts()

	if len(chartsToDownload) == 0 {
		logger.Info("No Helm charts are enabled for this configuration. Step is complete.")
		return true, nil
	}

	allExist := true
	for _, chart := range chartsToDownload {
		destFile := chart.LocalPath(ctx.GetGlobalWorkDir())
		if _, err := os.Stat(destFile); os.IsNotExist(err) {
			logger.Debugf("Chart package %s does not exist. Download is required.", destFile)
			allExist = false
			break
		}
	}

	if allExist {
		logger.Info("All required Helm charts already exist locally. Step is complete.")
		return true, nil
	}

	return false, nil
}

func (s *DownloadHelmCharts2Step) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	chartsToDownload := helmProvider.GetCharts()

	addedRepos := make(map[string]bool)
	for _, chart := range chartsToDownload {
		if !addedRepos[chart.RepoName()] {
			logger.Infof("Adding Helm repo: %s (%s)", chart.RepoName(), chart.RepoURL())
			repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", chart.RepoName(), chart.RepoURL(), "--force-update")
			if output, err := repoAddCmd.CombinedOutput(); err != nil && !strings.Contains(string(output), "already exists") {
				return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", chart.RepoName(), err, string(output))
			}
			addedRepos[chart.RepoName()] = true
		}
	}
	if len(addedRepos) > 0 {
		logger.Info("Updating Helm repositories...")
		repoUpdateCmd := exec.Command(s.HelmBinaryPath, "repo", "update")
		if output, err := repoUpdateCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to update helm repos: %w\nOutput: %s", err, string(output))
		}
	}

	jobs := make(chan *helm.HelmChart, len(chartsToDownload))
	errChan := make(chan error, len(chartsToDownload))

	var wg sync.WaitGroup
	for i := 0; i < s.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chart := range jobs {
				err := s.pullChart(ctx, chart)
				if err != nil {
					logger.Errorf("Worker %d: failed to download chart %s: %v", workerID, chart.FullName(), err)
					errChan <- fmt.Errorf("failed to download chart %s: %w", chart.FullName(), err)
				}
			}
		}(i)
	}

	for _, chart := range chartsToDownload {
		jobs <- chart
	}
	close(jobs)

	wg.Wait()
	close(errChan)

	var allErrors []string
	for err := range errChan {
		allErrors = append(allErrors, err.Error())
	}
	if len(allErrors) > 0 {
		return fmt.Errorf("failed to download some helm charts:\n- %s", strings.Join(allErrors, "\n- "))
	}

	logger.Info("All required Helm charts have been downloaded successfully.")
	return nil
}

func (s *DownloadHelmCharts2Step) pullChart(ctx runtime.ExecutionContext, chart *helm.HelmChart) error {
	logger := ctx.GetLogger().With("chart", chart.FullName())

	destFile := chart.LocalPath(ctx.GetGlobalWorkDir())
	destDir := filepath.Dir(destFile)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	if _, err := os.Stat(destFile); err == nil {
		logger.Infof("Chart package already exists, skipping download.", "path", destFile)
		return nil
	}

	logger.Infof("Pulling chart version %s...", chart.Version)
	pullCmd := exec.Command(s.HelmBinaryPath, "pull", chart.FullName(), "--version", chart.Version, "--destination", destDir)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull chart: %w\nOutput: %s", err, string(output))
	}
	logger.Info("Successfully downloaded chart.")
	return nil
}

func (s *DownloadHelmCharts2Step) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	helmProvider := helm.NewHelmProvider(ctx)
	charts := helmProvider.GetCharts()

	for _, chart := range charts {
		destFile := chart.LocalPath(ctx.GetGlobalWorkDir())
		if _, statErr := os.Stat(destFile); statErr == nil {
			logger.Warnf("Rolling back by deleting locally downloaded chart file: %s", destFile)
			if err := os.Remove(destFile); err != nil {
				logger.Errorf("Failed to remove file during rollback: %v", err)
			}
		}
	}
	return nil
}

var _ step.Step = (*DownloadHelmCharts2Step)(nil)
