package ingressnginx

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

type DistributeIngressNginxArtifactsStep struct {
	step.Base
	RemoteValuesPath string
	RemoteChartPath  string
}

type DistributeIngressNginxArtifactsStepBuilder struct {
	step.Builder[DistributeIngressNginxArtifactsStepBuilder, *DistributeIngressNginxArtifactsStep]
}

func NewDistributeIngressNginxArtifactsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DistributeIngressNginxArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("ingress-nginx")
	if chart == nil {
		return nil
	}

	s := &DistributeIngressNginxArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute Ingress-Nginx Helm artifacts to remote host", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	remoteDir := filepath.Join(ctx.GetUploadDir(), chart.RepoName(), chart.Version)

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, valuesFileName)
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(DistributeIngressNginxArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeIngressNginxArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeIngressNginxArtifactsStep) getLocalPaths(ctx runtime.ExecutionContext) (localValuesPath, localChartPath string, err error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("ingress-nginx")
	if chart == nil {
		return "", "", fmt.Errorf("cannot find chart info for ingress-nginx in BOM")
	}

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())
	chartDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))
	localValuesPath = filepath.Join(chartDir, chart.Version, valuesFileName)

	localChartPath = chart.LocalPath(ctx.GetGlobalWorkDir())

	return localValuesPath, localChartPath, nil
}

func (s *DistributeIngressNginxArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	localValuesPath, localChartPath, err := s.getLocalPaths(ctx)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(localValuesPath); os.IsNotExist(err) {
		return false, errors.Wrapf(err, "local source file not found: %s. Ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle).", localValuesPath)
	}
	if _, err := os.Stat(localChartPath); os.IsNotExist(err) {
		return false, errors.Wrapf(err, "local source file not found: %s. Ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle).", localChartPath)
	}

	valuesDone, err := helpers.CheckRemoteFileIntegrity(ctx, localValuesPath, s.RemoteValuesPath, s.Sudo)
	if err != nil {
		return false, err
	}
	chartDone, err := helpers.CheckRemoteFileIntegrity(ctx, localChartPath, s.RemoteChartPath, s.Sudo)
	if err != nil {
		return false, err
	}

	if valuesDone && chartDone {
		logger.Info("All Ingress-Nginx artifacts already exist on the remote host and are up-to-date. Skipping.")
		return true, nil
	}

	return false, nil
}

func (s *DistributeIngressNginxArtifactsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	localValuesPath, localChartPath, err := s.getLocalPaths(ctx)
	if err != nil {
		result.MarkFailed(err, err.Error())
		return result, err
	}
	valuesContent, err := os.ReadFile(localValuesPath)
	if err != nil {
		result.MarkFailed(err, "failed to read local values file")
		return result, err
	}
	chartContent, err := os.ReadFile(localChartPath)
	if err != nil {
		result.MarkFailed(err, "failed to read local chart file")
		return result, err
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, err.Error())
		return result, err
	}

	remoteDir := filepath.Dir(s.RemoteChartPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to create remote directory")
		return result, err
	}

	logger.Infof("Uploading rendered values.yaml to %s:%s", ctx.GetHost().GetName(), s.RemoteValuesPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(valuesContent), s.RemoteValuesPath, "0644", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to upload values.yaml")
		return result, err
	}

	logger.Infof("Uploading chart %s to %s:%s", filepath.Base(localChartPath), ctx.GetHost().GetName(), s.RemoteChartPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(chartContent), s.RemoteChartPath, "0644", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to upload helm chart")
		return result, err
	}

	logger.Info("Successfully distributed Ingress-Nginx artifacts to remote host.")
	result.MarkCompleted("Ingress-Nginx artifacts distributed successfully")
	return result, nil
}

func (s *DistributeIngressNginxArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*DistributeIngressNginxArtifactsStep)(nil)
