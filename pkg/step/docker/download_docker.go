package docker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo and CalculateRemoteFileChecksum
)

// DownloadDockerStepSpec defines parameters for downloading Docker static binaries archive.
type DownloadDockerStepSpec struct {
	spec.StepMeta `json:",inline"`

	Version             string `json:"version,omitempty"` // e.g., "20.10.17"
	Arch                string `json:"arch,omitempty"`    // e.g., "amd64", "arm64" (will be mapped to Docker's x86_64, aarch64)
	DownloadURLBase     string `json:"downloadURLBase,omitempty"`
	TargetDir           string `json:"targetDir,omitempty"`
	TargetFilename      string `json:"targetFilename,omitempty"`
	Checksum            string `json:"checksum,omitempty"`      // Format: "sha256:<value>"
	ChecksumURL         string `json:"checksumURL,omitempty"` // Not implemented in this version, but placeholder
	OutputArchiveCacheKey string `json:"outputArchiveCacheKey,omitempty"` // Required
	Sudo                bool   `json:"sudo,omitempty"`                  // For mkdir if TargetDir needs sudo
}

// NewDownloadDockerStepSpec creates a new DownloadDockerStepSpec.
func NewDownloadDockerStepSpec(name, description, version, arch, outputArchiveCacheKey string) *DownloadDockerStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Download Docker %s (%s)", version, arch)
	}
	finalDescription := description
	// Description refined in populateDefaults

	if outputArchiveCacheKey == "" {
		// Consider this a fatal error for construction or handle in validation.
		// For now, allow creation and let Run/Precheck fail.
	}

	return &DownloadDockerStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Version:             version,
		Arch:                arch,
		OutputArchiveCacheKey: outputArchiveCacheKey,
	}
}

// Name returns the step's name.
func (s *DownloadDockerStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *DownloadDockerStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *DownloadDockerStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *DownloadDockerStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *DownloadDockerStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *DownloadDockerStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// mapArchToDockerArch maps common arch names to Docker's naming convention.
func (s *DownloadDockerStepSpec) mapArchToDockerArch() string {
	switch strings.ToLower(s.Arch) {
	case "amd64", "x86_64":
		return "x86_64"
	case "arm64", "aarch64":
		return "aarch64"
	case "armhf", "armv7l": // Common for 32-bit ARM
		return "armhf"
	case "s390x":
		return "s390x"
	default:
		return s.Arch // Return as is if not a common mapping we handle
	}
}

func (s *DownloadDockerStepSpec) populateDefaults(logger runtime.Logger, hostArch string) {
	if s.Arch == "" {
		s.Arch = hostArch // Use host's architecture if not specified
		logger.Debug("Arch defaulted to host architecture.", "arch", s.Arch)
	}

	if s.DownloadURLBase == "" {
		s.DownloadURLBase = "https://download.docker.com/linux/static/stable/"
		logger.Debug("DownloadURLBase defaulted.", "url", s.DownloadURLBase)
	}
	if s.TargetDir == "" {
		s.TargetDir = "/tmp/kubexms_downloads" // Consider using ctx.GetWorkDir() if available
		logger.Debug("TargetDir defaulted.", "dir", s.TargetDir)
	}
	if s.TargetFilename == "" {
		// Docker typically uses docker-<version>.tgz, e.g., docker-20.10.17.tgz
		s.TargetFilename = fmt.Sprintf("docker-%s.tgz", s.Version)
		logger.Debug("TargetFilename defaulted.", "filename", s.TargetFilename)
	}

	if s.StepMeta.Description == "" {
		dockerArch := s.mapArchToDockerArch()
		fullURL := strings.TrimSuffix(s.DownloadURLBase, "/") + "/" + dockerArch + "/" + s.TargetFilename
		s.StepMeta.Description = fmt.Sprintf("Downloads Docker CE static archive version %s for %s from %s to %s/%s.",
			s.Version, dockerArch, s.DownloadURLBase, s.TargetDir, s.TargetFilename)
		if s.Checksum != "" {
			s.StepMeta.Description += fmt.Sprintf(" Verifies checksum %s.", s.Checksum)
		}
	}
}

// Precheck determines if the Docker archive seems to be already downloaded and valid.
func (s *DownloadDockerStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger, host.GetArch()) // Pass host.GetArch() for default

	if s.Version == "" || s.OutputArchiveCacheKey == "" {
		return false, fmt.Errorf("Version and OutputArchiveCacheKey must be specified for %s", s.GetName())
	}

	fullTargetPath := filepath.Join(s.TargetDir, s.TargetFilename)
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), fullTargetPath)
	if err != nil {
		logger.Warn("Failed to check if target file exists, will attempt download.", "path", fullTargetPath, "error", err)
		return false, nil // Let Run attempt.
	}

	if !exists {
		logger.Info("Target Docker archive does not exist. Download needed.", "path", fullTargetPath)
		return false, nil
	}

	logger.Info("Target Docker archive exists.", "path", fullTargetPath)
	if s.Checksum == "" {
		logger.Info("No checksum provided. Assuming existing file is correct and skipping download.")
		// Cache the path if an output key is specified, as Run would do.
		ctx.StepCache().Set(s.OutputArchiveCacheKey, fullTargetPath)
		return true, nil
	}

	logger.Debug("Verifying checksum of existing file.", "checksum", s.Checksum)
	checksumParts := strings.SplitN(s.Checksum, ":", 2)
	if len(checksumParts) != 2 {
		logger.Warn("Invalid checksum format. Expected type:value (e.g., sha256:abc). Will attempt download.", "checksum", s.Checksum)
		return false, nil
	}
	algo, expectedSum := checksumParts[0], checksumParts[1]

	actualSum, err := utils.CalculateRemoteFileChecksum(ctx.GoContext(), conn, fullTargetPath, algo)
	if err != nil {
		logger.Warn("Failed to calculate checksum for existing file. Will attempt download.", "path", fullTargetPath, "error", err)
		return false, nil
	}
	if actualSum == expectedSum {
		logger.Info("Existing file checksum matches. Skipping download.", "path", fullTargetPath)
		ctx.StepCache().Set(s.OutputArchiveCacheKey, fullTargetPath)
		return true, nil
	}

	logger.Info("Existing file checksum mismatch. Will re-download.", "path", fullTargetPath, "expected", expectedSum, "actual", actualSum)
	return false, nil
}

// Run executes the Docker archive download.
func (s *DownloadDockerStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger, host.GetArch()) // Pass host.GetArch() for default

	if s.Version == "" || s.OutputArchiveCacheKey == "" {
		return fmt.Errorf("Version and OutputArchiveCacheKey must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Ensure target directory exists
	// Sudo for mkdir based on path and spec's Sudo field
	mkdirSudo := s.Sudo || utils.PathRequiresSudo(s.TargetDir)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", s.TargetDir)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, &connector.ExecOptions{Sudo: mkdirSudo})
	if errMkdir != nil {
		return fmt.Errorf("failed to create target directory %s (stderr: %s) on host %s: %w", s.TargetDir, string(stderrMkdir), host.GetName(), errMkdir)
	}

	dockerArch := s.mapArchToDockerArch()
	downloadURL := strings.TrimSuffix(s.DownloadURLBase, "/") + "/" + dockerArch + "/" + s.TargetFilename
	fullTargetPath := filepath.Join(s.TargetDir, s.TargetFilename)

	logger.Info("Downloading Docker archive.", "url", downloadURL, "destination", fullTargetPath)

	// Use a generic download utility logic (e.g., curl)
	// Sudo for curl output if destination requires it. Usually not for /tmp.
	// The Sudo field in spec is for dir creation or if the download tool itself needs it.
	curlSudo := s.Sudo || utils.PathRequiresSudo(fullTargetPath)
	curlCmd := fmt.Sprintf("curl -sfL -o %s %s", fullTargetPath, downloadURL)
	_, stderrCurl, errCurl := conn.Exec(ctx.GoContext(), curlCmd, &connector.ExecOptions{Sudo: curlSudo})
	if errCurl != nil {
		return fmt.Errorf("failed to download Docker archive from %s to %s (stderr: %s): %w", downloadURL, fullTargetPath, string(stderrCurl), errCurl)
	}

	if s.Checksum != "" {
		checksumParts := strings.SplitN(s.Checksum, ":", 2)
		if len(checksumParts) == 2 {
			algo, expectedSum := checksumParts[0], checksumParts[1]
			logger.Debug("Verifying checksum of downloaded file.", "algorithm", algo)
			actualSum, err := utils.CalculateRemoteFileChecksum(ctx.GoContext(), conn, fullTargetPath, algo)
			if err != nil {
				return fmt.Errorf("failed to calculate checksum for downloaded file %s: %w", fullTargetPath, err)
			}
			if actualSum != expectedSum {
				_ = conn.Remove(ctx.GoContext(), fullTargetPath, connector.RemoveOptions{Sudo: curlSudo}) // Attempt cleanup
				return fmt.Errorf("checksum mismatch for downloaded file %s: expected %s (%s), got %s", fullTargetPath, expectedSum, algo, actualSum)
			}
			logger.Info("Checksum verified for downloaded Docker archive.", "path", fullTargetPath)
		} else {
			return fmt.Errorf("invalid checksum format provided: %s. Expected type:value", s.Checksum)
		}
	}

	ctx.StepCache().Set(s.OutputArchiveCacheKey, fullTargetPath)
	logger.Info("Docker archive downloaded successfully and path cached.", "key", s.OutputArchiveCacheKey, "path", fullTargetPath)
	return nil
}

// Rollback removes the downloaded Docker archive.
func (s *DownloadDockerStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger, host.GetArch()) // Ensure paths are populated

	if s.TargetDir == "" || s.TargetFilename == "" {
		logger.Info("TargetDir or TargetFilename is empty, cannot determine file to roll back.")
		return nil
	}
	fullTargetPath := filepath.Join(s.TargetDir, s.TargetFilename)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	logger.Info("Attempting to remove downloaded Docker archive.", "path", fullTargetPath)
	// Sudo for rm based on path and spec's Sudo field
	rmSudo := s.Sudo || utils.PathRequiresSudo(fullTargetPath)
	rmCmd := fmt.Sprintf("rm -f %s", fullTargetPath)
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, &connector.ExecOptions{Sudo: rmSudo})

	if errRm != nil {
		logger.Error("Failed to remove downloaded Docker archive during rollback (best effort).", "path", fullTargetPath, "stderr", string(stderrRm), "error", errRm)
	} else {
		logger.Info("Downloaded Docker archive removed successfully.", "path", fullTargetPath)
	}

	if s.OutputArchiveCacheKey != "" {
		ctx.StepCache().Delete(s.OutputArchiveCacheKey)
		logger.Debug("Removed downloaded archive path from cache.", "key", s.OutputArchiveCacheKey)
	}
	return nil
}

var _ step.Step = (*DownloadDockerStepSpec)(nil)
