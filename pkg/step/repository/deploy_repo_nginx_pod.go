package repository

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type DeployRepoNginxPodStep struct {
	step.Base
	ListenPort int
	ConfigPath string
}

type DeployRepoNginxPodStepBuilder struct {
	step.Builder[DeployRepoNginxPodStepBuilder, *DeployRepoNginxPodStep]
}

func NewDeployRepoNginxPodStepBuilder(ctx runtime.Context, instanceName string) *DeployRepoNginxPodStepBuilder {
	s := &DeployRepoNginxPodStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Deploy NGINX static pod for repository server", instanceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(DeployRepoNginxPodStepBuilder).Init(s)
}

func (b *DeployRepoNginxPodStepBuilder) WithListenPort(port int) *DeployRepoNginxPodStepBuilder {
	b.Step.ListenPort = port
	return b
}

func (b *DeployRepoNginxPodStepBuilder) WithConfigPath(path string) *DeployRepoNginxPodStepBuilder {
	b.Step.ConfigPath = path
	return b
}

func (s *DeployRepoNginxPodStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DeployRepoNginxPodStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

type NginxStaticPodTemplateData struct {
	Image      string
	ConfigHash string
	ListenPort int
}

func (s *DeployRepoNginxPodStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	logger := ctx.GetLogger().With("host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil, err
	}

	imageProvider := images.NewImageProvider(ctx)
	nginxImage := imageProvider.GetImage("nginx")
	fullImageName := nginxImage.FullName()
	logger.Debugf("Using NGINX image for repository: %s", fullImageName)

	configContent, err := runner.ReadFile(ctx.GoContext(), conn, s.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read repository nginx config file at '%s' to calculate checksum: %w", s.ConfigPath, err)
	}

	configHash := fmt.Sprintf("%x", sha256.Sum256(configContent))
	logger.Debugf("Calculated checksum for '%s': %s", s.ConfigPath, configHash)

	data := NginxStaticPodTemplateData{
		Image:      fullImageName,
		ConfigHash: configHash,
		ListenPort: s.ListenPort,
	}

	// Re-use the same static pod template as the load balancer
	templateContent, err := templates.Get("loadbalancer/nginx/nginx-static-pod.yaml.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get nginx static pod template: %w", err)
	}
	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render nginx static pod template: %w", err)
	}

	return []byte(renderedConfig), nil
}

func (s *DeployRepoNginxPodStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	staticPodDir := common.KubernetesManifestsDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, staticPodDir, "0755", true); err != nil {
		return fmt.Errorf("failed to create static pod directory '%s': %w", staticPodDir, err)
	}

	remoteManifestPath := filepath.Join(staticPodDir, "kube-repo-nginx.yaml")
	logger.Infof("Uploading repository NGINX static pod manifest to %s:%s", ctx.GetHost().GetName(), remoteManifestPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteManifestPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload repository NGINX static pod manifest: %w", err)
	}

	return nil
}

func (s *DeployRepoNginxPodStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteManifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-repo-nginx.yaml")
	logger.Warnf("Rolling back by removing static pod manifest: %s", remoteManifestPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteManifestPath, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteManifestPath, err)
	}
	return nil
}

var _ step.Step = (*DeployRepoNginxPodStep)(nil)
