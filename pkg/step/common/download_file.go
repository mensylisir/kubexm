package common

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
	// Assuming constants might be centralized, e.g.:
	// "github.com/kubexms/kubexms/pkg/common/constants"
)

// Default SharedData keys that might be used by other steps to provide input to this one.
const (
	DefaultArtifactURLKey      = "artifactURL"
	DefaultArtifactFileNameKey = "artifactFileName"
	DefaultDownloadedFilePathKey = "downloadedFilePath"
)

// DownloadFileStepSpec defines parameters for downloading a file.
type DownloadFileStepSpec struct {
	SharedDataURLKey      string `json:"sharedDataURLKey,omitempty"`      // Key to retrieve download URL from SharedData
	SharedDataFileNameKey string `json:"sharedDataFileNameKey,omitempty"` // Key to retrieve filename from SharedData
	DownloadDir           string `json:"downloadDir,omitempty"`           // Directory to download the file to
	OutputFilePathKey     string `json:"outputFilePathKey,omitempty"`     // SharedData key to store the path of the downloaded file
	Checksum              string `json:"checksum,omitempty"`              // Expected checksum of the file (e.g., "sha256:actualhash" or just "actualhash")
	ChecksumType          string `json:"checksumType,omitempty"`          // Type of checksum: "sha256", "sha512", "md5". Auto-detected if Checksum field has type prefix.
}

// GetName returns the name of the step.
func (s *DownloadFileStepSpec) GetName() string {
	return "Download File"
}

// PopulateDefaults sets default values for the spec.
func (s *DownloadFileStepSpec) PopulateDefaults(ctx *runtime.Context) {
	if s.SharedDataURLKey == "" {
		s.SharedDataURLKey = DefaultArtifactURLKey
	}
	if s.SharedDataFileNameKey == "" {
		s.SharedDataFileNameKey = DefaultArtifactFileNameKey
	}
	if s.DownloadDir == "" {
		if ctx != nil && ctx.WorkDir != "" {
			s.DownloadDir = filepath.Join(ctx.WorkDir, "downloads") // Use context-specific workdir if available
		} else {
			s.DownloadDir = "/tmp/kubexms_downloads" // Fallback
		}
	}
	if s.OutputFilePathKey == "" {
		s.OutputFilePathKey = DefaultDownloadedFilePathKey
	}

	// Auto-detect checksum type from checksum string if not explicitly set
	if s.ChecksumType == "" && s.Checksum != "" {
		parts := strings.SplitN(s.Checksum, ":", 2)
		if len(parts) == 2 {
			algo := strings.ToLower(parts[0])
			if algo == "sha256" || algo == "sha512" || algo == "md5" {
				s.ChecksumType = algo
				s.Checksum = parts[1] // Store only the hash part
			}
		}
	}
	// Default to sha256 if type is still empty but checksum is provided
	if s.ChecksumType == "" && s.Checksum != "" {
		s.ChecksumType = "sha256"
	}
}

// DownloadFileStepExecutor implements the logic for downloading a file.
type DownloadFileStepExecutor struct{}

// getFileChecksum calculates the checksum of a file.
// This is a local helper; a production runner would have a robust, tested version.
func (e *DownloadFileStepExecutor) getFileChecksum(filePath string, checksumType string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s for checksum: %w", filePath, err)
	}
	defer file.Close()

	var h hash.Hash
	switch strings.ToLower(checksumType) {
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	case "md5":
		h = md5.New()
	default:
		return "", fmt.Errorf("unsupported checksum type: %s", checksumType)
	}

	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to calculate %s checksum for %s: %w", checksumType, filePath, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Check determines if the file already exists and matches checksum.
func (e *DownloadFileStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	stepSpec, ok := s.(*DownloadFileStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T", s)
	}
	stepSpec.PopulateDefaults(ctx)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepSpec.GetName()).Sugar()

	fileNameVal,fileNameOk := ctx.SharedData.Load(stepSpec.SharedDataFileNameKey)
	if !fileNameOk {
		hostCtxLogger.Debugf("Filename not found in SharedData key '%s'. Cannot check for existing file.", stepSpec.SharedDataFileNameKey)
		return false, nil // Cannot determine expected file, so not done.
	}
	fileName, ok := fileNameVal.(string)
	if !ok || fileName == "" {
		hostCtxLogger.Warnf("Invalid or empty filename in SharedData key '%s'. Cannot check.", stepSpec.SharedDataFileNameKey)
		return false, nil
	}

	expectedFilePath := filepath.Join(stepSpec.DownloadDir, fileName)
	hostCtxLogger.Debugf("Checking for existing file at: %s", expectedFilePath)

	// Check if file exists using runner.Exists (assumes sudo context is handled by runner if needed for path)
	// For /tmp or WorkDir, sudo is usually not needed.
	fileExists, err := ctx.Host.Runner.Exists(ctx.GoContext, expectedFilePath)
	if err != nil {
		return false, fmt.Errorf("error checking existence of %s: %w", expectedFilePath, err)
	}
	if !fileExists {
		hostCtxLogger.Infof("File %s does not exist. Download needed.", expectedFilePath)
		return false, nil
	}
	hostCtxLogger.Debugf("File %s exists.", expectedFilePath)

	if stepSpec.Checksum != "" {
		hostCtxLogger.Debugf("Verifying checksum for %s (type: %s, expected: %s)", expectedFilePath, stepSpec.ChecksumType, stepSpec.Checksum)
		// Assuming runner has GetFileChecksum(path, type) or use local helper
		// currentChecksum, err := ctx.Host.Runner.GetFileChecksum(ctx.GoContext, expectedFilePath, stepSpec.ChecksumType)
		currentChecksum, err := e.getFileChecksum(expectedFilePath, stepSpec.ChecksumType) // Using local helper
		if err != nil {
			hostCtxLogger.Warnf("Failed to calculate checksum for %s: %v. Assuming not OK.", expectedFilePath, err)
			return false, nil // Cannot verify, assume not done.
		}
		if !strings.EqualFold(currentChecksum, stepSpec.Checksum) {
			hostCtxLogger.Infof("Checksum mismatch for %s. Expected: %s, Got: %s. Download needed.",
				expectedFilePath, stepSpec.Checksum, currentChecksum)
			return false, nil
		}
		hostCtxLogger.Debugf("Checksum for %s matches expected value.", expectedFilePath)
	}

	// If file exists and checksum matches (or no checksum specified), it's done.
	// Store path in SharedData even in Check if found and valid, so subsequent steps can rely on it if Execute is skipped.
	ctx.SharedData.Store(stepSpec.OutputFilePathKey, expectedFilePath)
	hostCtxLogger.Infof("File %s already downloaded and checksum matches (if specified).", expectedFilePath)
	return true, nil
}

// Execute downloads the file.
func (e *DownloadFileStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepSpec, ok := s.(*DownloadFileStepSpec)
	if !ok {
		myErr := fmt.Errorf("Execute: unexpected spec type %T", s)
		return step.NewResult("DownloadFile (type error)", ctx.Host.Name, time.Now(), myErr)
	}
	stepSpec.PopulateDefaults(ctx)

	stepName := stepSpec.GetName()
	startTime := time.Now()
	res := step.NewResult(stepName, ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Sugar()

	urlVal, urlOk := ctx.SharedData.Load(stepSpec.SharedDataURLKey)
	if !urlOk {
		res.Error = fmt.Errorf("download URL not found in SharedData key '%s'", stepSpec.SharedDataURLKey)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	urlStr, ok := urlVal.(string)
	if !ok || urlStr == "" {
		res.Error = fmt.Errorf("invalid or empty download URL in SharedData key '%s'", stepSpec.SharedDataURLKey)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	fileNameVal, fileNameOk := ctx.SharedData.Load(stepSpec.SharedDataFileNameKey)
	if !fileNameOk {
		res.Error = fmt.Errorf("filename not found in SharedData key '%s'", stepSpec.SharedDataFileNameKey)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	fileName, ok := fileNameVal.(string)
	if !ok || fileName == "" {
		res.Error = fmt.Errorf("invalid or empty filename in SharedData key '%s'", stepSpec.SharedDataFileNameKey)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	hostCtxLogger.Infof("Ensuring download directory %s exists...", stepSpec.DownloadDir)
	// Mkdirp usually doesn't need sudo if parent is writable or path is in user space (/tmp, WorkDir)
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, stepSpec.DownloadDir, "0755", true); err != nil {
		res.Error = fmt.Errorf("failed to create download directory %s: %w", stepSpec.DownloadDir, err)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	downloadPath := filepath.Join(stepSpec.DownloadDir, fileName)
	hostCtxLogger.Infof("Downloading file from %s to %s...", urlStr, downloadPath)

	// Download (sudo typically false, especially for /tmp or workdir)
	if err := ctx.Host.Runner.Download(ctx.GoContext, urlStr, downloadPath, false); err != nil {
		res.Error = fmt.Errorf("failed to download file from %s to %s: %w", urlStr, downloadPath, err)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Infof("File downloaded successfully to %s.", downloadPath)

	if stepSpec.Checksum != "" {
		hostCtxLogger.Infof("Verifying checksum for downloaded file %s (type: %s)...", downloadPath, stepSpec.ChecksumType)
		// currentChecksum, err := ctx.Host.Runner.GetFileChecksum(ctx.GoContext, downloadPath, stepSpec.ChecksumType)
		currentChecksum, err := e.getFileChecksum(downloadPath, stepSpec.ChecksumType) // Using local helper
		if err != nil {
			res.Error = fmt.Errorf("failed to calculate checksum for %s: %w", downloadPath, err)
			res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}
		if !strings.EqualFold(currentChecksum, stepSpec.Checksum) {
			// Attempt to remove the invalid file
			ctx.Host.Runner.Remove(ctx.GoContext, downloadPath, false) // Sudo false for /tmp
			res.Error = fmt.Errorf("checksum mismatch for downloaded file %s. Expected: %s, Got: %s. File removed.",
				downloadPath, stepSpec.Checksum, currentChecksum)
			res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}
		hostCtxLogger.Infof("Checksum for %s verified successfully.", downloadPath)
	}

	ctx.SharedData.Store(stepSpec.OutputFilePathKey, downloadPath)
	hostCtxLogger.Debugf("Stored downloaded file path '%s' in SharedData key '%s'.", downloadPath, stepSpec.OutputFilePathKey)

	// Perform post-execution check
	done, checkErr := e.Check(s, ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed verification: %v", res.Error); return res
	}
	if !done {
		errMsg := "post-execution check indicates file download was not successful or file is invalid"
		res.Error = fmt.Errorf(errMsg)
		res.SetFailed(errMsg); hostCtxLogger.Errorf("Step failed verification: %s", errMsg); return res
	}

	res.SetSucceeded(fmt.Sprintf("File %s downloaded and verified successfully.", downloadPath))
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&DownloadFileStepSpec{}), &DownloadFileStepExecutor{})
}
