package component_downloads

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	// "time" // No longer directly used

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For DownloadFileWithConnector
)

// Constants for Task Cache keys
const (
	EtcdDownloadedPathKey      = "EtcdDownloadedPath"
	EtcdDownloadedFileNameKey  = "EtcdDownloadedFileName"
	EtcdComponentTypeKey       = "EtcdComponentType"
	EtcdVersionKey             = "EtcdVersion"
	EtcdArchKey                = "EtcdArch"
	EtcdChecksumKey            = "EtcdChecksum"
	EtcdDownloadURLKey         = "EtcdDownloadURL"
)

// DownloadEtcdStep downloads the etcd binary.
type DownloadEtcdStep struct {
	Version              string
	Arch                 string
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
	// Internal fields
	determinedArch     string
	determinedFileName string
	determinedURL      string
}

// NewDownloadEtcdStep creates a new DownloadEtcdStep.
func NewDownloadEtcdStep(
	version, arch, zone, downloadDir, checksum string,
	outputFilePathKey, outputFileNameKey, outputComponentTypeKey,
	outputVersionKey, outputArchKey, outputChecksumKey, outputURLKey string,
) step.Step {
	if outputFilePathKey == "" { outputFilePathKey = EtcdDownloadedPathKey }
	if outputFileNameKey == "" { outputFileNameKey = EtcdDownloadedFileNameKey }
	if outputComponentTypeKey == "" { outputComponentTypeKey = EtcdComponentTypeKey }
	if outputVersionKey == "" { outputVersionKey = EtcdVersionKey }
	if outputArchKey == "" { outputArchKey = EtcdArchKey }
	if outputChecksumKey == "" { outputChecksumKey = EtcdChecksumKey }
	if outputURLKey == "" { outputURLKey = EtcdDownloadURLKey }

	return &DownloadEtcdStep{
		Version:              version,
		Arch:                 arch,
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

func (s *DownloadEtcdStep) populateAndDetermineInternals(ctx runtime.StepContext, host connector.Host) error {
	if s.determinedArch == "" {
		archToUse := s.Arch
		if archToUse == "" {
			if host != nil {
				hostArch := host.GetArch()
				if hostArch == "x86_64" {
					archToUse = "amd64"
				} else if hostArch == "aarch64" {
					archToUse = "arm64"
				} else {
					archToUse = hostArch
				}
				ctx.GetLogger().Debug("Host architecture determined for etcd", "rawArch", hostArch, "usingArch", archToUse)
			} else {
				return fmt.Errorf("arch is not specified and host is nil, cannot determine architecture for DownloadEtcdStep")
			}
		}
		s.determinedArch = archToUse
	}

	if s.determinedFileName == "" {
		// etcd version usually does not have 'v' prefix in filename, e.g. etcd-3.5.0-linux-amd64.tar.gz
		s.determinedFileName = fmt.Sprintf("etcd-%s-linux-%s.tar.gz", strings.TrimPrefix(s.Version, "v"), s.determinedArch)
	}

	if s.determinedURL == "" {
		// etcd versions for URLs typically include 'v', e.g. v3.5.0
		versionWithV := s.Version
		if !strings.HasPrefix(versionWithV, "v") {
			versionWithV = "v" + versionWithV
		}
		effectiveZone := s.Zone
		if effectiveZone == "" { effectiveZone = os.Getenv("KKZONE") }

		baseURL := fmt.Sprintf("https://github.com/etcd-io/etcd/releases/download/%s/%s", versionWithV, s.determinedFileName)
		// Note: The old code used coreos/etcd, new is etcd-io/etcd. Confirmed etcd-io/etcd is current.
		if effectiveZone == "cn" {
			// Example: https://kubernetes-release.pek3b.qingstor.com/etcd/release/download/v3.5.0/etcd-v3.5.0-linux-amd64.tar.gz
			// The qingstor URL seems to require the 'v' prefix in the path component for version,
			// but the filename in that path might or might not have it.
			// The old code used `version` (without v) for qingstor path, and `fileName` (which also has no v).
			// Let's use versionWithV for path consistency with github.
			baseURL = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/etcd/release/download/%s/%s", versionWithV, s.determinedFileName)
		}
		s.determinedURL = baseURL
	}
	return nil
}

func (s *DownloadEtcdStep) Name() string {
	return "Download Etcd"
}

func (s *DownloadEtcdStep) Description() string {
	archDesc := s.Arch
	if s.determinedArch != "" { archDesc = s.determinedArch }
	return fmt.Sprintf("Downloads etcd version %s for %s architecture.", s.Version, archDesc)
}

func (s *DownloadEtcdStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		logger.Error("Failed to populate internal fields", "error", err)
		return false, err
	}
	if s.DownloadDir == "" {
		return false, fmt.Errorf("DownloadDir not set for step %s on host %s", s.Name(), host.GetName())
	}

	expectedFilePath := filepath.Join(s.DownloadDir, s.determinedFileName)
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), expectedFilePath)
	if err != nil {
		logger.Warn("Failed to check existence of target file, proceeding to Run phase.", "path", expectedFilePath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("Etcd archive does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Etcd archive exists.", "path", expectedFilePath)

	if s.Checksum != "" {
		checksumValue := s.Checksum; checksumType := "sha256"
		if strings.Contains(s.Checksum, ":") {
			parts := strings.SplitN(s.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Info("Verifying checksum", "type", checksumType)
		actualHash, errC := conn.GetFileChecksum(ctx.GoContext(), expectedFilePath, checksumType)
		if errC != nil {
			logger.Warn("Failed to get checksum, assuming invalid.", "error", errC)
			return false, nil
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			logger.Warn("Checksum mismatch. File will be re-downloaded.", "expected", checksumValue, "actual", actualHash)
			return false, nil
		}
		logger.Info("Checksum verified.")
	}

	ctx.TaskCache().Set(s.OutputFilePathKey, expectedFilePath)
	ctx.TaskCache().Set(s.OutputFileNameKey, s.determinedFileName)
	ctx.TaskCache().Set(s.OutputComponentTypeKey, "ETCD")
	ctx.TaskCache().Set(s.OutputVersionKey, s.Version)
	ctx.TaskCache().Set(s.OutputArchKey, s.determinedArch)
	if s.Checksum != "" { ctx.TaskCache().Set(s.OutputChecksumKey, s.Checksum) }
	ctx.TaskCache().Set(s.OutputURLKey, s.determinedURL)
	logger.Info("Step is considered done, relevant info cached/updated.")
	return true, nil
}

func (s *DownloadEtcdStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		logger.Error("Failed to populate internal fields", "error", err)
		return err
	}
	if s.DownloadDir == "" {
		return fmt.Errorf("DownloadDir not set for step %s on host %s", s.Name(), host.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	errMkdir := conn.Mkdir(ctx.GoContext(), s.DownloadDir, "0755")
	if errMkdir != nil {
		return fmt.Errorf("failed to create download directory %s: %w", s.DownloadDir, errMkdir)
	}

	destinationPath := filepath.Join(s.DownloadDir, s.determinedFileName)
	logger.Info("Attempting to download etcd", "url", s.determinedURL, "destination", destinationPath)

	downloadedPath, dlErr := utils.DownloadFileWithConnector(ctx.GoContext(), logger, conn, s.determinedURL, s.DownloadDir, s.determinedFileName, s.Checksum)
	if dlErr != nil {
		return fmt.Errorf("failed to download etcd from URL %s: %w", s.determinedURL, dlErr)
	}
	logger.Info("Etcd downloaded successfully.", "path", downloadedPath)

	if s.Checksum != "" {
		checksumValue := s.Checksum; checksumType := "sha256"
		if strings.Contains(s.Checksum, ":") {
			parts := strings.SplitN(s.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Info("Verifying checksum post-download", "type", checksumType)
		actualHash, errC := conn.GetFileChecksum(ctx.GoContext(), downloadedPath, checksumType)
		if errC != nil {
			return fmt.Errorf("failed to get checksum for downloaded etcd file %s: %w", downloadedPath, errC)
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			return fmt.Errorf("checksum mismatch for downloaded etcd file %s (expected %s, got %s)", downloadedPath, checksumValue, actualHash)
		}
		logger.Info("Checksum verified post-download.")
	}

	ctx.TaskCache().Set(s.OutputFilePathKey, downloadedPath)
	ctx.TaskCache().Set(s.OutputFileNameKey, s.determinedFileName)
	ctx.TaskCache().Set(s.OutputComponentTypeKey, "ETCD")
	ctx.TaskCache().Set(s.OutputVersionKey, s.Version)
	ctx.TaskCache().Set(s.OutputArchKey, s.determinedArch)
	if s.Checksum != "" { ctx.TaskCache().Set(s.OutputChecksumKey, s.Checksum) }
	ctx.TaskCache().Set(s.OutputURLKey, s.determinedURL)
	return nil
}

func (s *DownloadEtcdStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")
	if err := s.populateAndDetermineInternals(ctx, host); err != nil {
		logger.Warn("Could not determine file name for rollback", "error", err)
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
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.Name(), err)
	}

	removeOpts := connector.RemoveOptions{ Recursive: false, IgnoreNotExist: true }
	if errRemove := conn.Remove(ctx.GoContext(), downloadedFilePath, removeOpts); errRemove != nil {
		logger.Error("Failed to remove file during rollback.", "path", downloadedFilePath, "error", errRemove)
		return fmt.Errorf("failed to remove file %s during rollback for step %s: %w", downloadedFilePath, s.Name(), host.GetName(), errRemove)
	}
	logger.Info("Successfully removed downloaded file for rollback (if it existed).", "path", downloadedFilePath)

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

// Ensure DownloadEtcdStep implements the step.Step interface.
var _ step.Step = (*DownloadEtcdStep)(nil)
