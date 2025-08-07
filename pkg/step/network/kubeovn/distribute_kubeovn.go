package kubeovn

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
)

type DistributeKubeovnArtifactsStep struct {
	step.Base
	RemoteValuesPath string
	RemoteChartPath  string
}

type DistributeKubeovnArtifactsStepBuilder struct {
	step.Builder[DistributeKubeovnArtifactsStepBuilder, *DistributeKubeovnArtifactsStep]
}

func NewDistributeKubeovnArtifactsStepBuilder(ctx runtime.Context, instanceName string) *DistributeKubeovnArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart(string(common.CNITypeKubeOvn))
	if chart == nil {
		if ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeKubeOvn) {
			fmt.Fprintf(os.Stderr, "Error: Kube-OVN is enabled but chart info is not found for K8s version %s\n", ctx.GetClusterConfig().Spec.Kubernetes.Version)
		}
		return nil
	}

	s := &DistributeKubeovnArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute Kube-OVN Helm artifacts to remote host", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	remoteDir := filepath.Join(ctx.GetUploadDir(), chart.RepoName(), chart.ChartName()+"-"+chart.Version)
	s.RemoteValuesPath = filepath.Join(remoteDir, "kubeovn-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(DistributeKubeovnArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeKubeovnArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeKubeovnArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeKubeOvn) {
		return true, nil
	}
	return false, nil
}

func (s *DistributeKubeovnArtifactsStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeKubeOvn))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for kube-ovn in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "kubeovn-values.yaml"), nil
}

func (s *DistributeKubeovnArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	localValuesPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}
	valuesContent, err := os.ReadFile(localValuesPath)
	if err != nil {
		return fmt.Errorf("failed to read generated values file from agreed path %s: %w. Ensure GenerateKubeovnValuesStep ran successfully.", localValuesPath, err)
	}

	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeKubeOvn))
	if chart == nil {
		return fmt.Errorf("cannot find chart info for kube-ovn in BOM")
	}
	localChartPath := chart.LocalPath(ctx.GetGlobalWorkDir())
	chartContent, err := os.ReadFile(localChartPath)
	if err != nil {
		return fmt.Errorf("failed to read offline chart file from %s: %w. Ensure DownloadKubeovnChartStep ran successfully.", localChartPath, err)
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

	logger.Info("Successfully distributed Kube-OVN artifacts to remote host.")
	return nil
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
