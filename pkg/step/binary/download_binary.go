package binary

import (
	"crypto/sha256"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DownloadBinariesStep 负责下载所有必需的二进制文件。
type DownloadBinariesStep struct {
	step.Base
	Concurrency int
}

// DownloadBinariesStepBuilder 用于构建 DownloadBinariesStep 实例。
type DownloadBinariesStepBuilder struct {
	step.Builder[DownloadBinariesStepBuilder, *DownloadBinariesStep]
}

// NewDownloadBinariesStepBuilder 是 DownloadBinariesStep 的构造函数。
func NewDownloadBinariesStepBuilder(ctx runtime.Context, instanceName string) *DownloadBinariesStepBuilder {
	s := &DownloadBinariesStep{
		Concurrency: 5, // 默认并发数为 5
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download all required binaries to local directories", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 60 * time.Minute

	b := new(DownloadBinariesStepBuilder).Init(s)
	return b
}

// WithConcurrency 设置下载的并发数。
func (b *DownloadBinariesStepBuilder) WithConcurrency(c int) *DownloadBinariesStepBuilder {
	if c > 0 {
		b.Step.Concurrency = c
	}
	return b
}

// Meta 返回步骤的元数据。
func (s *DownloadBinariesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// getRequiredArchs 从集群配置中获取所有需要支持的 CPU 架构。
// (这个函数可以放到一个公共的 helper 包中，与 SaveImagesStep 共享)
func (s *DownloadBinariesStep) getRequiredArchs(ctx runtime.ExecutionContext) (map[string]bool, error) {
	archs := make(map[string]bool)
	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found in cluster configuration")
	}
	for _, host := range allHosts {
		archs[host.GetArch()] = true
	}
	return archs, nil
}

// Precheck 检查执行此步骤所需的前置条件。
func (s *DownloadBinariesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// 下载步骤通常不需要预检，除非有特殊依赖。
	return false, nil
}

// Run 执行下载二进制文件的主要逻辑。
func (s *DownloadBinariesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	requiredArchs, err := s.getRequiredArchs(ctx)
	if err != nil {
		return err
	}

	binaryProvider := binary.NewBinaryProvider(ctx)
	var binariesToDownload []*binary.Binary
	for arch := range requiredArchs {
		binariesForArch, err := binaryProvider.GetBinaries(arch)
		if err != nil {
			return fmt.Errorf("failed to get binaries list for arch %s: %w", arch, err)
		}
		binariesToDownload = append(binariesToDownload, binariesForArch...)
	}

	if len(binariesToDownload) == 0 {
		logger.Info("No binaries need to be downloaded for this configuration.")
		return nil
	}

	// --- 并发下载逻辑 ---
	jobs := make(chan *binary.Binary, len(binariesToDownload))
	errChan := make(chan error, len(binariesToDownload))

	var wg sync.WaitGroup
	for i := 0; i < s.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for b := range jobs {
				logger.Infof("Worker %d: starting download for %s (%s)", workerID, b.FileName(), b.Arch)
				err := s.downloadAndVerify(b)
				if err != nil {
					logger.Errorf("Worker %d: failed to download %s: %v", workerID, b.FileName(), err)
					errChan <- fmt.Errorf("failed to download %s for %s: %w", b.FileName(), b.Arch, err)
				} else {
					logger.Infof("Worker %d: successfully downloaded %s", workerID, b.FileName())
				}
			}
		}(i)
	}

	for _, b := range binariesToDownload {
		jobs <- b
	}
	close(jobs)

	wg.Wait()
	close(errChan)

	var allErrors []string
	for err := range errChan {
		allErrors = append(allErrors, err.Error())
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("failed to download some binaries:\n- %s", strings.Join(allErrors, "\n- "))
	}

	logger.Info("All required binaries have been downloaded successfully.")
	return nil
}

// downloadAndVerify 是单个文件的下载和校验逻辑。
func (s *DownloadBinariesStep) downloadAndVerify(b *binary.Binary) error {
	destPath := b.FilePath()

	// 1. 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(destPath), err)
	}

	// 2. 如果文件已存在，检查校验和 (如果提供了)
	if _, err := os.Stat(destPath); err == nil {
		if b.Checksum() == "" || b.Checksum() == "dummy-etcd-checksum-val" { // 忽略未提供或虚拟的校验和
			// fmt.Printf("File %s already exists, skipping download as no checksum is provided.\n", b.FileName())
			return nil
		}

		// 文件存在且有校验和，进行校验
		match, err := verifyChecksum(destPath, b.Checksum())
		if err != nil {
			return fmt.Errorf("failed to verify checksum for existing file %s: %w", destPath, err)
		}
		if match {
			// fmt.Printf("File %s already exists and checksum matches. Skipping download.\n", b.FileName())
			return nil
		}
		// fmt.Printf("File %s exists but checksum mismatch. Re-downloading.\n", destPath)
	}

	// 3. 执行下载
	url := b.URL()
	// fmt.Printf("Downloading %s from %s ...\n", b.FileName(), url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status for %s: %s", url, resp.Status)
	}

	// 创建临时文件进行下载，避免下载中断导致文件损坏
	tmpFile, err := os.Create(destPath + ".tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close() // 确保关闭

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name()) // 下载失败，删除临时文件
		return fmt.Errorf("failed to copy response body to file: %w", err)
	}

	// 确保所有内容都写入磁盘
	if err := tmpFile.Sync(); err != nil {
		os.Remove(tmpFile.Name())
		return fmt.Errorf("failed to sync temp file to disk: %w", err)
	}

	// 4. 下载后重命名
	if err := os.Rename(tmpFile.Name(), destPath); err != nil {
		return fmt.Errorf("failed to rename temp file to final destination: %w", err)
	}

	// 5. 下载后再次校验 (如果提供了)
	if b.Checksum() != "" && b.Checksum() != "dummy-etcd-checksum-val" {
		match, err := verifyChecksum(destPath, b.Checksum())
		if err != nil {
			return fmt.Errorf("failed to verify checksum for downloaded file %s: %w", destPath, err)
		}
		if !match {
			return fmt.Errorf("checksum mismatch for downloaded file %s", destPath)
		}
		// fmt.Printf("Checksum for %s verified.\n", b.FileName())
	}

	return nil
}

// verifyChecksum 计算文件的 SHA256 哈希值并与预期值比较。
func verifyChecksum(filePath string, expectedChecksum string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}

	actualChecksum := fmt.Sprintf("%x", h.Sum(nil))
	return actualChecksum == expectedChecksum, nil
}

// Rollback 定义回滚操作。对于下载步骤，通常是清理已下载的文件。
func (s *DownloadBinariesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	// 注意：这里的回滚可能比较危险，因为它会删除所有下载的二进制文件。
	// 也许只打印警告信息更安全。
	// For example:
	workDir := ctx.GetGlobalWorkDir()
	clusterName := ctx.GetClusterConfig().Name
	binariesDir := filepath.Join(workDir, "kubexm", clusterName) // 根据你的路径结构调整
	logger.Warnf("Rollback is triggered. You might want to manually delete the binaries directory: %s", binariesDir)
	return nil
}

// 确保 DownloadBinariesStep 实现了 Step 接口
var _ step.Step = (*DownloadBinariesStep)(nil)
