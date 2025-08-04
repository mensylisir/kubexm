package argocd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
)

type DistributeArgoCDArtifactsStep struct {
	step.Base
	RemoteValuesPath string
	RemoteChartPath  string
}

type DistributeArgoCDArtifactsStepBuilder struct {
	step.Builder[DistributeArgoCDArtifactsStepBuilder, *DistributeArgoCDArtifactsStep]
}

func NewDistributeArgoCDArtifactsStepBuilder(ctx runtime.Context, instanceName string) *DistributeArgoCDArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart("argocd")
	if chart == nil {
		// TODO: Add a check for whether argocd is enabled
		return nil
	}

	s := &DistributeArgoCDArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute Argo CD Helm artifacts to remote host", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	remoteDir := filepath.Join(ctx.GetUploadDir(), ctx.GetHost().GetName(), chart.RepoName(), chart.ChartName()+"-"+chart.Version)
	s.RemoteValuesPath = filepath.Join(remoteDir, "argocd-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(DistributeArgoCDArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeArgoCDArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeArgoCDArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// TODO: Add a check to see if argocd is enabled
	return false, nil
}

func (s *DistributeArgoCDArtifactsStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("argocd")
	if chart == nil {
		return "", "", fmt.Errorf("cannot find chart info for argocd in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	valueYamlPath := filepath.Join(ctx.GetClusterWorkDir(), chart.ChartName(), chart.Version, "argocd-values.yaml")
	return chartTgzPath, valueYamlPath, nil
}

func (s *DistributeArgoCDArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	localTgzPath, localValuesPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
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
	if err := helpers.UploadFile(ctx, conn, localValuesPath, s.RemoteValuesPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload values.yaml to %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Infof("Uploading chart %s to %s:%s", filepath.Base(localTgzPath), ctx.GetHost().GetName(), s.RemoteChartPath)
	if err := helpers.UploadFile(ctx, conn, localTgzPath, s.RemoteChartPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload helm chart to %s: %w", ctx.GetHost().GetName(), err)
	}
	logger.Info("Successfully distributed Argo CD artifacts to remote host.")
	return nil
}

func (s *DistributeArgoCDArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*DistributeArgoCDArtifactsStep)(nil)
