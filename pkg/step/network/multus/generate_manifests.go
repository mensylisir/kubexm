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

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateMultusHelmArtifactsStep struct {
	step.Base
	RemoteValuesPath     string
	ChartSourceDecision  *helpers.ChartSourceDecision
	LocalPulledChartPath string
	RemoteChartPath      string

	ImageRepository string
	ImageTag        string
}

type GenerateMultusHelmArtifactsStepBuilder struct {
	step.Builder[GenerateMultusHelmArtifactsStepBuilder, *GenerateMultusHelmArtifactsStep]
}

func NewGenerateMultusHelmArtifactsStepBuilder(ctx runtime.Context, instanceName string) *GenerateMultusHelmArtifactsStepBuilder {
	s := &GenerateMultusHelmArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Multus CNI Helm artifacts", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	s.ChartSourceDecision = helpers.DecideMultusChartSource(ctx)

	s.LocalPulledChartPath = filepath.Join(common.DefaultKubexmTmpDir, "multus")
	remoteDir := filepath.Join(common.DefaultUploadTmpDir, "multus")
	s.RemoteValuesPath = filepath.Join(remoteDir, "multus-values.yaml")

	s.ImageRepository = "ghcr.io/k8snetworkplumbingwg/multus-cni"
	s.ImageTag = "v4.0.2"

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Network.Multus != nil &&
		clusterCfg.Spec.Network.Multus.Installation != nil &&
		clusterCfg.Spec.Network.Multus.Installation.Image != "" {

		fullImage := clusterCfg.Spec.Network.Multus.Installation.Image

		if lastColon := strings.LastIndex(fullImage, ":"); lastColon != -1 {
			if !strings.Contains(fullImage[lastColon:], "/") {
				s.ImageRepository = fullImage[:lastColon]
				s.ImageTag = fullImage[lastColon+1:]
			} else {
				s.ImageRepository = fullImage
				s.ImageTag = "latest"
			}
		} else {
			s.ImageRepository = fullImage
			s.ImageTag = "latest"
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

	repoName := s.ChartSourceDecision.RepoName
	repoURL := s.ChartSourceDecision.RepoURL

	repoAddCmd := exec.Command(helmPath, "repo", "add", repoName, repoURL)
	if output, err := repoAddCmd.CombinedOutput(); err != nil && !strings.Contains(string(output), "already exists") {
		return fmt.Errorf("failed to add helm repo '%s' from '%s': %w, output: %s", repoName, repoURL, err, string(output))
	}

	repoUpdateCmd := exec.Command(helmPath, "repo", "update", repoName)
	if err := repoUpdateCmd.Run(); err != nil {
		return fmt.Errorf("failed to update helm repo %s: %w", repoName, err)
	}

	chartFullName := fmt.Sprintf("%s/%s", repoName, s.ChartSourceDecision.ChartName)
	pullArgs := []string{"pull", chartFullName, "--destination", s.LocalPulledChartPath}
	if s.ChartSourceDecision.Version != "" {
		pullArgs = append(pullArgs, "--version", s.ChartSourceDecision.Version)
	}

	logger.Infof("Pulling Multus CNI Helm chart with command: helm %s", strings.Join(pullArgs, " "))
	pullCmd := exec.Command(helmPath, pullArgs...)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull Multus CNI helm chart: %w, output: %s", err, string(output))
	}

	pulledFiles, _ := filepath.Glob(filepath.Join(s.LocalPulledChartPath, "*.tgz"))
	if len(pulledFiles) == 0 {
		return fmt.Errorf("Multus CNI helm chart .tgz file not found in %s after pull", s.LocalPulledChartPath)
	}
	actualLocalChartPath := pulledFiles[0]
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
