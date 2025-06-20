package component_downloads

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	// "time" // No longer directly used by the step methods for step.Result

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For DownloadFileWithConnector
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
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
)

// DownloadContainerdStepSpec downloads the containerd binary.
type DownloadContainerdStepSpec struct {
	spec.StepMeta        `json:",inline"`
	Version              string `json:"version,omitempty"`
	Arch                 string `json:"arch,omitempty"`
	Zone                 string // e.g., "cn" for different download sources
	DownloadDir          string // Directory on the target host to download to
	Checksum             string // Expected checksum (e.g., "sha256:value")
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

// NewDownloadContainerdStepSpec creates a new DownloadContainerdStepSpec.
func NewDownloadContainerdStep(
	version, arch, zone, downloadDir, checksum string,
	outputFilePathKey, outputFileNameKey, outputComponentTypeKey,
	outputVersionKey, outputArchKey, outputChecksumKey, outputURLKey string,
) *DownloadContainerdStepSpec {
	// Apply default keys if provided keys are empty
	name := "Download Containerd" // Default name, can be customized by caller if needed
	description := fmt.Sprintf("Downloads containerd version %s for %s architecture.", version, arch)


	if outputFilePathKey == "" { outputFilePathKey = ContainerdDownloadedPathKey }
	if outputFileNameKey == "" { outputFileNameKey = ContainerdDownloadedFileNameKey }
	if outputComponentTypeKey == "" { outputComponentTypeKey = ContainerdComponentTypeKey }
	if outputVersionKey == "" { outputVersionKey = ContainerdVersionKey }
	if outputArchKey == "" { outputArchKey = ContainerdArchKey }
	if outputChecksumKey == "" { outputChecksumKey = ContainerdChecksumKey }
	if outputURLKey == "" { outputURLKey = ContainerdDownloadURLKey }

	return &DownloadContainerdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        name,
			Description: description, // Will be updated by populateAndDetermineInternals
		},
		Version:              version,
		Arch:                 arch, // Can be empty, Precheck/Run will determine
		Zone:                 zone,
		DownloadDir:          downloadDir,
		Checksum:             checksum,
		OutputFilePathKey:    outputFilePathKey,
		OutputFileNameKey:    outputFileNameKey,
		OutputComponentTypeKey: outputComponentTypeKey,
		OutputVersionKey:     outputVersionKey,
		OutputArchKey:        outputArchKey,
		OutputChecksumKey:    outputChecksumKey,
		OutputURLKey:         outputURLKey,
	}
}

func (s *DownloadContainerdStepSpec) populateAndDetermineInternals(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName())
	if s.determinedArch == "" {
		archToUse := s.Arch
		if archToUse == "" {
			if host != nil {
				hostArch := host.GetArch()
				if hostArch == "x86_64" { archToUse = "amd64"
				} else if hostArch == "aarch64" { archToUse = "arm64"
				} else { archToUse = hostArch }
				logger.Debug("Host architecture determined", "rawArch", hostArch, "usingArch", archToUse)
			} else {
				return fmt.Errorf("arch is not specified and host is nil, cannot determine architecture for %s", s.GetName())
			}
		}
		s.determinedArch = archToUse
	}

	if s.determinedFileName == "" {
		s.determinedFileName = fmt.Sprintf("containerd-%s-linux-%s.tar.gz", strings.TrimPrefix(s.Version, "v"), s.determinedArch)
	}

	if s.determinedURL == "" {
		versionWithV := s.Version
		if !strings.HasPrefix(versionWithV, "v") { versionWithV = "v" + versionWithV }
		effectiveZone := s.Zone
		if effectiveZone == "" { effectiveZone = os.Getenv("KKZONE") } // Consider passing zone via config

		baseURL := fmt.Sprintf("https://github.com/containerd/containerd/releases/download/%s/%s", versionWithV, s.determinedFileName)
		if effectiveZone == "cn" {
			baseURL = fmt.Sprintf("https://download.fastgit.org/containerd/containerd/releases/download/%s/%s", versionWithV, s.determinedFileName)
		}
		s.determinedURL = baseURL
	}
	// Update StepMeta description with determined values
	s.StepMeta.Description = fmt.Sprintf("Downloads containerd version %s for %s architecture from %s.", s.Version, s.determinedArch, s.determinedURL)
	return nil
}

// Name returns the step's name (implementing step.Step).
func (s *DownloadContainerdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *DownloadContainerdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *DownloadContainerdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *DownloadContainerdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *DownloadContainerdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *DownloadContainerdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *DownloadContainerdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if err := s.populateAndDetermineInternals(ctx, host); err != nil { // This also updates description
		logger.Error("Failed to populate internal fields during precheck", "error", err)
		return false, err
	}
	if s.DownloadDir == "" {
		return false, fmt.Errorf("DownloadDir not set for step %s on host %s", s.GetName(), host.GetName())
	}

	expectedFilePath := filepath.Join(s.DownloadDir, s.determinedFileName)
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.GetName(), err)
	}

	// Assuming connector.Exists(ctx, path) (bool, error)
	exists, err := conn.Exists(ctx.GoContext(), expectedFilePath)
	if err != nil {
		// Don't consider this a fatal error for precheck, Run should attempt it.
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
		logger.Info("Verifying checksum", "type", checksumType)
		// Assuming connector.GetFileChecksum(ctx, path, type) (string, error)
		actualHash, errC := conn.GetFileChecksum(ctx.GoContext(), expectedFilePath, checksumType)
		if errC != nil {
			logger.Warn("Failed to get checksum, assuming invalid and will re-download.", "error", errC)
			return false, nil
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			logger.Warn("Checksum mismatch. File will be re-downloaded.", "expected", checksumValue, "actual", actualHash)
			// Consider removing the bad file here if desired:
			// conn.Remove(ctx.GoContext(), expectedFilePath, connector.RemoveOptions{IgnoreNotExist: true})
			return false, nil
		}
		logger.Info("Checksum verified.")
	}

	// If file exists and checksum matches (or no checksum specified), update cache and return true
	ctx.TaskCache().Set(s.OutputFilePathKey, expectedFilePath)
	ctx.TaskCache().Set(s.OutputFileNameKey, s.determinedFileName)
	ctx.TaskCache().Set(s.OutputComponentTypeKey, "CONTAINER_RUNTIME") // Standardized component type
	ctx.TaskCache().Set(s.OutputVersionKey, s.Version)
	ctx.TaskCache().Set(s.OutputArchKey, s.determinedArch)
	if s.Checksum != "" { ctx.TaskCache().Set(s.OutputChecksumKey, s.Checksum) }
	ctx.TaskCache().Set(s.OutputURLKey, s.determinedURL)
	logger.Info("Step is considered done, relevant info cached/updated.")
	return true, nil
}

func (s *DownloadContainerdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if err := s.populateAndDetermineInternals(ctx, host); err != nil { // This also updates description
		logger.Error("Failed to populate internal fields during run", "error", err)
		return err
	}
	if s.DownloadDir == "" {
		return fmt.Errorf("DownloadDir not set for step %s on host %s", s.GetName(), host.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.GetName(), err)
	}

    // Ensure download directory exists
    // Assuming connector.Mkdir(ctx, path, permissions string) error
    errMkdir := conn.Mkdir(ctx.GoContext(), s.DownloadDir, "0755") // Ensure Mkdir handles sudo if needed or use Exec
    if errMkdir != nil {
        return fmt.Errorf("failed to create download directory %s for step %s on host %s: %w", s.DownloadDir, s.GetName(), host.GetName(), errMkdir)
    }

	destinationPath := filepath.Join(s.DownloadDir, s.determinedFileName)
	logger.Info("Attempting to download containerd", "url", s.determinedURL, "destination", destinationPath)

	// utils.DownloadFileWithConnector(goCtx, logger, conn, url, dir, name, checksumString) (string, error)
	downloadedPath, dlErr := utils.DownloadFileWithConnector(ctx.GoContext(), logger, conn, s.determinedURL, s.DownloadDir, s.determinedFileName, s.Checksum) // Assuming this utility exists and works
	if dlErr != nil {
		return fmt.Errorf("failed to download containerd for step %s on host %s from URL %s: %w", s.GetName(), host.GetName(), s.determinedURL, dlErr)
	}
	logger.Info("Containerd downloaded successfully.", "path", downloadedPath)

	// Verify checksum again after download, if specified. DownloadFileWithConnector might do this already.
	// If DownloadFileWithConnector already verifies, this is redundant but harmless.
	if s.Checksum != "" {
		checksumValue := s.Checksum
		checksumType := "sha256"
		if strings.Contains(s.Checksum, ":") {
			parts := strings.SplitN(s.Checksum, ":", 2)
			checksumType = parts[0]
			checksumValue = parts[1]
		}
		logger.Info("Verifying checksum post-download", "type", checksumType)
		actualHash, errC := conn.GetFileChecksum(ctx.GoContext(), downloadedPath, checksumType) // Assuming this method exists on connector
		if errC != nil {
			return fmt.Errorf("failed to get checksum for downloaded file %s for step %s on host %s: %w", downloadedPath, s.GetName(), host.GetName(), errC)
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			// Consider removing the bad file here
			// conn.Remove(ctx.GoContext(), downloadedPath, connector.RemoveOptions{IgnoreNotExist: true})
			return fmt.Errorf("checksum mismatch for downloaded file %s (expected %s, got %s) for step %s on host %s", downloadedPath, checksumValue, actualHash, s.GetName(), host.GetName())
		}
		logger.Info("Checksum verified post-download.")
	}

	ctx.TaskCache().Set(s.OutputFilePathKey, downloadedPath)
	ctx.TaskCache().Set(s.OutputFileNameKey, s.determinedFileName)
	ctx.TaskCache().Set(s.OutputComponentTypeKey, "CONTAINER_RUNTIME")
	ctx.TaskCache().Set(s.OutputVersionKey, s.Version)
	ctx.TaskCache().Set(s.OutputArchKey, s.determinedArch)
	if s.Checksum != "" { ctx.TaskCache().Set(s.OutputChecksumKey, s.Checksum) }
	ctx.TaskCache().Set(s.OutputURLKey, s.determinedURL)
	return nil
}

func (s *DownloadContainerdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")

	// Populate internals to get determinedFileName, as it might not have run if Precheck was true or Run failed early.
	if err := s.populateAndDetermineInternals(ctx, host); err != nil { // This also updates description
		// If we can't even determine filename (e.g. no host for arch), log and return nil as no specific file path to target.
		logger.Warn("Could not determine file name for rollback, possibly due to missing arch/host info early on. No specific file to remove.", "error", err)
		return nil
	}

	if s.determinedFileName == "" || s.DownloadDir == "" {
		logger.Warn("Cannot determine file path for rollback; filename or download dir not set/determined.")
		return nil
	}
	downloadedFilePath := filepath.Join(s.DownloadDir, s.determinedFileName)

	logger.Info("Attempting to remove downloaded file for rollback.", "path", downloadedFilePath)
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		// If connector fails, we can't do much for rollback. Log it.
		logger.Error("Failed to get connector for host during rollback. File may not be removed.", "error", err)
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.GetName(), err)
	}

	removeOpts := connector.RemoveOptions{ Recursive: false, IgnoreNotExist: true }
	if errRemove := conn.Remove(ctx.GoContext(), downloadedFilePath, removeOpts); errRemove != nil {
		logger.Error("Failed to remove file during rollback. It might have already been removed or permissions issue.", "path", downloadedFilePath, "error", errRemove)
		return fmt.Errorf("failed to remove file %s during rollback for step %s on host %s: %w", downloadedFilePath, s.GetName(), host.GetName(), errRemove)
	}
	logger.Info("Successfully removed downloaded file for rollback (if it existed).", "path", downloadedFilePath)

	// Clean up cache keys from TaskCache. Note: StepMeta/Spec pattern usually uses StepCache for outputs.
	// This step's original logic uses TaskCache. If changing to StepCache, update here too.
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

// Ensure DownloadContainerdStepSpec implements the step.Step interface.
var _ step.Step = (*DownloadContainerdStepSpec)(nil)
