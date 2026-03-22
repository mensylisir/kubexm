package kubevip

import (
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

type GenerateKubeVipManifestStep struct {
	step.Base
}

type GenerateKubeVipManifestStepBuilder struct {
	step.Builder[GenerateKubeVipManifestStepBuilder, *GenerateKubeVipManifestStep]
}

func NewGenerateKubeVipManifestStepBuilder(ctx runtime.ExecutionContext, instanceName string) *GenerateKubeVipManifestStepBuilder {
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

	if haCfg == nil || haCfg.Enabled == nil || !*haCfg.Enabled {
		return nil, fmt.Errorf("kube-vip is not enabled for this cluster")
	}

	externalEnabled := haCfg.External != nil && haCfg.External.Enabled != nil && *haCfg.External.Enabled &&
		haCfg.External.Type == string(common.ExternalLBTypeKubeVIP)
	internalEnabled := haCfg.Internal != nil && haCfg.Internal.Enabled != nil && *haCfg.Internal.Enabled &&
		haCfg.Internal.Type == string(common.InternalLBTypeKubeVIP)

	if !externalEnabled && !internalEnabled {
		return nil, fmt.Errorf("kube-vip is not configured as the load balancer for this cluster")
	}

	var kubeVipImageRef string
	if haCfg.External != nil && haCfg.External.KubeVIP != nil && haCfg.External.KubeVIP.Image != nil && *haCfg.External.KubeVIP.Image != "" {
		kubeVipImageRef = *haCfg.External.KubeVIP.Image
	} else {
		imageProvider := images.NewImageProvider(ctx)
		kubeVipImage := imageProvider.GetImage("kube-vip")
		if kubeVipImage == nil {
			return nil, fmt.Errorf("kube-vip image not found in BOM for the current cluster version")
		}
		kubeVipImageRef = kubeVipImage.FullName()
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return nil, fmt.Errorf("failed to gather facts from host %s to determine network interface: %w", ctx.GetHost().GetName(), err)
	}

	var iface *string
	var vipOverride *string
	if haCfg.External != nil && haCfg.External.KubeVIP != nil {
		iface = haCfg.External.KubeVIP.Interface
		vipOverride = haCfg.External.KubeVIP.VIP
	}
	if iface == nil {
		iface = &facts.DefaultInterface
	}
	if iface == nil || *iface == "" {
		return nil, fmt.Errorf("network interface for kube-vip is not specified in config and could not be determined from host facts")
	}

	vip := clusterCfg.Spec.ControlPlaneEndpoint.Address
	if vipOverride != nil && *vipOverride != "" {
		vip = *vipOverride
	}

	data := KubeVipTemplateData{
		VIP:              vip,
		Interface:        *iface,
		KubeVipImage:     kubeVipImageRef,
		LoadBalancerPort: clusterCfg.Spec.ControlPlaneEndpoint.Port,
	}

	templatePath := "loadbalancer/kube-vip/kube-vip-yaml.tmpl"
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

func (s *GenerateKubeVipManifestStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	renderedContentBytes, err := s.renderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render kube-vip manifest content during Run phase")
		return result, err
	}
	renderedContent := string(renderedContentBytes)

	staticPodDir := common.KubernetesManifestsDir
	if err := runner.Mkdirp(ctx.GoContext(), conn, staticPodDir, "0755", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to create static pod directory")
		return result, err
	}

	remoteManifestPath := filepath.Join(staticPodDir, "kube-vip.yaml")
	logger.Infof("Writing kube-vip static pod manifest to %s:%s", ctx.GetHost().GetName(), remoteManifestPath)

	if err := helpers.WriteContentToRemote(ctx, conn, renderedContent, remoteManifestPath, "0644", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to write kube-vip static pod manifest")
		return result, err
	}

	logger.Info("kube-vip static pod manifest generated and placed successfully.")
	result.MarkCompleted("kube-vip static pod manifest generated and placed successfully")
	return result, nil
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
