package common

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings" // Added for ToLower and EqualFold

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger" // For logger.Logger
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step" // For step.Step and step.StepContext
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

func (s *DownloadFileStep) verifyChecksum(log logger.Logger, filePath string) error { // Parameter renamed to log to avoid conflict
	if s.Checksum == "" || s.ChecksumType == "" {
		log.Debugf("No checksum provided for %s, skipping verification.", filePath)
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
	log.Infof("Checksum verified successfully for path: %s", filePath)
	return nil
}

func (s *DownloadFileStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	// This step runs on the control node.
	logger := ctx.GetLogger() // Get *logger.Logger

	// For local operations, we don't use runnerSvc.Exists, but os.Stat.
	// This assumes the StepContext is configured for local execution or can provide a local runner.
	// If this step is guaranteed to run on the control node where `kubexm` is executing,
	// direct os calls are appropriate.
	if _, err := os.Stat(s.DestPath); err == nil {
		// File exists, verify checksum if provided
		logger.Infof("Destination file already exists. path: %s", s.DestPath)
		if errVerify := s.verifyChecksum(*logger, s.DestPath); errVerify != nil { // Pass logger directly
			logger.Warnf("Existing file checksum verification failed for path %s, will re-download. error: %v", s.DestPath, errVerify)
			// Attempt to remove the corrupted/wrong file before re-downloading
			if removeErr := os.Remove(s.DestPath); removeErr != nil {
				logger.Errorf("Failed to remove existing file %s with bad checksum: %v", s.DestPath, removeErr)
				// If removal fails, download might also fail or append.
			}
			return false, nil
		}
		logger.Infof("Existing file %s is valid. Download step will be skipped.", s.DestPath)
		return true, nil
	} else if os.IsNotExist(err) {
		logger.Infof("Destination file does not exist, download required. path: %s", s.DestPath)
		return false, nil
	} else { // Other error like permission denied
		logger.Errorf("Failed to stat destination file %s during precheck: %v", s.DestPath, err)
		return false, fmt.Errorf("precheck failed to stat destination file %s: %w", s.DestPath, err)
	}
}

func (s *DownloadFileStep) Run(ctx step.StepContext, host connector.Host) error { // Changed to step.StepContext
	logger := ctx.GetLogger() // Get *logger.Logger

	if s.Sudo {
		// This indicates a design issue if sudo is needed for a download to a work_dir.
		// Downloads should ideally go to paths writable by the kubexm user on the control node.
		logger.Warnf("Sudo is true for DownloadFileStep (step: %s, host: %s). This is unusual for control-node work_dir operations.", s.meta.Name, host.GetName())
	}

	destDir := filepath.Dir(s.DestPath)
	logger.Infof("Ensuring destination directory exists for step %s on host %s. path: %s", s.meta.Name, host.GetName(), destDir)
	if err := os.MkdirAll(destDir, 0755); err != nil { // 0755 standard for dirs
		return fmt.Errorf("failed to create destination directory %s: %w", destDir, err)
	}

	logger.Infof("Starting download for step %s on host %s. url: %s, destination: %s", s.meta.Name, host.GetName(), s.URL, s.DestPath)

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

	if err := s.verifyChecksum(*logger, s.DestPath); err != nil {
		// Attempt to clean up file if checksum fails
		_ = os.Remove(s.DestPath)
		return fmt.Errorf("downloaded file checksum verification failed for %s: %w", s.DestPath, err)
	}

	logger.Infof("File downloaded successfully for step %s on host %s. path: %s", s.meta.Name, host.GetName(), s.DestPath)
	return nil
}

func (s *DownloadFileStep) Rollback(ctx step.StepContext, host connector.Host) error { // Changed to step.StepContext
	logger := ctx.GetLogger() // Get *logger.Logger
	logger.Infof("Attempting to remove downloaded file for step %s on host %s. path: %s", s.meta.Name, host.GetName(), s.DestPath)
	err := os.Remove(s.DestPath)
	if err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove downloaded file %s during rollback for step %s on host %s: %v", s.DestPath, s.meta.Name, host.GetName(), err)
		return fmt.Errorf("failed to remove %s during rollback: %w", s.DestPath, err)
	}
	logger.Infof("Downloaded file %s removed or was not present for step %s on host %s.", s.DestPath, s.meta.Name, host.GetName())
	return nil
}

var _ step.Step = (*DownloadFileStep)(nil)
