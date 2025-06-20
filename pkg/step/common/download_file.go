package common

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For potential checksum or download utils
)

// DownloadFileStepSpec defines the parameters for downloading a file.
type DownloadFileStepSpec struct {
	spec.StepMeta `json:",inline"`

	URL                string `json:"url,omitempty"`
	Destination        string `json:"destination,omitempty"` // Full path to the destination file on the target host
	Checksum           string `json:"checksum,omitempty"`    // e.g., "sha256:abcdef..." or "md5:abcdef..."
	Permissions        string `json:"permissions,omitempty"` // e.g., "0644", if needed after download
	OutputFilePathKey  string `json:"outputFilePathKey,omitempty"` // Optional: Cache key to store the destination path
	Overwrite          bool   `json:"overwrite,omitempty"`         // If true, overwrite if destination exists
}

// NewDownloadFileStepSpec creates a new DownloadFileStepSpec.
func NewDownloadFileStepSpec(name, description, url, destination, checksum, permissions, outputFilePathKey string, overwrite bool) *DownloadFileStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Download %s", filepath.Base(url))
	}
	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Downloads file from %s to %s", url, destination)
		if checksum != "" {
			finalDescription += fmt.Sprintf(" and verifies checksum (%s)", checksum)
		}
	}

	return &DownloadFileStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		URL:                url,
		Destination:        destination,
		Checksum:           checksum,
		Permissions:        permissions,
		OutputFilePathKey:  outputFilePathKey,
		Overwrite:          overwrite,
	}
}

// Name returns the step's name.
func (s *DownloadFileStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *DownloadFileStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *DownloadFileStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *DownloadFileStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *DownloadFileStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *DownloadFileStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Precheck checks if the file already exists and optionally matches the checksum.
func (s *DownloadFileStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.URL == "" || s.Destination == "" {
		logger.Error("URL or Destination is not specified for DownloadFileStep.")
		return false, fmt.Errorf("URL or Destination cannot be empty for DownloadFileStep: %s", s.GetName())
	}

	if s.Overwrite {
		logger.Debug("Overwrite is true, download will proceed regardless of existing file.")
		return false, nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.Destination)
	if err != nil {
		logger.Warn("Failed to check if destination file exists, will attempt download.", "path", s.Destination, "error", err)
		return false, nil
	}

	if exists {
		if s.Checksum == "" {
			logger.Info("Destination file already exists and no checksum provided. Skipping download.", "path", s.Destination)
			return true, nil
		}

		// Checksum verification
		logger.Debug("Destination file exists, verifying checksum.", "path", s.Destination, "checksum", s.Checksum)
		checksumParts := strings.SplitN(s.Checksum, ":", 2)
		if len(checksumParts) != 2 {
			logger.Warn("Invalid checksum format. Expected type:value (e.g., sha256:abc). Will attempt download.", "checksum", s.Checksum)
			return false, nil
		}
		algo, expectedSum := checksumParts[0], checksumParts[1]

		actualSum, err := utils.CalculateRemoteFileChecksum(ctx.GoContext(), conn, s.Destination, algo)
		if err != nil {
			logger.Warn("Failed to calculate checksum for existing file. Will attempt download.", "path", s.Destination, "error", err)
			return false, nil
		}
		if actualSum == expectedSum {
			logger.Info("Destination file exists and checksum matches. Skipping download.", "path", s.Destination)
			return true, nil
		}
		logger.Info("Destination file exists but checksum mismatch. Will attempt re-download.", "path", s.Destination, "expected", expectedSum, "actual", actualSum)
		// Consider removing the file if checksum mismatches and overwrite is intended (though handled by overwrite flag already)
		// For now, just return false to re-download.
	} else {
		logger.Info("Destination file does not exist. Download needed.", "path", s.Destination)
	}
	return false, nil
}

// Run executes the file download.
func (s *DownloadFileStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if s.URL == "" || s.Destination == "" {
		return fmt.Errorf("URL or Destination cannot be empty for DownloadFileStep: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(s.Destination)
	// Assuming sudo might be needed if destDir is privileged.
	// For simplicity, assuming target dirs are writable or handled by connection settings.
	// A more robust EnsureDirectoryStep could precede this if complex permissions are needed for the dir.
	mkdirCmd := fmt.Sprintf("mkdir -p %s", destDir)
	execOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(destDir)} // Basic sudo check
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
	if errMkdir != nil {
		return fmt.Errorf("failed to create destination directory %s (stderr: %s) on host %s: %w", destDir, string(stderrMkdir), host.GetName(), errMkdir)
	}

	// Download the file. Using utils.DownloadFileWithConnector.
	// This utility would typically handle atomic download (e.g. to temp then move).
	// For now, we'll assume it downloads directly or handles overwrite if the utility supports it.
	// If not, we might need to remove s.Destination first if s.Overwrite is true.
	if s.Overwrite {
		exists, _ := conn.Exists(ctx.GoContext(), s.Destination)
		if exists {
			logger.Debug("Overwrite is true, removing existing destination file before download.", "path", s.Destination)
			rmOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(s.Destination)}
			if _, _, rmErr := conn.Exec(ctx.GoContext(), fmt.Sprintf("rm -f %s", s.Destination), rmOpts); rmErr != nil {
				logger.Warn("Failed to remove existing file for overwrite, download might fail or behave unexpectedly.", "path", s.Destination, "error", rmErr)
			}
		}
	}

	logger.Info("Downloading file.", "url", s.URL, "destination", s.Destination)
	// Assuming a utility function that uses the connector to download.
	// If checksum is provided, this utility should verify it.
	// For this example, let's assume a simple download and then separate checksum/permissions.
	// A real DownloadFile utility would integrate this better.

	// Simplified download using curl via Exec. A robust solution would use conn.Download or a more featured utility.
	// This example doesn't handle checksum during download.
	// The -L handles redirects. -f makes curl fail silently on server errors. -s for silent. -o for output.
	// Need to ensure curl is available.
	curlCmd := fmt.Sprintf("curl -sfL -o %s %s", s.Destination, s.URL)
	// Sudo for curl if destination requires it (less common for curl output directly, but for consistency)
	curlOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(s.Destination)}
	_, stderrCurl, errCurl := conn.Exec(ctx.GoContext(), curlCmd, curlOpts)
	if errCurl != nil {
		return fmt.Errorf("failed to download file from %s to %s using curl (stderr: %s): %w", s.URL, s.Destination, string(stderrCurl), errCurl)
	}

	// Verify checksum if provided, after download
	if s.Checksum != "" {
		checksumParts := strings.SplitN(s.Checksum, ":", 2)
		if len(checksumParts) == 2 {
			algo, expectedSum := checksumParts[0], checksumParts[1]
			actualSum, err := utils.CalculateRemoteFileChecksum(ctx.GoContext(), conn, s.Destination, algo)
			if err != nil {
				return fmt.Errorf("failed to calculate checksum for downloaded file %s: %w", s.Destination, err)
			}
			if actualSum != expectedSum {
				// Attempt to remove the invalid file
				_ = conn.Remove(ctx.GoContext(), s.Destination, connector.RemoveOptions{})
				return fmt.Errorf("checksum mismatch for downloaded file %s: expected %s, got %s (%s)", s.Destination, expectedSum, actualSum, algo)
			}
			logger.Info("Checksum verified for downloaded file.", "path", s.Destination)
		} else {
			logger.Warn("Invalid checksum format, skipping verification.", "checksum", s.Checksum)
		}
	}

	// Set permissions if specified
	if s.Permissions != "" {
		chmodCmd := fmt.Sprintf("chmod %s %s", s.Permissions, s.Destination)
		chmodOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(s.Destination)}
		_, stderrChmod, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, chmodOpts)
		if errChmod != nil {
			return fmt.Errorf("failed to set permissions %s on %s (stderr: %s): %w", s.Permissions, s.Destination, string(stderrChmod), errChmod)
		}
		logger.Info("Permissions set on downloaded file.", "path", s.Destination, "permissions", s.Permissions)
	}

	if s.OutputFilePathKey != "" {
		ctx.StepCache().Set(s.OutputFilePathKey, s.Destination)
		logger.Debug("Stored downloaded file path in cache.", "key", s.OutputFilePathKey, "path", s.Destination)
	}

	logger.Info("File downloaded successfully.", "destination", s.Destination)
	return nil
}

// Rollback removes the downloaded file.
func (s *DownloadFileStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")

	if s.Destination == "" {
		logger.Info("Destination path is empty, nothing to roll back.")
		return nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	logger.Info("Attempting to remove downloaded file.", "path", s.Destination)
	// Use Exec for rm -f with potential sudo
	rmCmd := fmt.Sprintf("rm -f %s", s.Destination)
	rmOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(s.Destination)}
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, rmOpts)

	if errRm != nil {
		logger.Error("Failed to remove downloaded file during rollback (best effort).", "path", s.Destination, "stderr", string(stderrRm), "error", errRm)
		// Do not return error for rollback, as it's best-effort
	} else {
		logger.Info("Downloaded file removed successfully.", "path", s.Destination)
	}

	if s.OutputFilePathKey != "" {
		ctx.StepCache().Delete(s.OutputFilePathKey)
		logger.Debug("Removed downloaded file path from cache.", "key", s.OutputFilePathKey)
	}
	return nil
}

var _ step.Step = (*DownloadFileStepSpec)(nil)
