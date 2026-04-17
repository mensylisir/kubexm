package kubeovn

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/util/helm"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

type DistributeKubeovnArtifactsStep struct {
	step.Base
	RemoteValuesPath string
	RemoteChartPath  string
}

type DistributeKubeovnArtifactsStepBuilder struct {
	step.Builder[DistributeKubeovnArtifactsStepBuilder, *DistributeKubeovnArtifactsStep]
}

func NewDistributeKubeovnArtifactsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DistributeKubeovnArtifactsStepBuilder {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeKubeOvn) {
		return nil
	}

	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeKubeOvn))
	if chart == nil {
		return nil
	}

	s := &DistributeKubeovnArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute Kube-OVN Helm artifacts to remote host", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	remoteDir := filepath.Join(ctx.GetUploadDir(), chart.RepoName(), chart.Version)

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, valuesFileName)
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(DistributeKubeovnArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeKubeovnArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeKubeovnArtifactsStep) getLocalPaths(ctx runtime.ExecutionContext) (localValuesPath, localChartPath string, err error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeKubeOvn))
	if chart == nil {
		return "", "", fmt.Errorf("cannot find chart info for kube-ovn in BOM")
	}

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())
	chartDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))
	localValuesPath = filepath.Join(chartDir, chart.Version, valuesFileName)

	localChartPath = chart.LocalPath(ctx.GetGlobalWorkDir())

	return localValuesPath, localChartPath, nil
}

func (s *DistributeKubeovnArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeKubeOvn) {
		logger.Info("Kube-OVN is not enabled, skipping.")
		return true, nil
	}

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
		logger.Info("All Kube-OVN artifacts already exist on the remote host and are up-to-date. Skipping.")
		return true, nil
	}

	return false, nil
}

func (s *DistributeKubeovnArtifactsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	localValuesPath, localChartPath, err := s.getLocalPaths(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to get local paths")
		return result, err
	}
	valuesContent, err := os.ReadFile(localValuesPath)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to read local values file %s", localValuesPath))
		return result, err
	}
	chartContent, err := os.ReadFile(localChartPath)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to read local chart file %s", localChartPath))
		return result, err
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	remoteDir := filepath.Dir(s.RemoteChartPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to create remote directory %s on host %s", remoteDir, ctx.GetHost().GetName()))
		return result, err
	}

	logger.Infof("Uploading rendered values.yaml to %s:%s", ctx.GetHost().GetName(), s.RemoteValuesPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(valuesContent), s.RemoteValuesPath, "0644", s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to upload values.yaml to %s", ctx.GetHost().GetName()))
		return result, err
	}

	logger.Infof("Uploading chart %s to %s:%s", filepath.Base(localChartPath), ctx.GetHost().GetName(), s.RemoteChartPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(chartContent), s.RemoteChartPath, "0644", s.Sudo); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to upload helm chart to %s", ctx.GetHost().GetName()))
		return result, err
	}

	logger.Info("Successfully distributed Kube-OVN artifacts to remote host.")
	result.MarkCompleted("Kube-OVN artifacts distributed successfully")
	return result, nil
}

func (s *DistributeKubeovnArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*DistributeKubeovnArtifactsStep)(nil)
