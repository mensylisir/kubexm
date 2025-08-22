package common

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/schollz/progressbar/v3"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DownloadFileStep struct {
	step.Base
	URL          string
	DestPath     string
	Checksum     string
	ChecksumType string
}

type DownloadFileStepBuilder struct {
	step.Builder[DownloadFileStepBuilder, *DownloadFileStep]
}

func NewDownloadFileStepBuilder(ctx runtime.ExecutionContext, instanceName, url, destPath string) *DownloadFileStepBuilder {
	cs := &DownloadFileStep{
		URL:      url,
		DestPath: destPath,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Download %s to [%s]", instanceName, url, destPath)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(DownloadFileStepBuilder).Init(cs)
}

func (b *DownloadFileStepBuilder) WithChecksum(checksub string) *DownloadFileStepBuilder {
	b.Step.Checksum = checksub
	return b
}

func (b *DownloadFileStepBuilder) WithChecksumType(checksumType string) *DownloadFileStepBuilder {
	b.Step.ChecksumType = checksumType
	return b
}

func (s *DownloadFileStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DownloadFileStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	if _, err := os.Stat(s.DestPath); err == nil {
		logger.Info("Destination file already exists.", "path", s.DestPath)
		if errVerify := s.verifyChecksum(s.DestPath); errVerify != nil {
			logger.Warnf("Existing file checksum verification failed for path %s, will re-download. error: %v", s.DestPath, errVerify)
			if removeErr := os.Remove(s.DestPath); removeErr != nil {
				logger.Errorf("Failed to remove existing file %s with bad checksum: %v", s.DestPath, removeErr)
			}
			return false, nil
		}
		logger.Infof("Existing file %s is valid. Download step will be skipped.", s.DestPath)
		return true, nil
	} else if os.IsNotExist(err) {
		logger.Infof("Destination file does not exist, download required. path: %s", s.DestPath)
		return false, nil
	} else {
		logger.Errorf("Failed to stat destination file %s during precheck: %v", s.DestPath, err)
		return false, fmt.Errorf("precheck failed to stat destination file %s: %w", s.DestPath, err)
	}
}

func (s *DownloadFileStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	if s.Sudo {
		logger.Warn("Sudo is true for this step. This is unusual for control-node work_dir operations.")
	}

	destDir := filepath.Dir(s.DestPath)
	logger.Info("Ensuring destination directory exists.", "path", destDir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	logger.Info("Starting download.", "url", s.URL, "destination", s.DestPath)

	req, err := http.NewRequestWithContext(ctx.GoContext(), http.MethodGet, s.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request for %s: %w", s.URL, err)
	}
	resp, err := ctx.GetHttpClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to start download from %s: %w", s.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download request failed for %s: status %s", s.URL, resp.Status)
	}
	tmpPath := s.DestPath + ".part"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", tmpPath, err)
	}
	defer out.Close()
	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s", s.URL)),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
	)
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write download content to %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, s.DestPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file to destination: %w", err)
	}
	if err := s.verifyChecksum(s.DestPath); err != nil {
		_ = os.Remove(s.DestPath)
		return fmt.Errorf("downloaded file checksum verification failed for %s: %w", s.DestPath, err)
	}
	if outputKeyTmplVal, ok := ctx.GetFromRuntimeConfig("outputCacheKeyTemplate"); ok {
		if outputKeyTmpl, isString := outputKeyTmplVal.(string); isString && outputKeyTmpl != "" {
			cacheKey := fmt.Sprintf(outputKeyTmpl, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
			ctx.GetTaskCache().Set(cacheKey, s.DestPath)
			logger.Info("Stored downloaded path in cache", "key", cacheKey)
		} else {
			err := fmt.Errorf("invalid 'outputCacheKeyTemplate' in RuntimeConfig: expected a non-empty string, but got type %T", outputKeyTmplVal)
			logger.Error(err, "Configuration error for step output.")
			return err
		}
	}
	logger.Info("File downloaded successfully.", "path", s.DestPath)
	return nil
}

func (s *DownloadFileStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove downloaded file.", "path", s.DestPath)
	err := os.Remove(s.DestPath)
	if err != nil && !os.IsNotExist(err) {
		logger.Error(err, "Failed to remove downloaded file during rollback.", "path", s.DestPath)
		return fmt.Errorf("failed to remove %s during rollback: %w", s.DestPath, err)
	}
	logger.Info("Downloaded file removed or was not present.", "path", s.DestPath)
	return nil
}

func (s *DownloadFileStep) verifyChecksum(filePath string) error {
	if s.Checksum == "" || s.ChecksumType == "" {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s for checksum: %w", filePath, err)
	}
	defer file.Close()

	var h hash.Hash
	switch strings.ToLower(s.ChecksumType) {
	case "sha256":
		h = sha256.New()
	default:
		return fmt.Errorf("unsupported checksum type: %s for file %s", s.ChecksumType, filePath)
	}

	if _, err := io.Copy(h, file); err != nil {
		return fmt.Errorf("failed to read file %s for checksum: %w", filePath, err)
	}

	calculatedChecksum := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(calculatedChecksum, s.Checksum) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filePath, s.Checksum, calculatedChecksum)
	}
	return nil
}

var _ step.Step = (*DownloadFileStep)(nil)
