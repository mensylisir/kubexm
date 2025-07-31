package cilium

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
)

// DistributeCiliumArtifactsStep 负责将 Cilium 的 Helm Chart 和生成的 values 文件分发到远程节点。
type DistributeCiliumArtifactsStep struct {
	step.Base
	RemoteValuesPath string
	RemoteChartPath  string
}

type DistributeCiliumArtifactsStepBuilder struct {
	step.Builder[DistributeCiliumArtifactsStepBuilder, *DistributeCiliumArtifactsStep]
}

func NewDistributeCiliumArtifactsStepBuilder(ctx runtime.Context, instanceName string) *DistributeCiliumArtifactsStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart(string(common.CNITypeCilium))
	if chart == nil {
		if ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeCilium) {
			fmt.Fprintf(os.Stderr, "Error: Cilium is enabled but chart info is not found for K8s version %s\n", ctx.GetClusterConfig().Spec.Kubernetes.Version)
		}
		return nil
	}

	s := &DistributeCiliumArtifactsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute Cilium Helm artifacts to remote host", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	// 远程路径遵循同一目录的约定
	remoteDir := filepath.Join(common.DefaultUploadTmpDir, chart.RepoName(), chart.ChartName()+"-"+chart.Version)
	s.RemoteValuesPath = filepath.Join(remoteDir, "cilium-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(DistributeCiliumArtifactsStepBuilder).Init(s)
	return b
}

func (s *DistributeCiliumArtifactsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeCiliumArtifactsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeCilium) {
		return true, nil
	}
	return false, nil
}

// getLocalValuesPath 定义了与 GenerateCiliumValuesStep 完全相同的约定路径。
func (s *DistributeCiliumArtifactsStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeCilium))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for cilium in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetClusterArtifactsDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "cilium-values.yaml"), nil
}

func (s *DistributeCiliumArtifactsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	// 1. 根据约定，找到本地 artifacts 目录中的 values 文件
	localValuesPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}
	valuesContent, err := os.ReadFile(localValuesPath)
	if err != nil {
		return fmt.Errorf("failed to read generated values file from agreed path %s: %w. Ensure GenerateCiliumValuesStep ran successfully.", localValuesPath, err)
	}

	// 2. 找到本地 artifacts 目录中的 Chart 文件
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeCilium))
	if chart == nil {
		return fmt.Errorf("cannot find chart info for cilium in BOM")
	}
	localChartPath := chart.LocalPath(ctx.GetClusterArtifactsDir())
	chartContent, err := os.ReadFile(localChartPath)
	if err != nil {
		return fmt.Errorf("failed to read offline chart file from %s: %w. Ensure DownloadCiliumChartStep ran successfully.", localChartPath, err)
	}

	// 3. 上传文件到远程节点
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
	if err := runner.WriteFile(ctx.GoContext(), conn, valuesContent, s.RemoteValuesPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload values.yaml to %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Infof("Uploading chart %s to %s:%s", filepath.Base(localChartPath), ctx.GetHost().GetName(), s.RemoteChartPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, chartContent, s.RemoteChartPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to upload helm chart to %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Info("Successfully distributed Cilium artifacts to remote host.")
	return nil
}

func (s *DistributeCiliumArtifactsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		// 在回滚阶段，如果连接失败，通常最好是记录日志而不是返回错误，以允许其他回滚步骤继续
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

var _ step.Step = (*DistributeCiliumArtifactsStep)(nil)
