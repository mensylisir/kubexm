package kubeovn

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
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateKubeOvnHelmArtifactsStep struct {
	step.Base
	Chart                *helm.HelmChart
	RemoteValuesPath     string
	RemoteChartPath      string
	LocalPulledChartPath string

	ImageRegistry    string
	ImageTag         string
	PodCIDR          string
	ServiceCIDR      string
	Networking       *v1alpha1.KubeOvnNetworking
	Controller       *v1alpha1.KubeOvnControllerConfig
	AdvancedFeatures *v1alpha1.KubeOvnAdvancedFeatures
}

type GenerateKubeOvnHelmArtifactsStepBuilder struct {
	step.Builder[GenerateKubeOvnHelmArtifactsStepBuilder, *GenerateKubeOvnHelmArtifactsStep]
}

func NewGenerateKubeOvnHelmArtifactsStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeOvnHelmArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	kubeovnChart := helmProvider.GetChart(string(common.CNITypeKubeOvn))

	if kubeovnChart == nil {
		return nil
	}

	imageProvider := images.NewImageProvider(&ctx)
	kubeovnImage := imageProvider.GetImage("kubeovn")

	if kubeovnImage == nil {
		return nil
	}

	s := &GenerateKubeOvnHelmArtifactsStep{
		Chart: kubeovnChart,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Kube-OVN Helm artifacts (Chart: %s, Version: %s)", s.Base.Meta.Name, kubeovnChart.ChartName(), kubeovnChart.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	localChartDir := filepath.Dir(kubeovnChart.LocalPath(common.DefaultKubexmTmpDir))
	s.LocalPulledChartPath = localChartDir

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, kubeovnChart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, "kubeovn-values.yaml")

	s.ImageRegistry = kubeovnImage.FullNameWithoutTag()
	s.ImageTag = kubeovnImage.Tag()

	clusterCfg := ctx.GetClusterConfig()
	s.ServiceCIDR = clusterCfg.Spec.Network.KubeServiceCIDR
	s.PodCIDR = clusterCfg.Spec.Network.KubePodsCIDR

	userKubeOvnCfg := clusterCfg.Spec.Network.KubeOvn
	if userKubeOvnCfg != nil {
		s.Networking = userKubeOvnCfg.Networking
		s.Controller = userKubeOvnCfg.Controller
		s.AdvancedFeatures = userKubeOvnCfg.AdvancedFeatures

		if userKubeOvnCfg.Controller != nil && userKubeOvnCfg.Controller.PodDefaultSubnetCIDR != "" {
			s.PodCIDR = userKubeOvnCfg.Controller.PodDefaultSubnetCIDR
		}
	}

	if s.Networking == nil {
		s.Networking = &v1alpha1.KubeOvnNetworking{}
	}
	if s.Networking.TunnelType == "" {
		s.Networking.TunnelType = "geneve"
	}
	if s.Controller == nil {
		s.Controller = &v1alpha1.KubeOvnControllerConfig{
			JoinCIDR:       "100.64.0.0/16",
			NodeSwitchCIDR: "192.168.0.0/24",
		}
	}

	b := new(GenerateKubeOvnHelmArtifactsStepBuilder).Init(s)
	return b
}

func (s *GenerateKubeOvnHelmArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateKubeOvnHelmArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("helm executable not found in PATH on the machine running this tool")
	}

	if err := os.RemoveAll(s.LocalPulledChartPath); err != nil {
		logger.Warnf("Failed to clean up local temp directory %s, continuing...", s.LocalPulledChartPath)
	}
	if err := os.MkdirAll(s.LocalPulledChartPath, 0755); err != nil {
		return fmt.Errorf("failed to create local temp dir %s: %w", s.LocalPulledChartPath, err)
	}

	repoName := s.Chart.RepoName()
	repoURL := s.Chart.RepoURL()

	repoAddCmd := exec.Command(helmPath, "repo", "add", repoName, repoURL)
	if output, err := repoAddCmd.CombinedOutput(); err != nil && !strings.Contains(string(output), "already exists") {
		return fmt.Errorf("failed to add helm repo '%s' from '%s': %w, output: %s", repoName, repoURL, err, string(output))
	}

	repoUpdateCmd := exec.Command(helmPath, "repo", "update", repoName)
	if err := repoUpdateCmd.Run(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w", repoName, err)
	}

	chartFullName := s.Chart.FullName()
	pullArgs := []string{"pull", chartFullName, "--destination", s.LocalPulledChartPath, "--version", s.Chart.Version}

	logger.Infof("Pulling Helm chart with command: helm %s", strings.Join(pullArgs, " "))
	pullCmd := exec.Command(helmPath, pullArgs...)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Kube-OVN helm chart: %w, output: %s", err, string(output))
	}

	actualLocalChartPath := s.Chart.LocalPath(common.DefaultKubexmTmpDir)
	if _, err := os.Stat(actualLocalChartPath); os.IsNotExist(err) {
		return fmt.Errorf("expected Kube-OVN helm chart .tgz file not found at %s after pull", actualLocalChartPath)
	}

	s.RemoteChartPath = filepath.Join(filepath.Dir(s.RemoteValuesPath), filepath.Base(actualLocalChartPath))

	logger.Info("Rendering kubeovn-values.yaml from template...")
	valuesTemplateContent, err := templates.Get("cni/kubeovn/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded values.yaml.tmpl for kube-ovn: %w", err)
	}

	tmpl, err := template.New("kubeovnValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse kubeovn-values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render kubeovn-values.yaml.tmpl: %w", err)
	}
	valuesContent := valuesBuffer.Bytes()

	chartContent, err := os.ReadFile(actualLocalChartPath)
	if err != nil {
		return fmt.Errorf("failed to read pulled Kube-OVN chart file: %w", err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, filepath.Dir(s.RemoteChartPath), "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote dir: %w", err)
	}

	logger.Infof("Uploading Kube-OVN chart to remote path: %s", s.RemoteChartPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, chartContent, s.RemoteChartPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload Kube-OVN helm chart: %w", err)
	}

	logger.Infof("Uploading rendered values.yaml to remote path: %s", s.RemoteValuesPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, valuesContent, s.RemoteValuesPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload Kube-OVN values.yaml: %w", err)
	}

	logger.Info("Kube-OVN Helm artifacts, including rendered values.yaml, uploaded successfully.")
	return nil
}

func (s *GenerateKubeOvnHelmArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *GenerateKubeOvnHelmArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteDir := filepath.Dir(s.RemoteValuesPath)
	logger.Warnf("Rolling back by removing remote Kube-OVN Helm artifacts directory: %s", remoteDir)
	if err := runner.Remove(ctx.GoContext(), conn, remoteDir, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove remote Kube-OVN artifacts directory during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*GenerateKubeOvnHelmArtifactsStep)(nil)
