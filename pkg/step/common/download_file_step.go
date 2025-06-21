package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"strings" // Added for ToLower and EqualFold

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// DownloadFileStep downloads a file from a URL to a local path on the target host (usually control node).
type DownloadFileStep struct {
	meta         spec.StepMeta
	URL          string
	DestPath     string
	Checksum     string // Expected checksum (e.g., SHA256)
	ChecksumType string // e.g., "sha256"
	Sudo         bool   // Should generally be false if DestPath is within a user-writable work_dir
}

// NewDownloadFileStep creates a new DownloadFileStep.
// instanceName is optional; if empty, a default name will be generated.
func NewDownloadFileStep(instanceName, url, destPath, checksum, checksumType string, sudo bool) step.Step {
	metaName := instanceName
	if metaName == "" {
		metaName = fmt.Sprintf("DownloadFileTo-%s", filepath.Base(destPath))
	}
	// Default checksumType to sha256 if a checksum is provided but type is empty
	effectiveChecksumType := checksumType
	if checksum != "" && effectiveChecksumType == "" {
		effectiveChecksumType = "sha256"
	}

	return &DownloadFileStep{
		meta: spec.StepMeta{
			Name:        metaName,
			Description: fmt.Sprintf("Downloads file from %s to %s", url, destPath),
		},
		URL:          url,
		DestPath:     destPath,
		Checksum:     checksum,
		ChecksumType: effectiveChecksumType,
		Sudo:         sudo,
	}
}

func (s *DownloadFileStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DownloadFileStep) verifyChecksum(logger runtime.Logger, filePath string) error {
	if s.Checksum == "" || s.ChecksumType == "" {
		logger.Debug("No checksum provided, skipping verification.", "path", filePath)
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
	logger.Info("Checksum verified successfully.", "path", filePath)
	return nil
}

func (s *DownloadFileStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	// This step runs on the control node, so host connector is LocalConnector.
	// Runner methods for local operations should directly use os package.
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	// For local operations, we don't use runnerSvc.Exists, but os.Stat.
	// This assumes the StepContext is configured for local execution or can provide a local runner.
	// If this step is guaranteed to run on the control node where `kubexm` is executing,
	// direct os calls are appropriate.
	if _, err := os.Stat(s.DestPath); err == nil {
		// File exists, verify checksum if provided
		logger.Info("Destination file already exists.", "path", s.DestPath)
		if err := s.verifyChecksum(logger, s.DestPath); err != nil {
			logger.Warn("Existing file checksum verification failed, will re-download.", "path", s.DestPath, "error", err)
			// Attempt to remove the corrupted/wrong file before re-downloading
			if removeErr := os.Remove(s.DestPath); removeErr != nil {
				logger.Error(removeErr, "Failed to remove existing file with bad checksum.", "path", s.DestPath)
				// If removal fails, download might also fail or append.
			}
			return false, nil
		}
		logger.Info("Existing file is valid. Download step will be skipped.")
		return true, nil
	} else if os.IsNotExist(err) {
		logger.Info("Destination file does not exist, download required.", "path", s.DestPath)
		return false, nil
	} else { // Other error like permission denied
		logger.Error(err, "Failed to stat destination file during precheck.", "path", s.DestPath)
		return false, fmt.Errorf("precheck failed to stat destination file %s: %w", s.DestPath, err)
	}
}

func (s *DownloadFileStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	if s.Sudo {
		// This indicates a design issue if sudo is needed for a download to a work_dir.
		// Downloads should ideally go to paths writable by the kubexm user on the control node.
		logger.Warn("Sudo is true for DownloadFileStep. This is unusual for control-node work_dir operations.")
	}

	destDir := filepath.Dir(s.DestPath)
	logger.Info("Ensuring destination directory exists.", "path", destDir)
	if err := os.MkdirAll(destDir, 0755); err != nil { // 0755 standard for dirs
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	logger.Info("Starting download.", "url", s.URL, "destination", s.DestPath)

	req, err := http.NewRequestWithContext(ctx.GoContext(), http.MethodGet, s.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request for %s: %w", s.URL, err)
	}

	httpClient := &http.Client{} // Use default client or one from runtime context if available
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start download from %s: %w", s.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download request failed for %s: status %s", s.URL, resp.Status)
	}

	out, err := os.Create(s.DestPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", s.DestPath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		// Attempt to clean up partially downloaded file
		_ = os.Remove(s.DestPath)
		return fmt.Errorf("failed to write download content to %s: %w", s.DestPath, err)
	}
	// Explicitly close before checksum to flush buffers.
	if errClose := out.Close(); errClose != nil {
		// Attempt to clean up partially written file
		_ = os.Remove(s.DestPath)
		return fmt.Errorf("failed to close destination file %s after writing: %w", s.DestPath, errClose)
	}

	if err := s.verifyChecksum(logger, s.DestPath); err != nil {
		// Attempt to clean up file if checksum fails
		_ = os.Remove(s.DestPath)
		return fmt.Errorf("downloaded file checksum verification failed for %s: %w", s.DestPath, err)
	}

	logger.Info("File downloaded successfully.", "path", s.DestPath)
	return nil
}

func (s *DownloadFileStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove downloaded file.", "path", s.DestPath)
	err := os.Remove(s.DestPath)
	if err != nil && !os.IsNotExist(err) {
		logger.Error(err, "Failed to remove downloaded file during rollback.", "path", s.DestPath)
		return fmt.Errorf("failed to remove %s during rollback: %w", s.DestPath, err)
	}
	logger.Info("Downloaded file removed or was not present.", "path", s.DestPath)
	return nil
}

var _ step.Step = (*DownloadFileStep)(nil)

[end of pkg/step/common/download_file_step.go]
