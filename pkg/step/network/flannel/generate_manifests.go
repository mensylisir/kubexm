package flannel

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

// GenerateFlannelValuesStep is responsible for generating the Helm values file for Flannel based on the cluster configuration.
// This step runs on the control plane, not interacting with any remote hosts.
type GenerateFlannelValuesStep struct {
	step.Base
	ImageFlannelRepo   string
	ImageFlannelTag    string
	ImageCNIPluginRepo string
	ImageCNIPluginTag  string
	PodCIDR            string
	BackendType        string
	BackendVXLAN       *v1alpha1.FlannelVXLANConfig
	BackendIPsec       *v1alpha1.FlannelIPsecConfig
}

// GenerateFlannelValuesStepBuilder is used to build instances.
type GenerateFlannelValuesStepBuilder struct {
	step.Builder[GenerateFlannelValuesStepBuilder, *GenerateFlannelValuesStep]
}

func NewGenerateFlannelValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateFlannelValuesStepBuilder {
	imageProvider := images.NewImageProvider(&ctx)
	flannelImage := imageProvider.GetImage("flannel")
	cniPluginImage := imageProvider.GetImage("flannel-cni-plugin")

	if flannelImage == nil || cniPluginImage == nil {
		if ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeFlannel) {
			fmt.Fprintf(os.Stderr, "Error: Flannel is enabled but 'flannel' or 'flannel-cni-plugin' image is not found in BOM for K8s version %s\n", ctx.GetClusterConfig().Spec.Kubernetes.Version)
		}
		return nil
	}

	s := &GenerateFlannelValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Flannel Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

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

	b := new(GenerateFlannelValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateFlannelValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateFlannelValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeFlannel) {
		return true, nil
	}
	return false, nil
}

// getLocalValuesPath defines the conventional storage path for values.yaml in the cluster artifacts directory.
// It is placed in the same directory as the chart .tgz file.
func (s *GenerateFlannelValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeFlannel))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for flannel in BOM")
	}
	// Get the full path of the chart .tgz file
	chartTgzPath := chart.LocalPath(ctx.GetClusterArtifactsDir())
	// Get the directory where the file is located
	chartDir := filepath.Dir(chartTgzPath)
	// Place values.yaml in this directory
	return filepath.Join(chartDir, "flannel-values.yaml"), nil
}

func (s *GenerateFlannelValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("cni/flannel/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded flannel values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("flannelValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse flannel values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render flannel values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating Flannel Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated Flannel Helm values file.")
	return nil
}

func (s *GenerateFlannelValuesStep) Rollback(ctx runtime.ExecutionContext) error {
	if localPath, err := s.getLocalValuesPath(ctx); err == nil {
		os.Remove(localPath)
	}
	return nil
}

var _ step.Step = (*GenerateFlannelValuesStep)(nil)
