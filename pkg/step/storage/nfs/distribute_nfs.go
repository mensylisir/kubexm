package nfs

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/pkg/errors"
)

type DistributeNFSProvisionerArtifactsStep struct {
	step.Base
	RemoteValuesPath string
	RemoteChartPath  string
}

type DistributeNFSProvisionerArtifactsStepBuilder struct {
	step.Builder[DistributeNFSProvisionerArtifactsStepBuilder, *DistributeNFSProvisionerArtifactsStep]
}

func NewDistributeNFSProvisionerArtifactsStepBuilder(ctx runtime.Context, instanceName string) *DistributeNFSProvisionerArtifactsStepBuilder {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Storage == nil || cfg.Spec.Storage.NFS == nil || !*cfg.Spec.Storage.NFS.Enabled {
		return nil
	}

	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart(NfsChartName)
	if chart == nil {
		return nil
	}

	s := &DistributeNFSProvisionerArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Distribute NFS Provisioner Helm artifacts to remote host", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	remoteDir := filepath.Join(ctx.GetUploadDir(), chart.RepoName(), chart.Version)
	s.RemoteValuesPath = filepath.Join(remoteDir, "nfs-provisioner-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(DistributeNFSProvisionerArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeNFSProvisionerArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeNFSProvisionerArtifactsStep) getLocalPaths(ctx runtime.ExecutionContext) (localValuesPath, localChartPath string, err error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(NfsChartName)
	if chart == nil {
		return "", "", fmt.Errorf("cannot find chart info for '%s' in BOM", NfsChartName)
	}

	chartDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))
	localValuesPath = filepath.Join(chartDir, chart.Version, "nfs-provisioner-values.yaml")

	localChartPath = chart.LocalPath(ctx.GetGlobalWorkDir())

	return localValuesPath, localChartPath, nil
}

func (s *DistributeNFSProvisionerArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Storage == nil || cfg.Spec.Storage.NFS == nil || !*cfg.Spec.Storage.NFS.Enabled {
		return true, nil
	}

	localValuesPath, localChartPath, err := s.getLocalPaths(ctx)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(localValuesPath); os.IsNotExist(err) {
		return false, errors.Wrapf(err, "local source file not found: %s. Ensure GenerateNFSProvisionerValuesStep ran.", localValuesPath)
	}
	if _, err := os.Stat(localChartPath); os.IsNotExist(err) {
		return false, errors.Wrapf(err, "local source file not found: %s. Ensure DownloadNFSProvisionerChartStep ran.", localChartPath)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, errors.Wrap(err, "failed to get connector for precheck")
	}

	valuesExistsCmd := fmt.Sprintf("test -f %s", s.RemoteValuesPath)
	chartExistsCmd := fmt.Sprintf("test -f %s", s.RemoteChartPath)

	_, errValues := runner.Run(ctx.GoContext(), conn, valuesExistsCmd, s.Sudo)
	_, errChart := runner.Run(ctx.GoContext(), conn, chartExistsCmd, s.Sudo)

	if errValues == nil && errChart == nil {
		logger.Info("Both NFS Provisioner artifacts already exist on the remote host. Skipping.")
		return true, nil
	}

	return false, nil
}

func (s *DistributeNFSProvisionerArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	localValuesPath, localChartPath, err := s.getLocalPaths(ctx)
	if err != nil {
		return err
	}
	valuesContent, err := os.ReadFile(localValuesPath)
	if err != nil {
		return fmt.Errorf("failed to read local values file %s: %w", localValuesPath, err)
	}
	chartContent, err := os.ReadFile(localChartPath)
	if err != nil {
		return fmt.Errorf("failed to read local chart file %s: %w", localChartPath, err)
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
