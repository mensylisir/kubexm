package calico

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateCalicoHelmArtifactsStep struct {
	step.Base
	RemoteValuesPath     string
	Chart                *helm.HelmChart
	LocalPulledChartPath string
	RemoteChartPath      string

	OperatorImage        string
	OperatorNodeSelector map[string]string
	OperatorTolerations  []map[string]string
	Registry             string
	IPPools              []v1alpha1.CalicoIPPool
	VethMTU              int
	LogSeverityScreen    string
	TyphaEnabled         bool
	TyphaReplicas        int
	TyphaNodeSelector    map[string]string
}

type GenerateCalicoHelmArtifactsStepBuilder struct {
	step.Builder[GenerateCalicoHelmArtifactsStepBuilder, *GenerateCalicoHelmArtifactsStep]
}

func NewGenerateCalicoHelmArtifactsStepBuilder(ctx runtime.Context, instanceName string) *GenerateCalicoHelmArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	calicoChart := helmProvider.GetChart(string(common.CNITypeCalico))
	if calicoChart == nil {
		return nil
	}
	s := &GenerateCalicoHelmArtifactsStep{
		Chart: calicoChart,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Calico Helm artifacts", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	localChartDir := filepath.Dir(calicoChart.LocalPath(common.DefaultKubexmTmpDir))
	s.LocalPulledChartPath = localChartDir

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, calicoChart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, "calico-values.yaml")

	clusterCfg := ctx.GetClusterConfig()
	imageProvider := images.NewImageProvider(&ctx)
	operatorImg := imageProvider.GetImage("tigera-operator")
	if operatorImg == nil {
		return nil
	}
	s.OperatorImage = operatorImg.FullName()
	s.OperatorNodeSelector = map[string]string{"kubernetes.io/os": "linux"}
	s.OperatorTolerations = []map[string]string{
		{"key": "node-role.kubernetes.io/control-plane", "operator": "Exists", "effect": "NoSchedule"},
	}
	s.VethMTU = 1440
	s.LogSeverityScreen = "Info"
	s.Registry = clusterCfg.Spec.Registry.MirroringAndRewriting.PrivateRegistry
	s.TyphaNodeSelector = map[string]string{"node-role.kubernetes.io/control-plane": ""}

	defaultNat := true
	defaultBlockSize := 26
	defaultPool := v1alpha1.CalicoIPPool{
		CIDR:          clusterCfg.Spec.Network.KubePodsCIDR,
		Encapsulation: "VXLAN",
		NatOutgoing:   &defaultNat,
		BlockSize:     &defaultBlockSize,
	}

	userCalicoCfg := clusterCfg.Spec.Network.Calico
	if userCalicoCfg != nil {
		if userCalicoCfg.Networking != nil {
			if userCalicoCfg.Networking.VethMTU != nil {
				s.VethMTU = *userCalicoCfg.Networking.VethMTU
			}
			if userCalicoCfg.Networking.IPIPMode != "" {
				defaultPool.Encapsulation = "IPIP"
			}
			if userCalicoCfg.Networking.VXLANMode != "" {
				defaultPool.Encapsulation = "VXLAN"
			}
		}

		if userCalicoCfg.IPAM != nil && len(userCalicoCfg.IPAM.Pools) > 0 {
			s.IPPools = userCalicoCfg.IPAM.Pools
			for i := range s.IPPools {
				if s.IPPools[i].Encapsulation == "" {
					s.IPPools[i].Encapsulation = "VXLAN"
				}
				if s.IPPools[i].NatOutgoing == nil {
					s.IPPools[i].NatOutgoing = &defaultNat
				}
				if s.IPPools[i].BlockSize == nil {
					s.IPPools[i].BlockSize = &defaultBlockSize
				}
			}
		} else {
			s.IPPools = []v1alpha1.CalicoIPPool{defaultPool}
		}

		if userCalicoCfg.FelixConfiguration != nil && userCalicoCfg.FelixConfiguration.LogSeverityScreen != "" {
			s.LogSeverityScreen = userCalicoCfg.FelixConfiguration.LogSeverityScreen
		}

		if userCalicoCfg.TyphaDeployment != nil {
			if userCalicoCfg.TyphaDeployment.Replicas != nil {
				s.TyphaReplicas = *userCalicoCfg.TyphaDeployment.Replicas
			}
			if userCalicoCfg.TyphaDeployment.NodeSelector != nil {
				s.TyphaNodeSelector = userCalicoCfg.TyphaDeployment.NodeSelector
			}
			if userCalicoCfg.TyphaDeployment.Enabled != nil {
				s.TyphaEnabled = *userCalicoCfg.TyphaDeployment.Enabled
			}
		}
	} else {
		s.IPPools = []v1alpha1.CalicoIPPool{defaultPool}
	}

	if userCalicoCfg == nil || userCalicoCfg.TyphaDeployment == nil || userCalicoCfg.TyphaDeployment.Enabled == nil {
		if len(ctx.GetHostsByRole(common.RoleKubernetes)) > 50 {
			s.TyphaEnabled = true
		}
	}
	b := new(GenerateCalicoHelmArtifactsStepBuilder).Init(s)
	return b
}

func (s *GenerateCalicoHelmArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCalicoHelmArtifactsStep) Run(ctx runtime.ExecutionContext) error {
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

	chartFullName := fmt.Sprintf("%s/%s", repoName, s.Chart.ChartName())
	pullArgs := []string{"pull", chartFullName, "--destination", s.LocalPulledChartPath}
	if s.Chart.Version != "" {
		pullArgs = append(pullArgs, "--version", s.Chart.Version)
	}

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

	logger.Info("Rendering values.yaml from template...")
	valuesTemplateContent, err := templates.Get("cni/calico/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("calicoValues").Parse(valuesTemplateContent)
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

	logger.Info("Calico Helm artifacts, including rendered values.yaml, uploaded successfully.")
	return nil
}

func (s *GenerateCalicoHelmArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *GenerateCalicoHelmArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*GenerateCalicoHelmArtifactsStep)(nil)
