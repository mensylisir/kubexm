package nfs

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateNFSProvisionerValuesStep struct {
	step.Base
	ImageRegistry string
	ImageTag      string
	NfsServer     string
	NfsPath       string
}

type GenerateNFSProvisionerValuesStepBuilder struct {
	step.Builder[GenerateNFSProvisionerValuesStepBuilder, *GenerateNFSProvisionerValuesStep]
}

func NewGenerateNFSProvisionerValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateNFSProvisionerValuesStepBuilder {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Storage == nil || cfg.Spec.Storage.NFS == nil || !*cfg.Spec.Storage.NFS.Enabled {
		return nil
	}

	imageProvider := images.NewImageProvider(&ctx)
	provisionerImage := imageProvider.GetImage(NfsChartName)
	if provisionerImage == nil {
		ctx.GetLogger().Errorf("NFS Provisioner is enabled but its image ('%s') is not found in BOM for K8s version %s", NfsChartName, cfg.Spec.Kubernetes.Version)
		return nil
	}

	s := &GenerateNFSProvisionerValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Generate NFS Provisioner Helm values file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRegistry = provisionerImage.RegistryAddrWithNamespace()
	s.ImageTag = provisionerImage.Tag()

	s.NfsServer = cfg.Spec.Storage.NFS.Server
	s.NfsPath = cfg.Spec.Storage.NFS.Path

	b := new(GenerateNFSProvisionerValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateNFSProvisionerValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateNFSProvisionerValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(NfsChartName)
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for '%s' in BOM", NfsChartName)
	}

	chartDownloadDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))
	return filepath.Join(chartDownloadDir, chart.Version, "nfs-provisioner-values.yaml"), nil
}

func (s *GenerateNFSProvisionerValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	cfg := ctx.GetClusterConfig()
	if err != nil {
		return false, nil
	}
	if cfg.Spec.Storage == nil || cfg.Spec.Storage.NFS == nil || !*cfg.Spec.Storage.NFS.Enabled {
		return true, nil
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if _, err := os.Stat(localPath); err == nil {
		logger.Infof("NFS Provisioner values file %s already exists. Skipping generation.", localPath)
		return true, nil
	}
	return false, nil
}

func (s *GenerateNFSProvisionerValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	if s.NfsServer == "" || s.NfsPath == "" {
		return fmt.Errorf("NFS Server or Path is not configured, cannot generate values file")
	}

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
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		logger.Infof("Skipping rollback as no values path could be determined: %v", err)
		return nil
	}

	if _, statErr := os.Stat(localPath); statErr == nil {
		logger.Warnf("Rolling back by deleting generated values file: %s", localPath)
		if err := os.Remove(localPath); err != nil {
			logger.Errorf("Failed to remove file during rollback: %v", err)
		}
	} else {
		logger.Infof("Rollback unnecessary, file to be deleted does not exist: %s", localPath)
	}

	return nil
}

var _ step.Step = (*GenerateNFSProvisionerValuesStep)(nil)
