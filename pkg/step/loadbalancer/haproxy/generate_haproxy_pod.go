package haproxy

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateHAProxyStaticPodStep struct {
	step.Base
}
type GenerateHAProxyStaticPodStepBuilder struct {
	step.Builder[GenerateHAProxyStaticPodStepBuilder, *GenerateHAProxyStaticPodStep]
}

func NewGenerateHAProxyStaticPodStepBuilder(ctx runtime.Context, instanceName string) *GenerateHAProxyStaticPodStepBuilder {
	s := &GenerateHAProxyStaticPodStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate HAProxy Static Pod manifest", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateHAProxyStaticPodStepBuilder).Init(s)
	return b
}

func (s *GenerateHAProxyStaticPodStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type HAProxyStaticPodTemplateData struct {
	Image      string
	ConfigHash string
}

func (s *GenerateHAProxyStaticPodStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	logger := ctx.GetLogger().With("host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil, err
	}

	imageProvider := images.NewImageProvider(ctx)
	haproxyImage := imageProvider.GetImage("haproxy")
	fullImageName := haproxyImage.FullName()
	logger.Debugf("Using HAProxy image: %s", fullImageName)

	haproxyConfigPath := common.HAProxyDefaultConfigFileTarget
	configContent, err := runner.ReadFile(ctx.GoContext(), conn, haproxyConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read haproxy config file at '%s' to calculate checksum: %w", haproxyConfigPath, err)
	}

	configHash := fmt.Sprintf("%x", sha256.Sum256(configContent))
	logger.Debugf("Calculated checksum for '%s': %s", haproxyConfigPath, configHash)

	data := HAProxyStaticPodTemplateData{
		Image:      fullImageName,
		ConfigHash: configHash,
	}

	templateContent, err := templates.Get("haproxy/haproxy-static-pod.yaml.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get haproxy static pod template: %w", err)
	}
	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render haproxy static pod template: %w", err)
	}

	return []byte(renderedConfig), nil
}

func (s *GenerateHAProxyStaticPodStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteManifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-haproxy.yaml")
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteManifestPath)
	if err != nil || !exists {
		return false, err
	}

	expectedContent, err := s.renderContent(ctx)
	if err != nil {
		return false, err
	}
	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remoteManifestPath)
	if err != nil {
		return false, err
	}

	if bytes.Equal(bytes.TrimSpace(remoteContent), bytes.TrimSpace(expectedContent)) {
		logger.Info("HAProxy static pod manifest is up-to-date.")
		return true, nil
	}

	logger.Info("HAProxy static pod manifest differs. Step needs to run.")
	return false, nil
}

func (s *GenerateHAProxyStaticPodStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
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
		return fmt.Errorf("failed to create static pod directory '%s' with sudo: %w", staticPodDir, err)
	}

	remoteManifestPath := filepath.Join(staticPodDir, "kube-haproxy.yaml")
	logger.Infof("Uploading HAProxy static pod manifest to %s:%s", ctx.GetHost().GetName(), remoteManifestPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteManifestPath, "0644", true); err != nil {
		return fmt.Errorf("failed to upload HAProxy static pod manifest with sudo: %w", err)
	}

	logger.Info("HAProxy static pod manifest generated and uploaded successfully.")
	return nil
}

func (s *GenerateHAProxyStaticPodStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteManifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-haproxy.yaml")
	logger.Warnf("Rolling back by removing static pod manifest: %s", remoteManifestPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteManifestPath, true, true); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteManifestPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateHAProxyStaticPodStep)(nil)
