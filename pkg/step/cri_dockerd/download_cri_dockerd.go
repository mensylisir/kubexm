package cri_dockerd

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

// DownloadCriDockerdStepSpec defines parameters for downloading cri-dockerd archives.
type DownloadCriDockerdStepSpec struct {
	spec.StepMeta `json:",inline"`

	Version             string `json:"version,omitempty"` // e.g., "0.3.10" (without 'v')
	Arch                string `json:"arch,omitempty"`    // e.g., "amd64", "arm64"
	DownloadURLBase     string `json:"downloadURLBase,omitempty"`
	TargetDir           string `json:"targetDir,omitempty"`
	TargetFilename      string `json:"targetFilename,omitempty"`
	Checksum            string `json:"checksum,omitempty"`      // Format: "sha256:<value>"
	OutputArchiveCacheKey string `json:"outputArchiveCacheKey,omitempty"` // Required
	Sudo                bool   `json:"sudo,omitempty"`
}

// NewDownloadCriDockerdStepSpec creates a new DownloadCriDockerdStepSpec.
func NewDownloadCriDockerdStepSpec(name, description, version, arch, outputArchiveCacheKey string) *DownloadCriDockerdStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Download cri-dockerd %s (%s)", version, arch)
	}
	finalDescription := description
	// Description refined in populateDefaults

	if outputArchiveCacheKey == "" {
		// This is a required field.
	}

	return &DownloadCriDockerdStepSpec{
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
func (s *DownloadCriDockerdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *DownloadCriDockerdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *DownloadCriDockerdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *DownloadCriDockerdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *DownloadCriDockerdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *DownloadCriDockerdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// mapArchToCriDockerdArch maps common arch names to cri-dockerd's naming if needed.
// cri-dockerd seems to use common names like amd64, arm64 directly.
func (s *DownloadCriDockerdStepSpec) mapArchToCriDockerdArch() string {
	mappedArch := strings.ToLower(s.Arch)
	// Add mappings if cri-dockerd uses different arch names in its release assets
	// For example, if it used x86_64 for amd64:
	// if mappedArch == "amd64" { return "x86_64" }
	return mappedArch // Assuming direct mapping for now
}

func (s *DownloadCriDockerdStepSpec) populateDefaults(logger runtime.Logger, hostArch string) {
	if s.Arch == "" {
		s.Arch = hostArch // Use host's architecture if not specified
		logger.Debug("Arch defaulted to host architecture.", "arch", s.Arch)
	}

	if s.DownloadURLBase == "" {
		s.DownloadURLBase = "https://github.com/Mirantis/cri-dockerd/releases/download"
		logger.Debug("DownloadURLBase defaulted.", "url", s.DownloadURLBase)
	}
	if s.TargetDir == "" {
		s.TargetDir = "/tmp/kubexms_downloads"
		logger.Debug("TargetDir defaulted.", "dir", s.TargetDir)
	}

	// cri-dockerd release artifact naming is cri-dockerd-<version_without_v>.<arch>.tgz
	// and tag is v<version>.
	// Example: https://github.com/Mirantis/cri-dockerd/releases/download/v0.3.10/cri-dockerd-0.3.10.amd64.tgz
	versionWithoutV := strings.TrimPrefix(s.Version, "v")
	mappedArch := s.mapArchToCriDockerdArch()


	if s.TargetFilename == "" {
		s.TargetFilename = fmt.Sprintf("cri-dockerd-%s.%s.tgz", versionWithoutV, mappedArch)
		logger.Debug("TargetFilename defaulted.", "filename", s.TargetFilename)
	}

	if s.StepMeta.Description == "" {
		fullURL := fmt.Sprintf("%s/v%s/%s", strings.TrimSuffix(s.DownloadURLBase, "/"), versionWithoutV, s.TargetFilename)
		s.StepMeta.Description = fmt.Sprintf("Downloads cri-dockerd archive version %s for %s from %s to %s/%s.",
			s.Version, mappedArch, fullURL, s.TargetDir, s.TargetFilename)
		if s.Checksum != "" {
			s.StepMeta.Description += fmt.Sprintf(" Verifies checksum %s.", s.Checksum)
		}
	}
}

// Precheck determines if the cri-dockerd archive seems to be already downloaded and valid.
func (s *DownloadCriDockerdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger, host.GetArch())

	if s.Version == "" || s.OutputArchiveCacheKey == "" {
		return false, fmt.Errorf("Version and OutputArchiveCacheKey must be specified for %s", s.GetName())
	}
	if s.TargetFilename == "" || s.TargetDir == "" { // Should be set by populateDefaults
		return false, fmt.Errorf("TargetFilename or TargetDir is empty after defaults for %s", s.GetName())
	}


	fullTargetPath := filepath.Join(s.TargetDir, s.TargetFilename)
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), fullTargetPath)
	if err != nil {
		logger.Warn("Failed to check if target file exists, will attempt download.", "path", fullTargetPath, "error", err)
		return false, nil
	}

	if !exists {
		logger.Info("Target cri-dockerd archive does not exist. Download needed.", "path", fullTargetPath)
		return false, nil
	}

	logger.Info("Target cri-dockerd archive exists.", "path", fullTargetPath)
	if s.Checksum == "" {
		logger.Info("No checksum provided. Assuming existing file is correct and skipping download.")
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

// Run executes the cri-dockerd archive download.
func (s *DownloadCriDockerdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger, host.GetArch())

	if s.Version == "" || s.OutputArchiveCacheKey == "" {
		return fmt.Errorf("Version and OutputArchiveCacheKey must be specified for %s", s.GetName())
	}
	if s.TargetFilename == "" || s.TargetDir == "" || s.DownloadURLBase == "" || s.Arch == "" {
		return fmt.Errorf("essential fields (TargetFilename, TargetDir, DownloadURLBase, Arch) are empty after defaults for %s", s.GetName())
	}


	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	mkdirSudo := s.Sudo || utils.PathRequiresSudo(s.TargetDir)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", s.TargetDir)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, &connector.ExecOptions{Sudo: mkdirSudo})
	if errMkdir != nil {
		return fmt.Errorf("failed to create target directory %s (stderr: %s) on host %s: %w", s.TargetDir, string(stderrMkdir), host.GetName(), errMkdir)
	}

	versionWithV := s.Version
	if !strings.HasPrefix(versionWithV, "v") {
		versionWithV = "v" + versionWithV
	}
	// TargetFilename is already constructed with version without 'v' by populateDefaults.
	downloadURL := fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(s.DownloadURLBase, "/"), versionWithV, s.TargetFilename)
	fullTargetPath := filepath.Join(s.TargetDir, s.TargetFilename)

	logger.Info("Downloading cri-dockerd archive.", "url", downloadURL, "destination", fullTargetPath)

	curlSudo := s.Sudo || utils.PathRequiresSudo(fullTargetPath)
	curlCmd := fmt.Sprintf("curl -sfL -o %s %s", fullTargetPath, downloadURL)
	_, stderrCurl, errCurl := conn.Exec(ctx.GoContext(), curlCmd, &connector.ExecOptions{Sudo: curlSudo})
	if errCurl != nil {
		return fmt.Errorf("failed to download cri-dockerd archive from %s to %s (stderr: %s): %w", downloadURL, fullTargetPath, string(stderrCurl), errCurl)
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
				_ = conn.Remove(ctx.GoContext(), fullTargetPath, connector.RemoveOptions{Sudo: curlSudo})
				return fmt.Errorf("checksum mismatch for downloaded file %s: expected %s (%s), got %s", fullTargetPath, expectedSum, algo, actualSum)
			}
			logger.Info("Checksum verified for downloaded cri-dockerd archive.", "path", fullTargetPath)
		} else {
			return fmt.Errorf("invalid checksum format provided: %s. Expected type:value", s.Checksum)
		}
	}

	ctx.StepCache().Set(s.OutputArchiveCacheKey, fullTargetPath)
	logger.Info("cri-dockerd archive downloaded successfully and path cached.", "key", s.OutputArchiveCacheKey, "path", fullTargetPath)
	return nil
}

// Rollback removes the downloaded cri-dockerd archive.
func (s *DownloadCriDockerdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger, host.GetArch())

	if s.TargetDir == "" || s.TargetFilename == "" {
		logger.Info("TargetDir or TargetFilename is empty, cannot determine file to roll back.")
		return nil
	}
	fullTargetPath := filepath.Join(s.TargetDir, s.TargetFilename)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	logger.Info("Attempting to remove downloaded cri-dockerd archive.", "path", fullTargetPath)
	rmSudo := s.Sudo || utils.PathRequiresSudo(fullTargetPath)
	rmCmd := fmt.Sprintf("rm -f %s", fullTargetPath)
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, &connector.ExecOptions{Sudo: rmSudo})

	if errRm != nil {
		logger.Error("Failed to remove downloaded cri-dockerd archive during rollback (best effort).", "path", fullTargetPath, "stderr", string(stderrRm), "error", errRm)
	} else {
		logger.Info("Downloaded cri-dockerd archive removed successfully.", "path", fullTargetPath)
	}

	if s.OutputArchiveCacheKey != "" {
		ctx.StepCache().Delete(s.OutputArchiveCacheKey)
		logger.Debug("Removed downloaded archive path from cache.", "key", s.OutputArchiveCacheKey)
	}
	return nil
}

var _ step.Step = (*DownloadCriDockerdStepSpec)(nil)
