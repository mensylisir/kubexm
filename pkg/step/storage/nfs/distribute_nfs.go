package nfs

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
)

// DistributeNFSProvisionerArtifactsStep is responsible for distributing the NFS Provisioner artifacts.
type DistributeNFSProvisionerArtifactsStep struct {
	step.Base
	RemoteValuesPath string
	RemoteChartPath  string
}

// DistributeNFSProvisionerArtifactsStepBuilder is used to build instances.
type DistributeNFSProvisionerArtifactsStepBuilder struct {
	step.Builder[DistributeNFSProvisionerArtifactsStepBuilder, *DistributeNFSProvisionerArtifactsStep]
}

// NewDistributeNFSProvisionerArtifactsStepBuilder is the constructor.
func NewDistributeNFSProvisionerArtifactsStepBuilder(ctx runtime.Context, instanceName string) *DistributeNFSProvisionerArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart("nfs-subdir-external-provisioner")
	if chart == nil {
		// TODO: Add a check for whether nfs provisioner is enabled
		return nil
	}

	s := &DistributeNFSProvisionerArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute NFS Provisioner Helm artifacts to remote host", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	remoteDir := filepath.Join(ctx.GetUploadDir(), chart.RepoName(), chart.ChartName()+"-"+chart.Version)
	s.RemoteValuesPath = filepath.Join(remoteDir, "nfs-provisioner-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(DistributeNFSProvisionerArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeNFSProvisionerArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeNFSProvisionerArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// TODO: Add a check to see if nfs provisioner is enabled
	return false, nil
}

func (s *DistributeNFSProvisionerArtifactsStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("nfs-subdir-external-provisioner")
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for nfs-subdir-external-provisioner in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "nfs-provisioner-values.yaml"), nil
}

func (s *DistributeNFSProvisionerArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	localValuesPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}
	valuesContent, err := os.ReadFile(localValuesPath)
	if err != nil {
		return fmt.Errorf("failed to read generated values file from agreed path %s: %w. Ensure GenerateNFSProvisionerValuesStep ran successfully.", localValuesPath, err)
	}

	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("nfs-subdir-external-provisioner")
	if chart == nil {
		return fmt.Errorf("cannot find chart info for nfs-subdir-external-provisioner in BOM")
	}
	localChartPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	chartContent, err := os.ReadFile(localChartPath)
	if err != nil {
		return fmt.Errorf("failed to read offline chart file from %s: %w. Ensure DownloadNFSProvisionerChartStep ran successfully.", localChartPath, err)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	remoteDir := filepath.Dir(s.RemoteChartPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote directory %s on host %s: %w", remoteDir, ctx.GetHost().GetName(), err)
	}

	logger.Infof("Uploading rendered values.yaml to %s:%s", ctx.GetHost().GetName(), s.RemoteValuesPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(valuesContent), s.RemoteValuesPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload values.yaml to %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Infof("Uploading chart %s to %s:%s", filepath.Base(localChartPath), ctx.GetHost().GetName(), s.RemoteChartPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(chartContent), s.RemoteChartPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload helm chart to %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Info("Successfully distributed NFS Provisioner artifacts to remote host.")
	return nil
}

func (s *DistributeNFSProvisionerArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	remoteDir := filepath.Dir(s.RemoteValuesPath)
	logger.Warnf("Rolling back by removing remote Helm artifacts directory: %s", remoteDir)
	if err := runner.Remove(ctx.GoContext(), conn, remoteDir, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove remote artifacts directory '%s' during rollback: %v", remoteDir, err)
	}
	return nil
}

var _ step.Step = (*DistributeNFSProvisionerArtifactsStep)(nil)
