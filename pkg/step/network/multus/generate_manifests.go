package multus

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

type GenerateMultusHelmArtifactsStep struct {
	step.Base
	Chart                *helm.HelmChart
	RemoteValuesPath     string
	RemoteChartPath      string
	LocalPulledChartPath string

	ImageRepository string
	ImageTag        string
}

type GenerateMultusHelmArtifactsStepBuilder struct {
	step.Builder[GenerateMultusHelmArtifactsStepBuilder, *GenerateMultusHelmArtifactsStep]
}

func NewGenerateMultusHelmArtifactsStepBuilder(ctx runtime.Context, instanceName string) *GenerateMultusHelmArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	multusChart := helmProvider.GetChart(string(common.CNITypeMultus))

	if multusChart == nil {
		return nil
	}

	imageProvider := images.NewImageProvider(&ctx)
	multusImage := imageProvider.GetImage("multus")

	if multusImage == nil {
		return nil
	}

	s := &GenerateMultusHelmArtifactsStep{
		Chart: multusChart,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Multus CNI Helm artifacts (Chart: %s, Version: %s)", s.Base.Meta.Name, multusChart.ChartName(), multusChart.Version)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	localChartDir := filepath.Dir(multusChart.LocalPath(common.DefaultKubexmTmpDir))
	s.LocalPulledChartPath = localChartDir

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, multusChart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, "multus-values.yaml")

	s.ImageRepository = multusImage.FullNameWithoutTag()
	s.ImageTag = multusImage.Tag()

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Network.Multus != nil &&
		clusterCfg.Spec.Network.Multus.Installation != nil &&
		clusterCfg.Spec.Network.Multus.Installation.Image != "" {

		fullImage := clusterCfg.Spec.Network.Multus.Installation.Image
		if lastColon := strings.LastIndex(fullImage, ":"); lastColon != -1 {
			if !strings.Contains(fullImage[lastColon:], "/") {
				s.ImageRepository = fullImage[:lastColon]
				s.ImageTag = fullImage[lastColon+1:]
			}
		}
	}

	b := new(GenerateMultusHelmArtifactsStepBuilder).Init(s)
	return b
}

func (s *GenerateMultusHelmArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateMultusHelmArtifactsStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("failed to pull Multus CNI helm chart: %w, output: %s", err, string(output))
	}

	actualLocalChartPath := s.Chart.LocalPath(common.DefaultKubexmTmpDir)
	if _, err := os.Stat(actualLocalChartPath); os.IsNotExist(err) {
		return fmt.Errorf("expected Multus CNI helm chart .tgz file not found at %s after pull", actualLocalChartPath)
	}

	s.RemoteChartPath = filepath.Join(filepath.Dir(s.RemoteValuesPath), filepath.Base(actualLocalChartPath))

	logger.Info("Rendering multus-values.yaml from template...")
	valuesTemplateContent, err := templates.Get("cni/multus/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded values.yaml.tmpl for multus: %w", err)
	}

	tmpl, err := template.New("multusValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse multus-values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render multus-values.yaml.tmpl: %w", err)
	}
	valuesContent := valuesBuffer.Bytes()

	chartContent, err := os.ReadFile(actualLocalChartPath)
	if err != nil {
		return fmt.Errorf("failed to read pulled Multus CNI chart file: %w", err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, filepath.Dir(s.RemoteChartPath), "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote dir: %w", err)
	}

	logger.Infof("Uploading Multus CNI chart to remote path: %s", s.RemoteChartPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, chartContent, s.RemoteChartPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload Multus CNI helm chart: %w", err)
	}

	logger.Infof("Uploading rendered values.yaml to remote path: %s", s.RemoteValuesPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, valuesContent, s.RemoteValuesPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload Multus CNI values.yaml: %w", err)
	}

	logger.Info("Multus CNI Helm artifacts, including rendered values.yaml, uploaded successfully.")
	return nil
}

func (s *GenerateMultusHelmArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *GenerateMultusHelmArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteDir := filepath.Dir(s.RemoteValuesPath)
	logger.Warnf("Rolling back by removing remote Multus CNI Helm artifacts directory: %s", remoteDir)
	if err := runner.Remove(ctx.GoContext(), conn, remoteDir, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove remote Multus CNI artifacts directory during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*GenerateMultusHelmArtifactsStep)(nil)
