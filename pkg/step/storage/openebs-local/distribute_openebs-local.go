package openebslocal

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

// DistributeOpenEBSArtifactsStep is responsible for distributing the OpenEBS artifacts.
type DistributeOpenEBSArtifactsStep struct {
	step.Base
	RemoteValuesPath string
	RemoteChartPath  string
}

// DistributeOpenEBSArtifactsStepBuilder is used to build instances.
type DistributeOpenEBSArtifactsStepBuilder struct {
	step.Builder[DistributeOpenEBSArtifactsStepBuilder, *DistributeOpenEBSArtifactsStep]
}

// NewDistributeOpenEBSArtifactsStepBuilder is the constructor.
func NewDistributeOpenEBSArtifactsStepBuilder(ctx runtime.Context, instanceName string) *DistributeOpenEBSArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart("openebs")
	if chart == nil {
		// TODO: Add a check for whether openebs is enabled
		return nil
	}

	s := &DistributeOpenEBSArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute OpenEBS Helm artifacts to remote host", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	remoteDir := filepath.Join(ctx.GetUploadDir(), chart.RepoName(), chart.ChartName()+"-"+chart.Version)
	s.RemoteValuesPath = filepath.Join(remoteDir, "openebs-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(DistributeOpenEBSArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeOpenEBSArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeOpenEBSArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// TODO: Add a check to see if openebs is enabled
	return false, nil
}

func (s *DistributeOpenEBSArtifactsStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("openebs")
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for openebs in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "openebs-values.yaml"), nil
}

func (s *DistributeOpenEBSArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	localValuesPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}
	valuesContent, err := os.ReadFile(localValuesPath)
	if err != nil {
		return fmt.Errorf("failed to read generated values file from agreed path %s: %w. Ensure GenerateOpenEBSValuesStep ran successfully.", localValuesPath, err)
	}

	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("openebs")
	if chart == nil {
		return fmt.Errorf("cannot find chart info for openebs in BOM")
	}
	localChartPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	chartContent, err := os.ReadFile(localChartPath)
	if err != nil {
		return fmt.Errorf("failed to read offline chart file from %s: %w. Ensure DownloadOpenEBSChartStep ran successfully.", localChartPath, err)
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

	logger.Info("Successfully distributed OpenEBS artifacts to remote host.")
	return nil
}

func (s *DistributeOpenEBSArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*DistributeOpenEBSArtifactsStep)(nil)
