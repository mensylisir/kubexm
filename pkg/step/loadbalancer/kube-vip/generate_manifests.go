package kubevip

import (
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

type GenerateKubeVipManifestStep struct {
	step.Base
}

type GenerateKubeVipManifestStepBuilder struct {
	step.Builder[GenerateKubeVipManifestStepBuilder, *GenerateKubeVipManifestStep]
}

func NewGenerateKubeVipManifestStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeVipManifestStepBuilder {
	s := &GenerateKubeVipManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate kube-vip static pod manifest", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateKubeVipManifestStepBuilder).Init(s)
	return b
}

func (s *GenerateKubeVipManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type KubeVipTemplateData struct {
	VIP              string
	Interface        string
	KubeVipImage     string
	LoadBalancerPort int
}

func (s *GenerateKubeVipManifestStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	clusterCfg := ctx.GetClusterConfig()
	haCfg := clusterCfg.Spec.ControlPlaneEndpoint.HighAvailability

	if haCfg == nil || haCfg.Enabled == nil || !*haCfg.Enabled || haCfg.External.Type != string(common.ExternalLBTypeKubeVIP) {
		return nil, fmt.Errorf("kube-vip is not configured as the external load balancer for this host")
	}

	imageProvider := images.NewImageProvider(ctx)
	kubeVipImage := imageProvider.GetImage("kube-vip")
	if kubeVipImage == nil {
		return nil, fmt.Errorf("kube-vip image not found in BOM for the current cluster version")
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return nil, fmt.Errorf("failed to gather facts from host %s to determine network interface: %w", ctx.GetHost().GetName(), err)
	}

	iface := haCfg.External.KubeVIP.Interface
	if iface == nil {
		iface = &facts.DefaultInterface
	}
	if iface == nil {
		return nil, fmt.Errorf("network interface for kube-vip is not specified in config and could not be determined from host facts")
	}

	data := KubeVipTemplateData{
		VIP:              clusterCfg.Spec.ControlPlaneEndpoint.Address,
		Interface:        *iface,
		KubeVipImage:     kubeVipImage.FullName(),
		LoadBalancerPort: clusterCfg.Spec.ControlPlaneEndpoint.Port,
	}

	templatePath := "loadbalancer/kube-vip/kube-vip.yaml.tmpl"
	templateContent, err := templates.Get(templatePath)
	if err != nil {
		return nil, err
	}

	renderedContent, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template '%s': %w", templatePath, err)
	}

	return []byte(renderedContent), nil
}

func (s *GenerateKubeVipManifestStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	remoteManifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-vip.yaml")

	expectedContentBytes, err := s.renderContent(ctx)
	if err != nil {
		logger.Warnf("Could not render expected manifest content, skipping precheck and assuming step is done: %v", err)
		return true, nil
	}
	expectedContent := string(expectedContentBytes)

	isDone, err = helpers.CheckRemoteFileIntegrity(ctx, remoteManifestPath, expectedContent, s.Sudo)
	if err != nil {
		return false, err
	}

	if isDone {
		logger.Info("kube-vip static pod manifest is already up-to-date.")
	} else {
		logger.Info("kube-vip static pod manifest is missing or outdated. Step needs to run.")
	}
	return isDone, nil
}

func (s *GenerateKubeVipManifestStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	renderedContentBytes, err := s.renderContent(ctx)
	if err != nil {
		return fmt.Errorf("failed to render kube-vip manifest content during Run phase: %w", err)
	}
	renderedContent := string(renderedContentBytes)

	staticPodDir := common.KubernetesManifestsDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, staticPodDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create static pod directory '%s': %w", staticPodDir, err)
	}

	remoteManifestPath := filepath.Join(staticPodDir, "kube-vip.yaml")
	logger.Infof("Writing kube-vip static pod manifest to %s:%s", ctx.GetHost().GetName(), remoteManifestPath)

	if err := helpers.WriteContentToRemote(ctx, conn, renderedContent, remoteManifestPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write kube-vip static pod manifest: %w", err)
	}

	logger.Info("kube-vip static pod manifest generated and placed successfully.")
	return nil
}

func (s *GenerateKubeVipManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rolling back generation of kube-vip manifest by removing the manifest file.")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback, cannot remove manifest: %v", err)
		return nil
	}

	remoteManifestPath := filepath.Join(common.KubernetesManifestsDir, "kube-vip.yaml")
	if err := runner.Remove(ctx.GoContext(), conn, remoteManifestPath, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove kube-vip manifest during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*GenerateKubeVipManifestStep)(nil)
