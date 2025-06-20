package etcd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils"
)

const (
	// DefaultEtcdInstallURLBase is the default base URL for etcd downloads.
	DefaultEtcdInstallURLBase = "https://github.com/etcd-io/etcd/releases/download"
	// DefaultEtcdVersion is the version used if not specified.
	DefaultEtcdVersion = "v3.5.9" // Example default
	// EtcdDownloadedArchiveKey is the key used to store the downloaded etcd archive path in shared data.
	EtcdDownloadedArchiveKey = "EtcdDownloadedArchivePath"
)

// DownloadEtcdStepSpec defines the parameters for downloading an etcd release archive.
type DownloadEtcdStepSpec struct {
	spec.StepMeta // Embed common meta fields like Name, Description

	Version        string `json:"version,omitempty"`        // e.g., "v3.5.9"
	Arch           string `json:"arch,omitempty"`           // e.g., "amd64", "arm64"
	InstallURLBase string `json:"installURLBase,omitempty"` // Base URL for downloads
	DownloadDir    string `json:"downloadDir,omitempty"`    // Directory on the host to download the archive to
	Checksum       string `json:"checksum,omitempty"`       // Expected checksum of the archive (e.g., "sha256:...")
	OutputKey      string `json:"outputKey,omitempty"`      // Key to store the downloaded file path in shared data
}

// NewDownloadEtcdStepSpec creates a new DownloadEtcdStepSpec.
func NewDownloadEtcdStepSpec(stepName, version, arch, installURLBase, downloadDir, checksum, outputKey string) *DownloadEtcdStepSpec {
	if stepName == "" {
		stepName = "Download Etcd Archive"
	}
	normalizedVersion := version
	if normalizedVersion == "" {
		normalizedVersion = DefaultEtcdVersion
	}
	if !strings.HasPrefix(normalizedVersion, "v") {
		normalizedVersion = "v" + normalizedVersion
	}

	urlBase := installURLBase
	if urlBase == "" {
		urlBase = DefaultEtcdInstallURLBase
	}

	// Arch default is handled by executor/runner based on host info if empty

	outKey := outputKey
	if outKey == "" {
		outKey = EtcdDownloadedArchiveKey
	}

	desc := fmt.Sprintf("Downloads etcd version %s for %s architecture.", normalizedVersion, arch)
	if checksum != "" {
		desc += fmt.Sprintf(" Verifies checksum %s.", checksum)
	}


	return &DownloadEtcdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        stepName,
			Description: desc,
		},
		Version:        normalizedVersion,
		Arch:           arch, // Let executor determine if empty
		InstallURLBase: urlBase,
		DownloadDir:    downloadDir, // If empty, executor might use a default work dir
		Checksum:       checksum,
		OutputKey:      outKey,
	}
}

// GetName returns the step's name.
func (s *DownloadEtcdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
func (s *DownloadEtcdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec validates and returns the spec.
// This is a placeholder for potential future validation logic.
func (s *DownloadEtcdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *DownloadEtcdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Name returns the step's name (implementing step.Step).
func (s *DownloadEtcdStepSpec) Name() string { return s.GetName() }

// Description returns the step's description (implementing step.Step).
func (s *DownloadEtcdStepSpec) Description() string { return s.GetDescription() }

// Precheck checks if the etcd archive already exists and matches checksum.
func (s *DownloadEtcdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.Version == "" || s.Arch == "" || s.DownloadDir == "" {
		errMsg := "Version, Arch, and DownloadDir must be specified"
		logger.Error(errMsg)
		return false, fmt.Errorf(errMsg+" for step: %s", s.GetName())
	}

	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", s.Version, s.Arch)
	destinationFilePath := filepath.Join(s.DownloadDir, archiveName)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), destinationFilePath)
	if err != nil {
		logger.Warn("Failed to check if destination file exists, will attempt download.", "path", destinationFilePath, "error", err)
		return false, nil
	}

	if exists {
		if s.Checksum == "" {
			logger.Info("Destination file already exists and no checksum provided. Skipping download.", "path", destinationFilePath)
			return true, nil
		}
		logger.Debug("Destination file exists, verifying checksum.", "path", destinationFilePath, "checksum", s.Checksum)
		checksumParts := strings.SplitN(s.Checksum, ":", 2)
		if len(checksumParts) != 2 {
			logger.Warn("Invalid checksum format. Will attempt download.", "checksum", s.Checksum)
			return false, nil
		}
		algo, expectedSum := checksumParts[0], checksumParts[1]
		actualSum, err := utils.CalculateRemoteFileChecksum(ctx.GoContext(), conn, destinationFilePath, algo)
		if err != nil {
			logger.Warn("Failed to calculate checksum for existing file. Will attempt download.", "path", destinationFilePath, "error", err)
			return false, nil
		}
		if actualSum == expectedSum {
			logger.Info("Destination file exists and checksum matches. Skipping download.", "path", destinationFilePath)
			// Cache the path if an output key is specified, as Run would do.
			if s.OutputKey != "" {
				ctx.StepCache().Set(s.OutputKey, destinationFilePath)
			}
			return true, nil
		}
		logger.Info("Destination file exists but checksum mismatch. Will re-download.", "path", destinationFilePath)
	} else {
		logger.Info("Destination file does not exist. Download needed.", "path", destinationFilePath)
	}
	return false, nil
}

// Run executes the etcd archive download.
func (s *DownloadEtcdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if s.Version == "" || s.Arch == "" || s.DownloadDir == "" || s.InstallURLBase == "" {
		return fmt.Errorf("Version, Arch, DownloadDir, and InstallURLBase must be specified for step: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Ensure download directory exists
	mkdirCmd := fmt.Sprintf("mkdir -p %s", s.DownloadDir)
	execOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(s.DownloadDir)}
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
	if errMkdir != nil {
		return fmt.Errorf("failed to create download directory %s (stderr: %s) on host %s: %w", s.DownloadDir, string(stderrMkdir), host.GetName(), errMkdir)
	}

	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", s.Version, s.Arch)
	downloadURL := fmt.Sprintf("%s/%s/%s", s.InstallURLBase, s.Version, archiveName)
	destinationFilePath := filepath.Join(s.DownloadDir, archiveName)

	logger.Info("Downloading etcd archive.", "url", downloadURL, "destination", destinationFilePath)

	// Simplified download using curl.
	curlCmd := fmt.Sprintf("curl -sfL -o %s %s", destinationFilePath, downloadURL)
	curlOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(destinationFilePath)}
	_, stderrCurl, errCurl := conn.Exec(ctx.GoContext(), curlCmd, curlOpts)
	if errCurl != nil {
		return fmt.Errorf("failed to download file from %s to %s using curl (stderr: %s): %w", downloadURL, destinationFilePath, string(stderrCurl), errCurl)
	}

	if s.Checksum != "" {
		checksumParts := strings.SplitN(s.Checksum, ":", 2)
		if len(checksumParts) == 2 {
			algo, expectedSum := checksumParts[0], checksumParts[1]
			actualSum, err := utils.CalculateRemoteFileChecksum(ctx.GoContext(), conn, destinationFilePath, algo)
			if err != nil {
				return fmt.Errorf("failed to calculate checksum for downloaded file %s: %w", destinationFilePath, err)
			}
			if actualSum != expectedSum {
				_ = conn.Remove(ctx.GoContext(), destinationFilePath, connector.RemoveOptions{})
				return fmt.Errorf("checksum mismatch for downloaded file %s: expected %s, got %s (%s)", destinationFilePath, expectedSum, actualSum, algo)
			}
			logger.Info("Checksum verified for downloaded etcd archive.", "path", destinationFilePath)
		} else {
			logger.Warn("Invalid checksum format, skipping verification.", "checksum", s.Checksum)
		}
	}

	if s.OutputKey != "" {
		ctx.StepCache().Set(s.OutputKey, destinationFilePath)
		logger.Debug("Stored downloaded etcd archive path in cache.", "key", s.OutputKey, "path", destinationFilePath)
	}

	logger.Info("Etcd archive downloaded successfully.", "destination", destinationFilePath)
	return nil
}

// Rollback removes the downloaded etcd archive.
func (s *DownloadEtcdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")

	if s.Version == "" || s.Arch == "" || s.DownloadDir == "" {
		logger.Info("Version, Arch, or DownloadDir is empty, cannot determine file to roll back.")
		return nil
	}
	archiveName := fmt.Sprintf("etcd-%s-linux-%s.tar.gz", s.Version, s.Arch)
	destinationFilePath := filepath.Join(s.DownloadDir, archiveName)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	logger.Info("Attempting to remove downloaded etcd archive.", "path", destinationFilePath)
	rmCmd := fmt.Sprintf("rm -f %s", destinationFilePath)
	rmOpts := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(destinationFilePath)}
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, rmOpts)

	if errRm != nil {
		logger.Error("Failed to remove downloaded etcd archive during rollback (best effort).", "path", destinationFilePath, "stderr", string(stderrRm), "error", errRm)
	} else {
		logger.Info("Downloaded etcd archive removed successfully.", "path", destinationFilePath)
	}

	if s.OutputKey != "" {
		ctx.StepCache().Delete(s.OutputKey)
		logger.Debug("Removed downloaded etcd archive path from cache.", "key", s.OutputKey)
	}
	return nil
}

var _ step.Step = (*DownloadEtcdStepSpec)(nil)
