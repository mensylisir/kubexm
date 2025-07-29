package flannel

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

type GenerateFlannelHelmArtifactsStep struct {
	step.Base
	Chart                *helm.HelmChart
	RemoteValuesPath     string
	RemoteChartPath      string
	LocalPulledChartPath string

	ImageFlannelRepo   string
	ImageFlannelTag    string
	ImageCNIPluginRepo string
	ImageCNIPluginTag  string
	PodCIDR            string
	BackendType        string
	BackendVXLAN       *v1alpha1.FlannelVXLANConfig
	BackendIPsec       *v1alpha1.FlannelIPsecConfig
}

type GenerateFlannelHelmArtifactsStepBuilder struct {
	step.Builder[GenerateFlannelHelmArtifactsStepBuilder, *GenerateFlannelHelmArtifactsStep]
}

func NewGenerateFlannelHelmArtifactsStepBuilder(ctx runtime.Context, instanceName string) *GenerateFlannelHelmArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	flannelChart := helmProvider.GetChart(string(common.CNITypeFlannel))

	if flannelChart == nil {
		return nil
	}

	imageProvider := images.NewImageProvider(&ctx)
	flannelImage := imageProvider.GetImage("flannel")
	cniPluginImage := imageProvider.GetImage("flannel-cni-plugin")

	if flannelImage == nil || cniPluginImage == nil {
		return nil
	}

	s := &GenerateFlannelHelmArtifactsStep{
		Chart: flannelChart,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Flannel Helm artifacts (Chart: %s, Version: %s)", s.Base.Meta.Name, flannelChart.ChartName(), flannelChart.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	localChartDir := filepath.Dir(flannelChart.LocalPath(common.DefaultKubexmTmpDir))
	s.LocalPulledChartPath = localChartDir

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, flannelChart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, "flannel-values.yaml")

	s.ImageFlannelRepo = flannelImage.FullNameWithoutTag()
	s.ImageFlannelTag = flannelImage.Tag()
	s.ImageCNIPluginRepo = cniPluginImage.FullNameWithoutTag()
	s.ImageCNIPluginTag = cniPluginImage.Tag()

	clusterCfg := ctx.GetClusterConfig()
	s.PodCIDR = clusterCfg.Spec.Network.KubePodsCIDR

	userFlannelCfg := clusterCfg.Spec.Network.Flannel
	if userFlannelCfg != nil && userFlannelCfg.Backend != nil {
		backendCfg := userFlannelCfg.Backend
		s.BackendType = backendCfg.Type
		s.BackendVXLAN = backendCfg.VXLAN
		s.BackendIPsec = backendCfg.IPsec
	}

	if s.BackendType == "" {
		s.BackendType = "vxlan"
	}

	b := new(GenerateFlannelHelmArtifactsStepBuilder).Init(s)
	return b
}

func (s *GenerateFlannelHelmArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateFlannelHelmArtifactsStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("failed to pull helm chart: %w, output: %s", err, string(output))
	}

	actualLocalChartPath := s.Chart.LocalPath(common.DefaultKubexmTmpDir)
	if _, err := os.Stat(actualLocalChartPath); os.IsNotExist(err) {
		return fmt.Errorf("expected helm chart .tgz file not found at %s after pull", actualLocalChartPath)
	}

	s.RemoteChartPath = filepath.Join(filepath.Dir(s.RemoteValuesPath), filepath.Base(actualLocalChartPath))

	logger.Info("Rendering flannel-values.yaml from template...")
	valuesTemplateContent, err := templates.Get("cni/flannel/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("flannelValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render values.yaml.tmpl: %w", err)
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
		return fmt.Errorf("failed to create remote dir: %w", err)
	}

	logger.Infof("Uploading chart to remote path: %s", s.RemoteChartPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, chartContent, s.RemoteChartPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload helm chart: %w", err)
	}

	logger.Infof("Uploading rendered values.yaml to remote path: %s", s.RemoteValuesPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, valuesContent, s.RemoteValuesPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload values.yaml: %w", err)
	}

	logger.Info("Flannel Helm artifacts, including rendered values.yaml, uploaded successfully.")
	return nil
}

func (s *GenerateFlannelHelmArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *GenerateFlannelHelmArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteDir := filepath.Dir(s.RemoteValuesPath)
	logger.Warnf("Rolling back by removing remote Helm artifacts directory: %s", remoteDir)
	if err := runner.Remove(ctx.GoContext(), conn, remoteDir, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove remote artifacts directory during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*GenerateFlannelHelmArtifactsStep)(nil)
