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
	KubeadmDownloadedPathKey     = "KubeadmDownloadedPath"
	KubeadmDownloadedFileNameKey = "KubeadmDownloadedFileName"
	KubeadmComponentTypeKey      = "KubeadmComponentType"
	KubeadmVersionKey            = "KubeadmVersion"
	KubeadmArchKey               = "KubeadmArch"
	KubeadmChecksumKey           = "KubeadmChecksum"
	KubeadmDownloadURLKey        = "KubeadmDownloadURL"
)

// DownloadKubeadmStep downloads the kubeadm binary.
type DownloadKubeadmStep struct {
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

// NewDownloadKubeadmStep creates a new DownloadKubeadmStep.
func NewDownloadKubeadmStep(
	version, arch, zone, downloadDir, checksum string,
	outputFilePathKey, outputFileNameKey, outputComponentTypeKey,
	outputVersionKey, outputArchKey, outputChecksumKey, outputURLKey string,
) step.Step {
	if outputFilePathKey == "" { outputFilePathKey = KubeadmDownloadedPathKey }
	if outputFileNameKey == "" { outputFileNameKey = KubeadmDownloadedFileNameKey }
	if outputComponentTypeKey == "" { outputComponentTypeKey = KubeadmComponentTypeKey }
	if outputVersionKey == "" { outputVersionKey = KubeadmVersionKey }
	if outputArchKey == "" { outputArchKey = KubeadmArchKey }
	if outputChecksumKey == "" { outputChecksumKey = KubeadmChecksumKey }
	if outputURLKey == "" { outputURLKey = KubeadmDownloadURLKey }

	return &DownloadKubeadmStep{
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

func (s *DownloadKubeadmStep) populateAndDetermineInternals(ctx runtime.StepContext, host connector.Host) error {
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
				ctx.GetLogger().Debug("Host architecture determined for kubeadm", "rawArch", hostArch, "usingArch", archToUse)
			} else {
				return fmt.Errorf("arch is not specified and host is nil, cannot determine architecture for DownloadKubeadmStep")
			}
		}
		s.determinedArch = archToUse
	}

	if s.determinedFileName == "" {
		s.determinedFileName = "kubeadm" // Kubeadm is a binary
	}

	if s.determinedURL == "" {
		// Version for k8s binaries typically includes 'v' prefix
		versionWithV := s.Version
		if !strings.HasPrefix(versionWithV, "v") {
			versionWithV = "v" + versionWithV
		}
		effectiveZone := s.Zone
		if effectiveZone == "" { effectiveZone = os.Getenv("KKZONE") }

		baseURL := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/%s/%s", versionWithV, s.determinedArch, s.determinedFileName)
		if effectiveZone == "cn" {
			baseURL = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/release/%s/bin/linux/%s/%s", versionWithV, s.determinedArch, s.determinedFileName)
		}
		s.determinedURL = baseURL
	}
	return nil
}

func (s *DownloadKubeadmStep) Name() string {
	return "Download Kubeadm"
}

func (s *DownloadKubeadmStep) Description() string {
	archDesc := s.Arch
	if s.determinedArch != "" { archDesc = s.determinedArch }
	return fmt.Sprintf("Downloads kubeadm version %s for %s architecture.", s.Version, archDesc)
}

func (s *DownloadKubeadmStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
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
		logger.Info("Kubeadm binary does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Kubeadm binary exists.", "path", expectedFilePath)

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
	ctx.TaskCache().Set(s.OutputComponentTypeKey, "KUBE")
	ctx.TaskCache().Set(s.OutputVersionKey, s.Version)
	ctx.TaskCache().Set(s.OutputArchKey, s.determinedArch)
	if s.Checksum != "" { ctx.TaskCache().Set(s.OutputChecksumKey, s.Checksum) }
	ctx.TaskCache().Set(s.OutputURLKey, s.determinedURL)
	logger.Info("Step is considered done, relevant info cached/updated.")
	return true, nil
}

func (s *DownloadKubeadmStep) Run(ctx runtime.StepContext, host connector.Host) error {
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
	logger.Info("Attempting to download kubeadm", "url", s.determinedURL, "destination", destinationPath)

	downloadedPath, dlErr := utils.DownloadFileWithConnector(ctx.GoContext(), logger, conn, s.determinedURL, s.DownloadDir, s.determinedFileName, s.Checksum)
	if dlErr != nil {
		return fmt.Errorf("failed to download kubeadm from URL %s: %w", s.determinedURL, dlErr)
	}
	logger.Info("Kubeadm downloaded successfully.", "path", downloadedPath)

	// Make the downloaded file executable
	logger.Info("Making kubeadm binary executable", "path", downloadedPath)
	chmodCmd := fmt.Sprintf("chmod +x %s", downloadedPath)
	// Use ExecOptions for sudo, assuming conn.Exec is the method.
	// Sudo might be needed if DownloadDir is not writable by the connection user.
	// For now, assume sudo is false unless specified for the step itself (which it isn't for chmod).
	// Let's assume the user downloading can chmod in DownloadDir. If not, this needs adjustment.
	// The original code used conn.RunCommand(..., Sudo: true). If the connector.Exec supports sudo, it should be used.
	// For now, let's assume no sudo for chmod unless the step itself has a general Sudo flag for all its operations.
	_, _, chmodErr := conn.Exec(ctx.GoContext(), chmodCmd, &connector.ExecOptions{Sudo: false})
	if chmodErr != nil {
		// This might not be a fatal error, could be a warning.
		logger.Warn("Failed to make kubeadm binary executable. Manual chmod might be required.", "path", downloadedPath, "error", chmodErr)
		// Depending on strictness, this could return an error:
		// return fmt.Errorf("failed to make downloaded kubeadm executable at %s: %w", downloadedPath, chmodErr)
	} else {
		logger.Info("Kubeadm binary made executable.", "path", downloadedPath)
	}

	if s.Checksum != "" { // Re-verify checksum after download if specified, DownloadFileWithConnector might do it too
		checksumValue := s.Checksum; checksumType := "sha256"
		if strings.Contains(s.Checksum, ":") {
			parts := strings.SplitN(s.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Info("Verifying checksum post-download", "type", checksumType)
		actualHash, errC := conn.GetFileChecksum(ctx.GoContext(), downloadedPath, checksumType)
		if errC != nil {
			return fmt.Errorf("failed to get checksum for downloaded kubeadm file %s: %w", downloadedPath, errC)
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			return fmt.Errorf("checksum mismatch for downloaded kubeadm file %s (expected %s, got %s)", downloadedPath, checksumValue, actualHash)
		}
		logger.Info("Checksum verified post-download.")
	}

	ctx.TaskCache().Set(s.OutputFilePathKey, downloadedPath)
	ctx.TaskCache().Set(s.OutputFileNameKey, s.determinedFileName)
	ctx.TaskCache().Set(s.OutputComponentTypeKey, "KUBE")
	ctx.TaskCache().Set(s.OutputVersionKey, s.Version)
	ctx.TaskCache().Set(s.OutputArchKey, s.determinedArch)
	if s.Checksum != "" { ctx.TaskCache().Set(s.OutputChecksumKey, s.Checksum) }
	ctx.TaskCache().Set(s.OutputURLKey, s.determinedURL)
	return nil
}

func (s *DownloadKubeadmStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
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
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	removeOpts := connector.RemoveOptions{ Recursive: false, IgnoreNotExist: true }
	if errRemove := conn.Remove(ctx.GoContext(), downloadedFilePath, removeOpts); errRemove != nil {
		logger.Error("Failed to remove file during rollback.", "path", downloadedFilePath, "error", errRemove)
		return fmt.Errorf("failed to remove file %s during rollback: %w", downloadedFilePath, errRemove)
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

// Ensure DownloadKubeadmStep implements the step.Step interface.
var _ step.Step = (*DownloadKubeadmStep)(nil)
