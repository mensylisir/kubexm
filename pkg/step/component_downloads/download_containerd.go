package component_downloads

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	// "time" // No longer directly used by the step methods for step.Result

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
	// "github.com/mensylisir/kubexm/pkg/utils" // Removed
)

// Constants for Task Cache keys
const (
	ContainerdDownloadedPathKey     = "ContainerdDownloadedPath"
	ContainerdDownloadedFileNameKey = "ContainerdDownloadedFileName"
	ContainerdComponentTypeKey      = "ContainerdComponentType"
	ContainerdVersionKey            = "ContainerdVersion"
	ContainerdArchKey               = "ContainerdArch"
	ContainerdChecksumKey           = "ContainerdChecksum"
	ContainerdDownloadURLKey        = "ContainerdDownloadURL"
)

// DownloadContainerdStep downloads the containerd binary.
type DownloadContainerdStep struct {
	meta                 spec.StepMeta
	Version              string
	Arch                 string
	Zone                 string // e.g., "cn" for different download sources
	DownloadDir          string // Directory on the target host to download to
	Checksum             string // Expected checksum (e.g., "sha256:value")
	DownloadSudo         bool   // Sudo for mkdir and potentially for remove if needed
	OutputFilePathKey    string
	OutputFileNameKey    string
	OutputComponentTypeKey string
	OutputVersionKey     string
	OutputArchKey        string
	OutputChecksumKey    string
	OutputURLKey         string
	// Internal fields, not part of constructor args usually
	determinedArch     string
	determinedFileName string
	determinedURL      string
}

// NewDownloadContainerdStep creates a new DownloadContainerdStep.
func NewDownloadContainerdStep(
	instanceName string,
	version, arch, zone, downloadDir, checksum string,
	downloadSudo bool,
	outputFilePathKey, outputFileNameKey, outputComponentTypeKey,
	outputVersionKey, outputArchKey, outputChecksumKey, outputURLKey string,
) step.Step {
	// Apply default keys if provided keys are empty
	if outputFilePathKey == "" {
		outputFilePathKey = ContainerdDownloadedPathKey
	}
	if outputFileNameKey == "" {
		outputFileNameKey = ContainerdDownloadedFileNameKey
	}
	if outputComponentTypeKey == "" {
		outputComponentTypeKey = ContainerdComponentTypeKey
	}
	if outputVersionKey == "" {
		outputVersionKey = ContainerdVersionKey
	}
	if outputArchKey == "" {
		outputArchKey = ContainerdArchKey
	}
	if outputChecksumKey == "" {
		outputChecksumKey = ContainerdChecksumKey
	}
	if outputURLKey == "" {
		outputURLKey = ContainerdDownloadURLKey
	}

	name := instanceName
	if name == "" {
		name = "DownloadContainerd"
	}

	s := &DownloadContainerdStep{
		meta: spec.StepMeta{
			Name: name,
			// Description will be set more accurately by populateAndDetermineInternals
			Description: fmt.Sprintf("Downloads containerd version %s.", version),
		},
		Version:              version,
		Arch:                 arch,
		Zone:                 zone,
		DownloadDir:          downloadDir,
		Checksum:             checksum,
		DownloadSudo:         downloadSudo,
		OutputFilePathKey:    outputFilePathKey,
		OutputFileNameKey:    outputFileNameKey,
		OutputComponentTypeKey: outputComponentTypeKey,
		OutputVersionKey:     outputVersionKey,
		OutputArchKey:        outputArchKey,
		OutputChecksumKey:    outputChecksumKey,
		OutputURLKey:         outputURLKey,
	}
	return s
}

func (s *DownloadContainerdStep) populateAndDetermineInternals(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	if s.determinedArch == "" {
		archToUse := s.Arch
		if archToUse == "" {
			if host != nil {
				facts, err := ctx.GetHostFacts(host)
				if err != nil {
					return fmt.Errorf("failed to get host facts for arch detection in step %s on host %s: %w", s.meta.Name, host.GetName(), err)
				}
				if facts.OS != nil && facts.OS.Arch != "" {
					archToUse = facts.OS.Arch
					if archToUse == "x86_64" {
						archToUse = "amd64"
					} else if archToUse == "aarch64" {
						archToUse = "arm64"
					}
					logger.Debug("Host architecture determined", "rawArch", facts.OS.Arch, "usingArch", archToUse)
				} else {
					return fmt.Errorf("host OS.Arch is empty, cannot determine architecture for DownloadContainerdStep on host %s", host.GetName())
				}
			} else {
				return fmt.Errorf("arch is not specified and host is nil, cannot determine architecture for DownloadContainerdStep")
			}
		}
		s.determinedArch = archToUse
	}

	if s.determinedFileName == "" {
		s.determinedFileName = fmt.Sprintf("containerd-%s-linux-%s.tar.gz", strings.TrimPrefix(s.Version, "v"), s.determinedArch)
	}

	if s.determinedURL == "" {
		versionWithV := s.Version
		if !strings.HasPrefix(versionWithV, "v") {
			versionWithV = "v" + versionWithV
		}
		effectiveZone := s.Zone
		// TODO: Replace os.Getenv("KKZONE") with a value from ClusterSpec or runtime config for better testability/explicitness.
		if effectiveZone == "" {
			effectiveZone = os.Getenv("KKZONE")
		}

		baseURL := fmt.Sprintf("https://github.com/containerd/containerd/releases/download/%s/%s", versionWithV, s.determinedFileName)
		if effectiveZone == "cn" {
			// TODO: Make mirror URL configurable.
			baseURL = fmt.Sprintf("https://download.fastgit.org/containerd/containerd/releases/download/%s/%s", versionWithV, s.determinedFileName)
		}
		s.determinedURL = baseURL
	}
	// Update meta description with determined values
	s.meta.Description = fmt.Sprintf("Downloads containerd version %s for %s architecture from %s.", s.Version, s.determinedArch, s.determinedURL)
	return nil
}

// Meta returns the step's metadata.
func (s *DownloadContainerdStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DownloadContainerdStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		ctx.GetLogger().Error("Failed to populate internal fields during Precheck", "step", s.meta.Name, "host", host.GetName(), "error", err)
		return false, err
	}
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	if s.DownloadDir == "" {
		return false, fmt.Errorf("DownloadDir not set for step %s on host %s", s.meta.Name, host.GetName())
	}

	expectedFilePath := filepath.Join(s.DownloadDir, s.determinedFileName)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, expectedFilePath)
	if err != nil {
		logger.Warn("Failed to check existence of target file, proceeding to Run phase.", "path", expectedFilePath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("Containerd archive does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Containerd archive exists.", "path", expectedFilePath)

	if s.Checksum != "" {
		checksumValue := s.Checksum
		checksumType := "sha256" // Default
		if strings.Contains(s.Checksum, ":") {
			parts := strings.SplitN(s.Checksum, ":", 2)
			checksumType = parts[0]
			checksumValue = parts[1]
		}
		if strings.ToLower(checksumType) != "sha256" {
			logger.Warn("Only SHA256 checksum verification is currently supported by runnerSvc.GetSHA256. Skipping checksum.", "type", checksumType)
		} else {
			logger.Info("Verifying checksum", "type", checksumType)
			actualHash, errC := runnerSvc.GetSHA256(ctx.GoContext(), conn, expectedFilePath)
			if errC != nil {
				logger.Warn("Failed to get checksum, assuming invalid and will re-download.", "error", errC)
				return false, nil
			}
			if !strings.EqualFold(actualHash, checksumValue) {
				logger.Warn("Checksum mismatch. File will be re-downloaded.", "expected", checksumValue, "actual", actualHash)
				// runnerSvc.Remove(ctx.GoContext(), conn, expectedFilePath, s.DownloadSudo) // Remove bad file
				return false, nil
			}
			logger.Info("Checksum verified.")
		}
	}

	ctx.TaskCache().Set(s.OutputFilePathKey, expectedFilePath)
	ctx.TaskCache().Set(s.OutputFileNameKey, s.determinedFileName)
	ctx.TaskCache().Set(s.OutputComponentTypeKey, "CONTAINER_RUNTIME")
	ctx.TaskCache().Set(s.OutputVersionKey, s.Version)
	ctx.TaskCache().Set(s.OutputArchKey, s.determinedArch)
	if s.Checksum != "" {
		ctx.TaskCache().Set(s.OutputChecksumKey, s.Checksum)
	}
	ctx.TaskCache().Set(s.OutputURLKey, s.determinedURL)
	logger.Info("Step is considered done, relevant info cached/updated.")
	return true, nil
}

func (s *DownloadContainerdStep) Run(ctx runtime.StepContext, host connector.Host) error {
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		ctx.GetLogger().Error("Failed to populate internal fields during Run", "step", s.meta.Name, "host", host.GetName(), "error", err)
		return err
	}
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	if s.DownloadDir == "" {
		return fmt.Errorf("DownloadDir not set for step %s on host %s", s.meta.Name, host.GetName())
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}
	facts, err := ctx.GetHostFacts(host)
	if err != nil {
		return fmt.Errorf("failed to get host facts for %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	errMkdir := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.DownloadDir, "0755", s.DownloadSudo)
	if errMkdir != nil {
		return fmt.Errorf("failed to create download directory %s for step %s on host %s: %w", s.DownloadDir, s.meta.Name, host.GetName(), errMkdir)
	}

	destinationPath := filepath.Join(s.DownloadDir, s.determinedFileName)
	logger.Info("Attempting to download containerd", "url", s.determinedURL, "destination", destinationPath)

	// runnerSvc.Download takes the full destination file path.
	dlErr := runnerSvc.Download(ctx.GoContext(), conn, facts, s.determinedURL, destinationPath, s.DownloadSudo)
	if dlErr != nil {
		return fmt.Errorf("failed to download containerd for step %s on host %s from URL %s: %w", s.meta.Name, host.GetName(), s.determinedURL, dlErr)
	}
	logger.Info("Containerd downloaded successfully.", "path", destinationPath)

	if s.Checksum != "" {
		checksumValue := s.Checksum
		checksumType := "sha256"
		if strings.Contains(s.Checksum, ":") {
			parts := strings.SplitN(s.Checksum, ":", 2)
			checksumType = parts[0]
			checksumValue = parts[1]
		}
		if strings.ToLower(checksumType) != "sha256" {
			logger.Warn("Only SHA256 checksum verification is currently supported by runnerSvc.GetSHA256. Skipping checksum check post-download.", "type", checksumType)
		} else {
			logger.Info("Verifying checksum post-download", "type", checksumType)
			actualHash, errC := runnerSvc.GetSHA256(ctx.GoContext(), conn, destinationPath)
			if errC != nil {
				return fmt.Errorf("failed to get checksum for downloaded file %s for step %s on host %s: %w", destinationPath, s.meta.Name, host.GetName(), errC)
			}
			if !strings.EqualFold(actualHash, checksumValue) {
				// runnerSvc.Remove(ctx.GoContext(), conn, destinationPath, s.DownloadSudo) // Remove bad file
				return fmt.Errorf("checksum mismatch for downloaded file %s (expected %s, got %s) for step %s on host %s", destinationPath, checksumValue, actualHash, s.meta.Name, host.GetName())
			}
			logger.Info("Checksum verified post-download.")
		}
	}

	ctx.TaskCache().Set(s.OutputFilePathKey, destinationPath)
	ctx.TaskCache().Set(s.OutputFileNameKey, s.determinedFileName)
	ctx.TaskCache().Set(s.OutputComponentTypeKey, "CONTAINER_RUNTIME")
	ctx.TaskCache().Set(s.OutputVersionKey, s.Version)
	ctx.TaskCache().Set(s.OutputArchKey, s.determinedArch)
	if s.Checksum != "" {
		ctx.TaskCache().Set(s.OutputChecksumKey, s.Checksum)
	}
	ctx.TaskCache().Set(s.OutputURLKey, s.determinedURL)
	return nil
}

func (s *DownloadContainerdStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		ctx.GetLogger().Warn("Could not determine file name for rollback during populateInternals. No specific file to remove.", "step", s.meta.Name, "host", host.GetName(), "error", err)
		return nil
	}
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")


	if s.determinedFileName == "" || s.DownloadDir == "" {
		logger.Warn("Cannot determine file path for rollback; filename or download dir not set/determined.")
		return nil
	}
	downloadedFilePath := filepath.Join(s.DownloadDir, s.determinedFileName)

	logger.Info("Attempting to remove downloaded file for rollback.", "path", downloadedFilePath)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback. File may not be removed.", "error", err)
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.meta.Name, err)
	}

	if errRemove := runnerSvc.Remove(ctx.GoContext(), conn, downloadedFilePath, s.DownloadSudo); errRemove != nil {
		logger.Error("Failed to remove file during rollback. It might have already been removed or permissions issue.", "path", downloadedFilePath, "error", errRemove)
		// Non-critical for rollback to fail here.
	} else {
		logger.Info("Successfully removed downloaded file for rollback (if it existed).", "path", downloadedFilePath)
	}

	ctx.TaskCache().Delete(s.OutputFilePathKey)
	ctx.TaskCache().Delete(s.OutputFileNameKey)
	ctx.TaskCache().Delete(s.OutputComponentTypeKey)
	ctx.TaskCache().Delete(s.OutputVersionKey)
	ctx.TaskCache().Delete(s.OutputArchKey)
	ctx.TaskCache().Delete(s.OutputChecksumKey)
	ctx.TaskCache().Delete(s.OutputURLKey)
	logger.Debug("Cleaned up task cache keys for rollback.")
	return nil
}

// Ensure DownloadContainerdStep implements the step.Step interface.
var _ step.Step = (*DownloadContainerdStep)(nil)
