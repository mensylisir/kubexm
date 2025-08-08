package addon

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type artifactToDownload struct {
	LocalPath    string
	IsChart      bool
	ChartRepoURL string
	ChartName    string
	ChartVersion string

	IsYaml  bool
	YamlURL string
}

type DownloadAddonArtifactsStep struct {
	step.Base
	AddonName      string
	HelmBinaryPath string
}

type DownloadAddonArtifactsStepBuilder struct {
	step.Builder[DownloadAddonArtifactsStepBuilder, *DownloadAddonArtifactsStep]
}

func NewDownloadAddonArtifactsStepBuilder(ctx runtime.Context, addonName string) *DownloadAddonArtifactsStepBuilder {
	var targetAddon *v1alpha1.Addon
	for i := range ctx.GetClusterConfig().Spec.Addons {
		if ctx.GetClusterConfig().Spec.Addons[i].Name == addonName {
			targetAddon = &ctx.GetClusterConfig().Spec.Addons[i]
			break
		}
	}
	if targetAddon == nil || (targetAddon.Enabled != nil && !*targetAddon.Enabled) {
		return nil
	}

	s := &DownloadAddonArtifactsStep{
		AddonName: addonName,
	}
	s.Base.Meta.Name = fmt.Sprintf("DownloadAddonArtifacts-%s", addonName)
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download all artifacts for addon '%s'", s.Base.Meta.Name, addonName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DownloadAddonArtifactsStepBuilder).Init(s)
	return b
}

func (s *DownloadAddonArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadAddonArtifactsStep) getArtifactsToDownload(ctx runtime.ExecutionContext) ([]artifactToDownload, error) {
	var artifacts []artifactToDownload
	cfg := ctx.GetClusterConfig()

	var targetAddon *v1alpha1.Addon
	for i := range cfg.Spec.Addons {
		if cfg.Spec.Addons[i].Name == s.AddonName {
			targetAddon = &cfg.Spec.Addons[i]
			break
		}
	}
	if targetAddon == nil {
		return nil, fmt.Errorf("addon '%s' not found in cluster spec", s.AddonName)
	}

	for i, source := range targetAddon.Sources {
		if source.Chart != nil {
			chart := source.Chart
			chartFileName := fmt.Sprintf("%s-%s.tgz", chart.Name, chart.Version)
			localPath := filepath.Join(ctx.GetGlobalWorkDir(), "helm", cfg.Spec.Kubernetes.Version, s.AddonName, chartFileName)
			artifacts = append(artifacts, artifactToDownload{
				LocalPath:    localPath,
				IsChart:      true,
				ChartRepoURL: chart.Repo,
				ChartName:    chart.Name,
				ChartVersion: chart.Version,
			})
		}
		if source.Yaml != nil {
			for j, yamlPath := range source.Yaml.Path {
				if _, err := url.ParseRequestURI(yamlPath); err == nil {
					fileName := filepath.Base(yamlPath)
					localPath := filepath.Join(ctx.GetGlobalWorkDir(), "addons", s.AddonName, fmt.Sprintf("source-%d", i), fmt.Sprintf("yaml-%d-%s", j, fileName))
					artifacts = append(artifacts, artifactToDownload{
						LocalPath: localPath,
						IsYaml:    true,
						YamlURL:   yamlPath,
					})
				}
			}
		}
	}
	return artifacts, nil
}

func (s *DownloadAddonArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, errors.Wrap(err, "helm command not found in local PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath

	artifacts, err := s.getArtifactsToDownload(ctx)
	if err != nil {
		logger.Infof("Skipping step, could not determine artifacts: %v", err)
		return true, nil
	}

	if len(artifacts) == 0 {
		logger.Info("No remote artifacts to download for this addon. Step is complete.")
		return true, nil
	}

	for _, artifact := range artifacts {
		if _, err := os.Stat(artifact.LocalPath); os.IsNotExist(err) {
			logger.Debugf("Artifact %s does not exist. Download is required.", artifact.LocalPath)
			return false, nil
		}
	}

	logger.Info("All required artifacts for this addon already exist locally. Step is complete.")
	return true, nil
}

func (s *DownloadAddonArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	artifacts, err := s.getArtifactsToDownload(ctx)
	if err != nil {
		return err
	}

	for _, artifact := range artifacts {
		if _, err := os.Stat(artifact.LocalPath); err == nil {
			logger.Infof("Artifact %s already exists, skipping download.", artifact.LocalPath)
			continue
		}

		destDir := filepath.Dir(artifact.LocalPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
		}

		if artifact.IsChart {
			repoName := s.AddonName
			repoURL := artifact.ChartRepoURL
			fullName := fmt.Sprintf("%s/%s", repoName, artifact.ChartName)

			logger.Infof("Adding Helm repo: %s (%s)", repoName, repoURL)
			repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", repoName, repoURL, "--force-update")
			if output, err := repoAddCmd.CombinedOutput(); err != nil && !strings.Contains(string(output), "already exists") {
				return fmt.Errorf("failed to add helm repo '%s': %w\nOutput: %s", repoName, err, string(output))
			}

			logger.Infof("Pulling chart %s version %s to %s", fullName, artifact.ChartVersion, destDir)
			pullCmd := exec.Command(s.HelmBinaryPath, "pull", fullName, "--version", artifact.ChartVersion, "--destination", destDir)
			if output, err := pullCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to pull chart: %w\nOutput: %s", err, string(output))
			}
		}

		if artifact.IsYaml {
			logger.Infof("Downloading YAML from %s to %s", artifact.YamlURL, artifact.LocalPath)
			resp, err := http.Get(artifact.YamlURL)
			if err != nil {
				return errors.Wrapf(err, "failed to download yaml from %s", artifact.YamlURL)
			}
			defer resp.Body.Close()

			out, err := os.Create(artifact.LocalPath)
			if err != nil {
				return errors.Wrapf(err, "failed to create local file %s", artifact.LocalPath)
			}
			defer out.Close()

			_, err = io.Copy(out, resp.Body)
			if err != nil {
				return errors.Wrap(err, "failed to write yaml content to file")
			}
		}
	}

	logger.Info("All required addon artifacts have been downloaded successfully.")
	return nil
}

func (s *DownloadAddonArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	artifacts, err := s.getArtifactsToDownload(ctx)
	if err != nil {
		logger.Infof("Skipping rollback as artifacts could not be determined: %v", err)
		return nil
	}

	for _, artifact := range artifacts {
		if _, statErr := os.Stat(artifact.LocalPath); statErr == nil {
			logger.Warnf("Rolling back by deleting locally downloaded artifact: %s", artifact.LocalPath)
			if err := os.Remove(artifact.LocalPath); err != nil {
				logger.Errorf("Failed to remove file during rollback: %v", err)
			}
		}
	}

	return nil
}

var _ step.Step = (*DownloadAddonArtifactsStep)(nil)
