package helm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DownloadHelmChartsStep 负责从网络上下载所有必需的 Helm Charts。
type DownloadHelmChartsStep struct {
	step.Base
	HelmBinaryPath string // Helm 可执行文件的路径，在 Precheck 中找到
	Concurrency    int
	ChartsDir      string // Chart 的基础存储目录
}

// DownloadHelmChartsStepBuilder 用于构建 DownloadHelmChartsStep 实例。
type DownloadHelmChartsStepBuilder struct {
	step.Builder[DownloadHelmChartsStepBuilder, *DownloadHelmChartsStep]
}

// NewDownloadHelmChartsStepBuilder 是 DownloadHelmChartsStep 的构造函数。
func NewDownloadHelmChartsStepBuilder(ctx runtime.Context, instanceName string) *DownloadHelmChartsStepBuilder {
	s := &DownloadHelmChartsStep{
		Concurrency: 5,
		// 使用你的 HelmChart.LocalPath() 方法需要一个基础目录
		// 这里我们从运行时上下文中获取，假设它提供了这个功能
		ChartsDir: filepath.Join(ctx.GetClusterArtifactsDir(), "helm"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download all required Helm charts to local directory", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 30 * time.Minute // Helm 操作通常比下载大二进制文件快

	b := new(DownloadHelmChartsStepBuilder).Init(s)
	return b
}

// WithConcurrency 设置下载的并发数。
func (b *DownloadHelmChartsStepBuilder) WithConcurrency(c int) *DownloadHelmChartsStepBuilder {
	if c > 0 {
		b.Step.Concurrency = c
	}
	return b
}

// Meta 返回步骤的元数据。
func (s *DownloadHelmChartsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// Precheck 检查 helm 命令是否存在。
func (s *DownloadHelmChartsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return false, fmt.Errorf("helm command not found in PATH, please install it first")
	}
	s.HelmBinaryPath = helmPath
	return false, nil
}

// Run 执行下载 Helm charts 的主要逻辑。
func (s *DownloadHelmChartsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	helmProvider := helm.NewHelmProvider(ctx)
	chartsToDownload := helmProvider.GetCharts()

	if len(chartsToDownload) == 0 {
		logger.Info("No Helm charts need to be downloaded for this configuration.")
		return nil
	}

	// 确保基础目录存在
	if err := os.MkdirAll(s.ChartsDir, 0755); err != nil {
		return fmt.Errorf("failed to create base charts directory '%s': %w", s.ChartsDir, err)
	}

	// --- 并发下载逻辑 ---
	jobs := make(chan *helm.HelmChart, len(chartsToDownload))
	errChan := make(chan error, len(chartsToDownload))

	var wg sync.WaitGroup
	for i := 0; i < s.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chart := range jobs {
				logger.Infof("Worker %d: starting download for chart %s", workerID, chart.FullName())
				err := s.downloadChart(ctx, chart)
				if err != nil {
					logger.Errorf("Worker %d: failed to download chart %s: %v", workerID, chart.FullName(), err)
					errChan <- fmt.Errorf("failed to download chart %s: %w", chart.FullName(), err)
				} else {
					logger.Infof("Worker %d: successfully downloaded chart %s version %s", workerID, chart.FullName(), chart.Version)
				}
			}
		}(i)
	}

	for _, chart := range chartsToDownload {
		jobs <- chart
	}
	close(jobs)

	wg.Wait()
	close(errChan)

	var allErrors []string
	for err := range errChan {
		allErrors = append(allErrors, err.Error())
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("failed to download some helm charts:\n- %s", strings.Join(allErrors, "\n- "))
	}

	logger.Info("All required Helm charts have been downloaded successfully.")
	return nil
}

// downloadChart 是单个 chart 的下载逻辑。
func (s *DownloadHelmChartsStep) downloadChart(ctx runtime.ExecutionContext, chart *helm.HelmChart) error {
	// 使用你在 HelmChart 对象中定义的 LocalPath 方法来确定存储路径
	// 这里我们传递 ChartsDir 作为基础目录
	destFile := chart.LocalPath(s.ChartsDir)
	destDir := filepath.Dir(destFile)

	// 1. 确保目标目录存在
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	// 2. 检查文件是否已存在，如果存在则跳过，实现幂等性
	if _, err := os.Stat(destFile); err == nil {
		ctx.GetLogger().Infof("Chart package %s already exists, skipping download.", destFile)
		return nil
	}

	// 3. 添加/更新 Helm 仓库
	// 使用 --force-update 可以确保我们总是获取最新的 repo 索引
	repoAddCmd := exec.Command(s.HelmBinaryPath, "repo", "add", chart.RepoName(), chart.RepoURL(), "--force-update")
	if output, err := repoAddCmd.CombinedOutput(); err != nil {
		// 有时 repo 已经存在会报错，但我们可以忽略这种特定错误
		if !strings.Contains(string(output), "already exists") {
			return fmt.Errorf("failed to add helm repo '%s' from '%s': %w\nOutput: %s", chart.RepoName(), chart.RepoURL(), err, string(output))
		}
	}

	// 4. 拉取 Helm Chart 包
	// 我们下载 .tgz 压缩包，而不是解压它，这更适合离线包分发
	pullCmd := exec.Command(s.HelmBinaryPath,
		"pull",
		chart.FullName(),
		"--version", chart.Version,
		"--destination", destDir,
	)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to pull chart '%s' version '%s': %w\nOutput: %s", chart.FullName(), chart.Version, err, string(output))
	}

	return nil
}

// Rollback 定义回滚操作。
func (s *DownloadHelmChartsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for DownloadHelmChartsStep is a no-op. You can manually delete the charts directory:", "path", s.ChartsDir)
	return nil
}

// 确保 DownloadHelmChartsStep 实现了 Step 接口
var _ step.Step = (*DownloadHelmChartsStep)(nil)
