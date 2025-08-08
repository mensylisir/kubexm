package addon

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/pkg/errors"
)

type artifactToDistribute struct {
	LocalPath  string
	RemotePath string
}

type DistributeAddonArtifactsStep struct {
	step.Base
	AddonName string
}

type DistributeAddonArtifactsStepBuilder struct {
	step.Builder[DistributeAddonArtifactsStepBuilder, *DistributeAddonArtifactsStep]
}

func NewDistributeAddonArtifactsStepBuilder(ctx runtime.Context, addonName string) *DistributeAddonArtifactsStepBuilder {
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

	s := &DistributeAddonArtifactsStep{
		AddonName: addonName,
	}
	s.Base.Meta.Name = fmt.Sprintf("DistributeAddonArtifacts-%s", addonName)
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute all artifacts for addon '%s' to remote hosts", s.Base.Meta.Name, s.AddonName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(DistributeAddonArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeAddonArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeAddonArtifactsStep) getArtifactsToDistribute(ctx runtime.ExecutionContext) ([]artifactToDistribute, error) {
	var artifacts []artifactToDistribute
	cfg := ctx.GetClusterConfig()

	var targetAddon *v1alpha1.Addon
	for i := range cfg.Spec.Addons {
		if cfg.Spec.Addons[i].Name == s.AddonName {
			targetAddon = &cfg.Spec.Addons[i]
			break
		}
	}
	if targetAddon == nil {
		return nil, fmt.Errorf("addon '%s' not found", s.AddonName)
	}

	for i, source := range targetAddon.Sources {
		if source.Chart != nil {
			chart := source.Chart
			chartFileName := fmt.Sprintf("%s-%s.tgz", chart.Name, chart.Version)
			localChartPath := filepath.Join(ctx.GetGlobalWorkDir(), "helm", cfg.Spec.Kubernetes.Version, s.AddonName, chartFileName)
			remoteChartPath := filepath.Join(ctx.GetUploadDir(), s.AddonName, chart.Version, chartFileName)
			artifacts = append(artifacts, artifactToDistribute{LocalPath: localChartPath, RemotePath: remoteChartPath})

			if chart.ValuesFile != "" {
				localValuesPath := chart.ValuesFile
				remoteValuesFileName := fmt.Sprintf("%s-values-%d.yaml", s.AddonName, i)
				remoteValuesPath := filepath.Join(ctx.GetUploadDir(), s.AddonName, chart.Version, remoteValuesFileName)
				artifacts = append(artifacts, artifactToDistribute{LocalPath: localValuesPath, RemotePath: remoteValuesPath})
			}
		}
		if source.Yaml != nil {
			for j, yamlPath := range source.Yaml.Path {
				var localYamlPath string
				if _, err := url.ParseRequestURI(yamlPath); err == nil {
					fileName := filepath.Base(yamlPath)
					localYamlPath = filepath.Join(ctx.GetGlobalWorkDir(), "addons", s.AddonName, fmt.Sprintf("source-%d", i), fmt.Sprintf("yaml-%d-%s", j, fileName))
				} else {
					localYamlPath = yamlPath
				}

				remoteYamlFileName := fmt.Sprintf("%s-yaml-%d-%d.yaml", s.AddonName, i, j)
				remoteYamlPath := filepath.Join(ctx.GetUploadDir(), s.AddonName, source.Yaml.Version, remoteYamlFileName)
				artifacts = append(artifacts, artifactToDistribute{LocalPath: localYamlPath, RemotePath: remoteYamlPath})
			}
		}
	}
	return artifacts, nil
}

func (s *DistributeAddonArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	artifacts, err := s.getArtifactsToDistribute(ctx)
	if err != nil {
		return false, err
	}
	if len(artifacts) == 0 {
		logger.Info("No artifacts to distribute for this addon. Step is complete.")
		return true, nil
	}

	allDone := true
	for _, artifact := range artifacts {
		if _, err := os.Stat(artifact.LocalPath); os.IsNotExist(err) {
			return false, errors.Wrapf(err, "local source artifact not found: %s", artifact.LocalPath)
		}

		isUpToDate, err := helpers.CheckRemoteFileIntegrity(ctx, artifact.LocalPath, artifact.RemotePath, s.Sudo)
		if err != nil {
			return false, err
		}
		if !isUpToDate {
			allDone = false
		}
	}

	if allDone {
		logger.Info("All artifacts for addon are already distributed and up-to-date. Skipping.")
	}
	return allDone, nil
}

func (s *DistributeAddonArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	artifacts, err := s.getArtifactsToDistribute(ctx)
	if err != nil {
		return err
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	for _, artifact := range artifacts {
		isUpToDate, _ := helpers.CheckRemoteFileIntegrity(ctx, artifact.LocalPath, artifact.RemotePath, s.Sudo)
		if isUpToDate {
			logger.Debugf("Artifact %s is already up-to-date on remote, skipping upload.", filepath.Base(artifact.LocalPath))
			continue
		}

		logger.Infof("Uploading %s to %s:%s", artifact.LocalPath, ctx.GetHost().GetName(), artifact.RemotePath)

		content, err := os.ReadFile(artifact.LocalPath)
		if err != nil {
			return errors.Wrapf(err, "failed to read local artifact %s", artifact.LocalPath)
		}

		remoteDir := filepath.Dir(artifact.RemotePath)
		if err := ctx.GetRunner().Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
			return errors.Wrapf(err, "failed to create remote directory %s", remoteDir)
		}

		if err := helpers.WriteContentToRemote(ctx, conn, string(content), artifact.RemotePath, "0644", s.Sudo); err != nil {
			return errors.Wrapf(err, "failed to upload artifact to %s", artifact.RemotePath)
		}
	}

	logger.Info("Successfully distributed all artifacts for addon.")
	return nil
}

func (s *DistributeAddonArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	remoteAddonDir := filepath.Join(ctx.GetUploadDir(), s.AddonName)
	logger.Warnf("Rolling back by removing remote addon artifacts directory: %s", remoteAddonDir)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	if err := runner.Remove(ctx.GoContext(), conn, remoteAddonDir, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove remote artifacts directory '%s' during rollback: %v", remoteAddonDir, err)
	}
	return nil
}

var _ step.Step = (*DistributeAddonArtifactsStep)(nil)
