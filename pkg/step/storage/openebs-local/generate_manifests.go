package openebslocal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

// GenerateOpenEBSValuesStep is responsible for generating the Helm values file for OpenEBS.
type GenerateOpenEBSValuesStep struct {
	step.Base
	ImageRegistry       string
	ImageTag            string
	NdmOperatorImageTag string
	NdmDaemonImageTag   string
}

// GenerateOpenEBSValuesStepBuilder is used to build instances.
type GenerateOpenEBSValuesStepBuilder struct {
	step.Builder[GenerateOpenEBSValuesStepBuilder, *GenerateOpenEBSValuesStep]
}

// NewGenerateOpenEBSValuesStepBuilder is the constructor.
func NewGenerateOpenEBSValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateOpenEBSValuesStepBuilder {
	imageProvider := images.NewImageProvider(&ctx)

	provisionerImage := imageProvider.GetImage("openebs-provisioner-localpv")
	ndmOperatorImage := imageProvider.GetImage("openebs-ndm-operator")
	ndmDaemonImage := imageProvider.GetImage("openebs-ndm")

	if provisionerImage == nil || ndmOperatorImage == nil || ndmDaemonImage == nil {
		// TODO: Add a check for whether openebs is enabled
		fmt.Fprintf(os.Stderr, "Error: OpenEBS is enabled but one or more required images are not found in BOM for K8s version %s\n", ctx.GetClusterConfig().Spec.Kubernetes.Version)
		return nil
	}

	s := &GenerateOpenEBSValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate OpenEBS Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRegistry = provisionerImage.Registry()
	if reg := ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting.PrivateRegistry; reg != "" {
		s.ImageRegistry = reg
	}

	s.ImageTag = provisionerImage.Tag()
	s.NdmOperatorImageTag = ndmOperatorImage.Tag()
	s.NdmDaemonImageTag = ndmDaemonImage.Tag()

	b := new(GenerateOpenEBSValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateOpenEBSValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateOpenEBSValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// TODO: Add a check to see if openebs is enabled
	return false, nil
}

func (s *GenerateOpenEBSValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("openebs")
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for openebs in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetClusterArtifactsDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "openebs-values.yaml"), nil
}

func (s *GenerateOpenEBSValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("storage/openebs-local/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded openebs values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("openebsValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse openebs values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render openebs values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating OpenEBS Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated OpenEBS Helm values file.")
	return nil
}

func (s *GenerateOpenEBSValuesStep) Rollback(ctx runtime.ExecutionContext) error {
	if localPath, err := s.getLocalValuesPath(ctx); err == nil {
		os.Remove(localPath)
	}
	return nil
}

var _ step.Step = (*GenerateOpenEBSValuesStep)(nil)
