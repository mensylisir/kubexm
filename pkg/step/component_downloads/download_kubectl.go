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
	KubectlDownloadedPathKey     = "KubectlDownloadedPath"
	KubectlDownloadedFileNameKey = "KubectlDownloadedFileName"
	KubectlComponentTypeKey      = "KubectlComponentType"
	KubectlVersionKey            = "KubectlVersion"
	KubectlArchKey               = "KubectlArch"
	KubectlChecksumKey           = "KubectlChecksum"
	KubectlDownloadURLKey        = "KubectlDownloadURL"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
)

// DownloadKubectlStepSpec downloads the kubectl binary.
type DownloadKubectlStepSpec struct {
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
	// Internal fields
	determinedArch     string
	determinedFileName string
	determinedURL      string
}

// NewDownloadKubectlStepSpec creates a new DownloadKubectlStepSpec.
func NewDownloadKubectlStep(
	version, arch, zone, downloadDir, checksum string,
	outputFilePathKey, outputFileNameKey, outputComponentTypeKey,
	outputVersionKey, outputArchKey, outputChecksumKey, outputURLKey string,
) *DownloadKubectlStepSpec {
	name := "Download Kubectl" // Default name
	description := fmt.Sprintf("Downloads kubectl version %s for %s architecture.", version, arch)

	if outputFilePathKey == "" { outputFilePathKey = KubectlDownloadedPathKey }
	if outputFileNameKey == "" { outputFileNameKey = KubectlDownloadedFileNameKey }
	if outputComponentTypeKey == "" { outputComponentTypeKey = KubectlComponentTypeKey }
	if outputVersionKey == "" { outputVersionKey = KubectlVersionKey }
	if outputArchKey == "" { outputArchKey = KubectlArchKey }
	if outputChecksumKey == "" { outputChecksumKey = KubectlChecksumKey }
	if outputURLKey == "" { outputURLKey = KubectlDownloadURLKey }

	return &DownloadKubectlStepSpec{
		StepMeta: spec.StepMeta{
			Name:        name,
			Description: description, // Will be updated by populateAndDetermineInternals
		},
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

func (s *DownloadKubectlStepSpec) populateAndDetermineInternals(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName())
	if s.determinedArch == "" {
		archToUse := s.Arch
		if archToUse == "" {
			if host != nil {
				hostArch := host.GetArch()
				if hostArch == "x86_64" { archToUse = "amd64"
				} else if hostArch == "aarch64" { archToUse = "arm64"
				} else { archToUse = hostArch }
				logger.Debug("Host architecture determined for kubectl", "rawArch", hostArch, "usingArch", archToUse)
			} else {
				return fmt.Errorf("arch is not specified and host is nil, cannot determine architecture for %s", s.GetName())
			}
		}
		s.determinedArch = archToUse
	}

	if s.determinedFileName == "" {
		s.determinedFileName = "kubectl" // Kubectl is a binary
	}

	if s.determinedURL == "" {
		versionWithV := s.Version
		if !strings.HasPrefix(versionWithV, "v") { versionWithV = "v" + versionWithV }
		effectiveZone := s.Zone
		if effectiveZone == "" { effectiveZone = os.Getenv("KKZONE") } // Consider passing zone via config

		baseURL := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/%s/%s", versionWithV, s.determinedArch, s.determinedFileName)
		if effectiveZone == "cn" {
			baseURL = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/release/%s/bin/linux/%s/%s", versionWithV, s.determinedArch, s.determinedFileName)
		}
		s.determinedURL = baseURL
	}
	// Update StepMeta description with determined values
	s.StepMeta.Description = fmt.Sprintf("Downloads kubectl version %s for %s architecture from %s.", s.Version, s.determinedArch, s.determinedURL)
	return nil
}

// Name returns the step's name (implementing step.Step).
func (s *DownloadKubectlStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *DownloadKubectlStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *DownloadKubectlStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *DownloadKubectlStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *DownloadKubectlStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *DownloadKubectlStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *DownloadKubectlStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
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
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), expectedFilePath)
	if err != nil {
		logger.Warn("Failed to check existence of target file, proceeding to Run phase.", "path", expectedFilePath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("Kubectl binary does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Kubectl binary exists.", "path", expectedFilePath)

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

func (s *DownloadKubectlStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
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
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	errMkdir := conn.Mkdir(ctx.GoContext(), s.DownloadDir, "0755") // Ensure Mkdir handles sudo if needed
	if errMkdir != nil {
		return fmt.Errorf("failed to create download directory %s: %w", s.DownloadDir, errMkdir)
	}

	destinationPath := filepath.Join(s.DownloadDir, s.determinedFileName)
	logger.Info("Attempting to download kubectl", "url", s.determinedURL, "destination", destinationPath)

	downloadedPath, dlErr := utils.DownloadFileWithConnector(ctx.GoContext(), logger, conn, s.determinedURL, s.DownloadDir, s.determinedFileName, s.Checksum) // Assuming this utility exists
	if dlErr != nil {
		return fmt.Errorf("failed to download kubectl from URL %s: %w", s.determinedURL, dlErr)
	}
	logger.Info("Kubectl downloaded successfully.", "path", downloadedPath)

	logger.Info("Making kubectl binary executable", "path", downloadedPath)
	chmodCmd := fmt.Sprintf("chmod +x %s", downloadedPath)
	_, _, chmodErr := conn.Exec(ctx.GoContext(), chmodCmd, &connector.ExecOptions{Sudo: utils.PathRequiresSudo(downloadedPath)})
	if chmodErr != nil {
		logger.Warn("Failed to make kubectl binary executable. Manual chmod might be required.", "path", downloadedPath, "error", chmodErr)
	} else {
		logger.Info("Kubectl binary made executable.", "path", downloadedPath)
	}

	if s.Checksum != "" {
		checksumValue := s.Checksum; checksumType := "sha256"
		if strings.Contains(s.Checksum, ":") {
			parts := strings.SplitN(s.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Info("Verifying checksum post-download", "type", checksumType)
		actualHash, errC := conn.GetFileChecksum(ctx.GoContext(), downloadedPath, checksumType) // Assuming this method exists
		if errC != nil {
			return fmt.Errorf("failed to get checksum for downloaded kubectl file %s: %w", downloadedPath, errC)
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			return fmt.Errorf("checksum mismatch for downloaded kubectl file %s (expected %s, got %s)", downloadedPath, checksumValue, actualHash)
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

func (s *DownloadKubectlStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	if err := s.populateAndDetermineInternals(ctx, host); err != nil { // This also updates description
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

// Ensure DownloadKubectlStepSpec implements the step.Step interface.
var _ step.Step = (*DownloadKubectlStepSpec)(nil)
