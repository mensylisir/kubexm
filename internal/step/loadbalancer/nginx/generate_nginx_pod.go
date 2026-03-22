package nginx

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

type GenerateNginxStaticPodStep struct {
	step.Base
}
type GenerateNginxStaticPodStepBuilder struct {
	step.Builder[GenerateNginxStaticPodStepBuilder, *GenerateNginxStaticPodStep]
}

func NewGenerateNginxStaticPodStepBuilder(ctx runtime.ExecutionContext, instanceName string) *GenerateNginxStaticPodStepBuilder {
	s := &GenerateNginxStaticPodStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate NGINX Static Pod manifest", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateNginxStaticPodStepBuilder).Init(s)
	return b
}

func (s *GenerateNginxStaticPodStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type NginxStaticPodTemplateData struct {
	Image      string
	ConfigHash string
	ListenPort int
}

func (s *GenerateNginxStaticPodStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	logger := ctx.GetLogger().With("host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil, err
	}
	cluster := ctx.GetClusterConfig()

	imageProvider := images.NewImageProvider(ctx)
	nginxImage := imageProvider.GetImage("nginx")
	fullImageName := nginxImage.FullName()
	logger.Debugf("Using NGINX image: %s", fullImageName)

	nginxConfigPath := common.DefaultNginxConfigFilePath
	configContent, err := runner.ReadFile(ctx.GoContext(), conn, nginxConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read nginx config file at '%s' to calculate checksum: %w", nginxConfigPath, err)
	}

	configHash := fmt.Sprintf("%x", sha256.Sum256(configContent))
	logger.Debugf("Calculated checksum for '%s': %s", nginxConfigPath, configHash)

	data := NginxStaticPodTemplateData{
		Image:      fullImageName,
		ConfigHash: configHash,
		ListenPort: cluster.Spec.ControlPlaneEndpoint.Port,
	}

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

func (s *GenerateNginxStaticPodStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteManifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-nginx-lb.yaml")
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteManifestPath)
	if err != nil || !exists {
		return false, err
	}

	expectedContent, err := s.renderContent(ctx)
	if err != nil {
		return false, nil
	}
	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remoteManifestPath)
	if err != nil {
		return false, nil
	}

	if bytes.Equal(bytes.TrimSpace(remoteContent), bytes.TrimSpace(expectedContent)) {
		logger.Info("NGINX static pod manifest is up-to-date.")
		return true, nil
	}

	logger.Info("NGINX static pod manifest differs. Step needs to run.")
	return false, nil
}

func (s *GenerateNginxStaticPodStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render content")
		return result, err
	}

	staticPodDir := common.KubernetesManifestsDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, staticPodDir, "0755", true); err != nil {
		result.MarkFailed(err, "failed to create static pod directory")
		return result, err
	}

	remoteManifestPath := filepath.Join(staticPodDir, "kube-nginx-lb.yaml")
	logger.Infof("Uploading NGINX static pod manifest to %s:%s", ctx.GetHost().GetName(), remoteManifestPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteManifestPath, "0644", true); err != nil {
		result.MarkFailed(err, "failed to upload NGINX static pod manifest")
		return result, err
	}

	logger.Info("NGINX static pod manifest generated and uploaded successfully.")
	result.MarkCompleted("NGINX static pod manifest generated and uploaded successfully")
	return result, nil
}

func (s *GenerateNginxStaticPodStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteManifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-nginx-lb.yaml")
	logger.Warnf("Rolling back by removing static pod manifest: %s", remoteManifestPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteManifestPath, true, true); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteManifestPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateNginxStaticPodStep)(nil)
