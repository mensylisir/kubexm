package nfs

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

// GenerateNFSProvisionerValuesStep is responsible for generating the Helm values file for the NFS Provisioner.
type GenerateNFSProvisionerValuesStep struct {
	step.Base
	ImageRegistry string
	ImageTag      string
	NfsServer     string
	NfsPath       string
}

// GenerateNFSProvisionerValuesStepBuilder is used to build instances.
type GenerateNFSProvisionerValuesStepBuilder struct {
	step.Builder[GenerateNFSProvisionerValuesStepBuilder, *GenerateNFSProvisionerValuesStep]
}

// NewGenerateNFSProvisionerValuesStepBuilder is the constructor.
func NewGenerateNFSProvisionerValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateNFSProvisionerValuesStepBuilder {
	imageProvider := images.NewImageProvider(&ctx)

	provisionerImage := imageProvider.GetImage("nfs-subdir-external-provisioner")
	if provisionerImage == nil {
		// TODO: Add a check for whether nfs is enabled
		fmt.Fprintf(os.Stderr, "Error: NFS Provisioner is enabled but its image is not found in BOM for K8s version %s\n", ctx.GetClusterConfig().Spec.Kubernetes.Version)
		return nil
	}

	s := &GenerateNFSProvisionerValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate NFS Provisioner Helm values file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRegistry = provisionerImage.Registry()
	if reg := ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting.PrivateRegistry; reg != "" {
		s.ImageRegistry = reg
	}
	s.ImageTag = provisionerImage.Tag()

	// These values should be provided by the cluster configuration.
	// TODO: Add logic to read these from the cluster config spec.
	// For now, using placeholder values.
	s.NfsServer = "127.0.0.1" // Placeholder
	s.NfsPath = "/nfs/share" // Placeholder

	b := new(GenerateNFSProvisionerValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateNFSProvisionerValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateNFSProvisionerValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// TODO: Add a check to see if nfs is enabled
	return false, nil
}

func (s *GenerateNFSProvisionerValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("nfs-subdir-external-provisioner")
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for nfs-subdir-external-provisioner in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetClusterArtifactsDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "nfs-provisioner-values.yaml"), nil
}

func (s *GenerateNFSProvisionerValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("storage/nfs/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded nfs values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("nfsValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse nfs values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render nfs values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating NFS Provisioner Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated NFS Provisioner Helm values file.")
	return nil
}

func (s *GenerateNFSProvisionerValuesStep) Rollback(ctx runtime.ExecutionContext) error {
	if localPath, err := s.getLocalValuesPath(ctx); err == nil {
		os.Remove(localPath)
	}
	return nil
}

var _ step.Step = (*GenerateNFSProvisionerValuesStep)(nil)
