package binary

import (
	"crypto/sha256"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/logger"
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

type DownloadBinariesStep struct {
	step.Base
	Concurrency int
}

type DownloadBinariesStepBuilder struct {
	step.Builder[DownloadBinariesStepBuilder, *DownloadBinariesStep]
}

func NewDownloadBinariesStepBuilder(ctx runtime.Context, instanceName string) *DownloadBinariesStepBuilder {
	s := &DownloadBinariesStep{
		Concurrency: 5,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Download all required binaries to local directories", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 60 * time.Minute

	b := new(DownloadBinariesStepBuilder).Init(s)
	return b
}

func (b *DownloadBinariesStepBuilder) WithConcurrency(c int) *DownloadBinariesStepBuilder {
	if c > 0 {
		b.Step.Concurrency = c
	}
	return b
}

func (s *DownloadBinariesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

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

func (s *DownloadBinariesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("DownloadBinariesStep will always run to ensure binaries are up-to-date.")
	return false, nil
}

func (s *DownloadBinariesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

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

	jobs := make(chan *binary.Binary, len(binariesToDownload))
	errChan := make(chan error, len(binariesToDownload))

	var wg sync.WaitGroup
	for i := 0; i < s.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerLogger := logger.With("worker", workerID)
			for b := range jobs {
				workerLogger.Info("Starting download.", "binary", b.FileName(), "arch", b.Arch)
				err := s.downloadAndVerify(workerLogger, b)
				if err != nil {
					workerLogger.Error(err, "Failed to download.", "binary", b.FileName())
					errChan <- fmt.Errorf("failed to download %s for %s: %w", b.FileName(), b.Arch, err)
				} else {
					workerLogger.Info("Successfully downloaded.", "binary", b.FileName())
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

func (s *DownloadBinariesStep) downloadAndVerify(logger *logger.Logger, b *binary.Binary) error {
	destPath := b.FilePath()

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(destPath), err)
	}

	if _, err := os.Stat(destPath); err == nil {
		if b.Checksum() == "" || b.Checksum() == "dummy-etcd-checksum-val" {
			logger.Debug("File already exists, skipping download as no checksum is provided.", "file", b.FileName())
			return nil
		}

		match, err := verifyChecksum(destPath, b.Checksum())
		if err != nil {
			return fmt.Errorf("failed to verify checksum for existing file %s: %w", destPath, err)
		}
		if match {
			logger.Debug("File already exists and checksum matches. Skipping download.", "file", b.FileName())
			return nil
		}
		logger.Warn("File exists but checksum mismatches. Re-downloading.", "file", b.FileName())
	}

	url := b.URL()
	logger.Debug("Downloading from URL.", "url", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status for %s: %s", url, resp.Status)
	}

	tmpFile, err := os.Create(destPath + ".tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return fmt.Errorf("failed to copy response body to file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		os.Remove(tmpFile.Name())
		return fmt.Errorf("failed to sync temp file to disk: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), destPath); err != nil {
		return fmt.Errorf("failed to rename temp file to final destination: %w", err)
	}

	if b.Checksum() != "" && b.Checksum() != "dummy-etcd-checksum-val" {
		logger.Debug("Verifying checksum of downloaded file.", "file", b.FileName())
		match, err := verifyChecksum(destPath, b.Checksum())
		if err != nil {
			return fmt.Errorf("failed to verify checksum for downloaded file %s: %w", destPath, err)
		}
		if !match {
			return fmt.Errorf("checksum mismatch for downloaded file %s", destPath)
		}
		logger.Debug("Checksum verified successfully.", "file", b.FileName())
	}

	return nil
}

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

func (s *DownloadBinariesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	workDir := ctx.GetGlobalWorkDir()
	clusterName := ctx.GetClusterConfig().Name
	binariesDir := filepath.Join(workDir, "kubexm", clusterName)
	logger.Warn("Rollback is triggered. You might want to manually delete the binaries directory.", "path", binariesDir)
	return nil
}

var _ step.Step = (*DownloadBinariesStep)(nil)
