package cilium

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/util"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateCiliumHelmArtifactsStep struct {
	step.Base
	RemoteValuesPath     string
	ChartSourceDecision  *helpers.ChartSourceDecision
	LocalPulledChartPath string
	RemoteChartPath      string

	ImageRepository         string
	ImageTag                string
	Tunnel                  string
	IpamMode                string
	KubeProxyReplacement    string
	BpfMasquerade           bool
	HubbleEnabled           bool
	HubbleUiEnabled         bool
	IdentityAllocationMode  string
	EncryptionEnabled       bool
	BandwidthManagerEnabled bool
	OperatorReplicas        int
}

type GenerateCiliumHelmArtifactsStepBuilder struct {
	step.Builder[GenerateCiliumHelmArtifactsStepBuilder, *GenerateCiliumHelmArtifactsStep]
}

func NewGenerateCiliumHelmArtifactsStepBuilder(ctx runtime.Context, instanceName string) *GenerateCiliumHelmArtifactsStepBuilder {
	s := &GenerateCiliumHelmArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Cilium Helm artifacts", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	s.ChartSourceDecision = helpers.DecideCiliumChartSource(ctx)

	s.LocalPulledChartPath = filepath.Join(common.DefaultKubexmTmpDir, "cilium")
	remoteDir := filepath.Join(common.DefaultUploadTmpDir, "cilium")
	s.RemoteValuesPath = filepath.Join(remoteDir, "cilium-values.yaml")

	clusterCfg := ctx.GetClusterConfig()
	ciliumImage := util.GetImage(ctx, "cilium")
	s.ImageRepository = ciliumImage.ImageRepo()
	s.ImageTag = ciliumImage.Tag

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
	s.ImageRepository = clusterCfg.Spec.Registry.MirroringAndRewriting.PrivateRegistry
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
		return fmt.Errorf("helm executable not found in PATH")
	}

	if err := os.RemoveAll(s.LocalPulledChartPath); err != nil {
		logger.Warnf("Failed to clean up local temp directory %s, continuing...", s.LocalPulledChartPath)
	}
	if err := os.MkdirAll(s.LocalPulledChartPath, 0755); err != nil {
		return fmt.Errorf("failed to create local temp dir: %w", err)
	}

	repoName, repoURL := s.ChartSourceDecision.RepoName, s.ChartSourceDecision.RepoURL
	repoAddCmd := exec.Command(helmPath, "repo", "add", repoName, repoURL)
	if output, err := repoAddCmd.CombinedOutput(); err != nil && !strings.Contains(string(output), "already exists") {
		return fmt.Errorf("failed to add helm repo '%s': %w, output: %s", repoName, err, string(output))
	}
	if err := exec.Command(helmPath, "repo", "update", repoName).Run(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w", repoName, err)
	}

	chartFullName := fmt.Sprintf("%s/%s", repoName, s.ChartSourceDecision.ChartName)
	pullArgs := []string{"pull", chartFullName, "--destination", s.LocalPulledChartPath}
	if s.ChartSourceDecision.Version != "" {
		pullArgs = append(pullArgs, "--version", s.ChartSourceDecision.Version)
	}

	pullCmd := exec.Command(helmPath, pullArgs...)
	logger.Infof("Pulling Helm chart with command: helm %s", strings.Join(pullArgs, " "))
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull helm chart: %w, output: %s", err, string(output))
	}

	pulledFiles, _ := filepath.Glob(filepath.Join(s.LocalPulledChartPath, "*.tgz"))
	if len(pulledFiles) == 0 {
		return fmt.Errorf("helm chart .tgz file not found in %s", s.LocalPulledChartPath)
	}

	actualLocalChartPath := pulledFiles[0]
	s.RemoteChartPath = filepath.Join(filepath.Dir(s.RemoteValuesPath), filepath.Base(actualLocalChartPath))

	valuesTemplateContent, err := templates.Get("cni/cilium/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get values.yaml.tmpl for cilium: %w", err)
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
