package cilium

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	// 引入必要的包
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateCiliumHelmArtifactsStep struct {
	step.Base
	Chart                *helm.HelmChart
	RemoteValuesPath     string
	RemoteChartPath      string
	LocalPulledChartPath string

	ImageRepository         string
	ImageTag                string
	OperatorImageRepository string
	OperatorImageTag        string
	Tunnel                  string
	IpamMode                string
	KubeProxyReplacement    string
	BpfMasquerade           bool
	HubbleEnabled           bool
	HubbleUiEnabled         bool
	HubbleRelayEnabled      bool
	IdentityAllocationMode  string
	EncryptionEnabled       bool
	BandwidthManagerEnabled bool
	AutoDirectNodeRoutes    bool
	OperatorReplicas        int
}

type GenerateCiliumHelmArtifactsStepBuilder struct {
	step.Builder[GenerateCiliumHelmArtifactsStepBuilder, *GenerateCiliumHelmArtifactsStep]
}

func NewGenerateCiliumHelmArtifactsStepBuilder(ctx runtime.Context, instanceName string) *GenerateCiliumHelmArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	ciliumChart := helmProvider.GetChart(string(common.CNITypeCilium))

	if ciliumChart == nil {
		return nil
	}

	imageProvider := images.NewImageProvider(&ctx)
	ciliumImage := imageProvider.GetImage("cilium")
	operatorImage := imageProvider.GetImage("cilium-operator-generic")

	if ciliumImage == nil || operatorImage == nil {
		return nil
	}

	s := &GenerateCiliumHelmArtifactsStep{
		Chart: ciliumChart,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Cilium Helm artifacts (Chart: %s, Version: %s)", s.Base.Meta.Name, ciliumChart.ChartName(), ciliumChart.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	localChartDir := filepath.Dir(ciliumChart.LocalPath(common.DefaultKubexmTmpDir))
	s.LocalPulledChartPath = localChartDir

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, ciliumChart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, "cilium-values.yaml")

	s.ImageRepository = ciliumImage.FullNameWithoutTag()
	s.ImageTag = ciliumImage.Tag()
	s.OperatorImageRepository = operatorImage.FullNameWithoutTag()
	s.OperatorImageTag = operatorImage.Tag()

	s.Tunnel = "vxlan"
	s.IpamMode = "kubernetes"
	s.KubeProxyReplacement = "probe"
	s.BpfMasquerade = true
	s.HubbleEnabled = true
	s.HubbleUiEnabled = true
	s.IdentityAllocationMode = "crd"
	s.EncryptionEnabled = false
	s.BandwidthManagerEnabled = false
	s.OperatorReplicas = 1
	s.HubbleRelayEnabled = true
	s.AutoDirectNodeRoutes = false

	clusterCfg := ctx.GetClusterConfig()
	userCiliumCfg := clusterCfg.Spec.Network.Cilium
	if userCiliumCfg != nil {
		if userCiliumCfg.Network != nil {
			if userCiliumCfg.Network.TunnelingMode != "" {
				s.Tunnel = userCiliumCfg.Network.TunnelingMode
			}
			if userCiliumCfg.Network.IPAMMode != "" {
				s.IpamMode = userCiliumCfg.Network.IPAMMode
			}
		}
		if userCiliumCfg.KubeProxy != nil {
			if userCiliumCfg.KubeProxy.ReplacementMode != "" {
				s.KubeProxyReplacement = userCiliumCfg.KubeProxy.ReplacementMode
			}
			if userCiliumCfg.KubeProxy.EnableBPFMasquerade != nil {
				s.BpfMasquerade = *userCiliumCfg.KubeProxy.EnableBPFMasquerade
			}
		}
		if userCiliumCfg.Hubble != nil {
			s.HubbleEnabled = userCiliumCfg.Hubble.Enable
			s.HubbleUiEnabled = userCiliumCfg.Hubble.EnableUI
		}
		if userCiliumCfg.Security != nil {
			if userCiliumCfg.Security.IdentityAllocationMode != "" {
				s.IdentityAllocationMode = userCiliumCfg.Security.IdentityAllocationMode
			}
			if userCiliumCfg.Security.EnableEncryption != nil {
				s.EncryptionEnabled = *userCiliumCfg.Security.EnableEncryption
			}
		}
		if userCiliumCfg.Performance != nil {
			if userCiliumCfg.Performance.EnableBandwidthManager != nil {
				s.BandwidthManagerEnabled = *userCiliumCfg.Performance.EnableBandwidthManager
			}
		}
	}

	b := new(GenerateCiliumHelmArtifactsStepBuilder).Init(s)
	return b
}

func (s *GenerateCiliumHelmArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCiliumHelmArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("helm executable not found in PATH on the machine running this tool")
	}

	if err := os.RemoveAll(s.LocalPulledChartPath); err != nil {
		logger.Warnf("Failed to clean up local temp directory %s, continuing...", s.LocalPulledChartPath)
	}
	if err := os.MkdirAll(s.LocalPulledChartPath, 0755); err != nil {
		return fmt.Errorf("failed to create local temp dir: %w", err)
	}

	repoName := s.Chart.RepoName()
	repoURL := s.Chart.RepoURL()

	logger.Infof("Adding Helm repo '%s' from '%s'", repoName, repoURL)
	repoAddCmd := exec.Command(helmPath, "repo", "add", repoName, repoURL)
	if output, err := repoAddCmd.CombinedOutput(); err != nil && !strings.Contains(string(output), "already exists") {
		return fmt.Errorf("failed to add helm repo: %w, output: %s", err, string(output))
	}

	logger.Infof("Updating Helm repo '%s'", repoName)
	if err := exec.Command(helmPath, "repo", "update", repoName).Run(); err != nil {
		return fmt.Errorf("failed to update helm repo: %w", err)
	}

	chartFullName := s.Chart.FullName()
	pullArgs := []string{"pull", chartFullName, "--destination", s.LocalPulledChartPath, "--version", s.Chart.Version}

	logger.Infof("Pulling Helm chart with command: helm %s", strings.Join(pullArgs, " "))
	pullCmd := exec.Command(helmPath, pullArgs...)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull helm chart: %w, output: %s", err, string(output))
	}

	actualLocalChartPath := s.Chart.LocalPath(common.DefaultKubexmTmpDir)
	if _, err := os.Stat(actualLocalChartPath); os.IsNotExist(err) {
		return fmt.Errorf("expected helm chart .tgz file not found at %s after pull", actualLocalChartPath)
	}

	s.RemoteChartPath = filepath.Join(filepath.Dir(s.RemoteValuesPath), filepath.Base(actualLocalChartPath))

	valuesTemplateContent, err := templates.Get("cni/cilium/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded values.yaml.tmpl for cilium: %w", err)
	}
	tmpl, err := template.New("ciliumValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse cilium values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render cilium values.yaml.tmpl: %w", err)
	}
	valuesContent := valuesBuffer.Bytes()

	chartContent, err := os.ReadFile(actualLocalChartPath)
	if err != nil {
		return fmt.Errorf("failed to read pulled chart file: %w", err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	if err := runner.Mkdirp(ctx.GoContext(), conn, filepath.Dir(s.RemoteChartPath), "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote dir for cilium artifacts: %w", err)
	}
	logger.Infof("Uploading Cilium chart to %s", s.RemoteChartPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, chartContent, s.RemoteChartPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload cilium helm chart: %w", err)
	}
	logger.Infof("Uploading Cilium values to %s", s.RemoteValuesPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, valuesContent, s.RemoteValuesPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload cilium values.yaml: %w", err)
	}

	logger.Info("Cilium Helm artifacts prepared and uploaded successfully.")
	return nil
}

func (s *GenerateCiliumHelmArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *GenerateCiliumHelmArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	remoteDir := filepath.Dir(s.RemoteValuesPath)
	logger.Warnf("Rolling back by removing remote Cilium Helm artifacts directory: %s", remoteDir)
	if err := runner.Remove(ctx.GoContext(), conn, remoteDir, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove remote Cilium artifacts directory during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*GenerateCiliumHelmArtifactsStep)(nil)
