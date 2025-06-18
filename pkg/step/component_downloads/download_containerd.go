package component_downloads

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
	"github.com/kubexms/kubexms/pkg/utils"
)

const (
	ContainerdDownloadedPathKey     = "ContainerdDownloadedPath"
	ContainerdDownloadedFileNameKey = "ContainerdDownloadedFileName"
	ContainerdComponentTypeKey      = "ContainerdComponentType"
	ContainerdVersionKey            = "ContainerdVersion"
	ContainerdArchKey               = "ContainerdArch"
	ContainerdChecksumKey           = "ContainerdChecksum"
	ContainerdDownloadURLKey        = "ContainerdDownloadURL"
)

// DownloadContainerdStepSpec defines parameters for downloading containerd.
type DownloadContainerdStepSpec struct {
	Version              string `json:"version"` // e.g., "1.6.4" (no 'v' prefix usually for containerd file names)
	Arch                 string `json:"arch"`
	Zone                 string `json:"zone,omitempty"`
	DownloadDir          string `json:"downloadDir,omitempty"`
	Checksum             string `json:"checksum,omitempty"`
	OutputFilePathKey    string `json:"outputFilePathKey,omitempty"`
	OutputFileNameKey    string `json:"outputFileNameKey,omitempty"`
	OutputComponentTypeKey string `json:"outputComponentTypeKey,omitempty"`
	OutputVersionKey     string `json:"outputVersionKey,omitempty"`
	OutputArchKey        string `json:"outputArchKey,omitempty"`
	OutputChecksumKey    string `json:"outputChecksumKey,omitempty"`
	OutputURLKey         string `json:"outputURLKey,omitempty"`
}

// GetName returns the step name.
func (s *DownloadContainerdStepSpec) GetName() string {
	return "Download containerd"
}

// PopulateDefaults sets default values.
func (s *DownloadContainerdStepSpec) PopulateDefaults(ctx *runtime.Context) {
	if s.Arch == "" && ctx != nil && ctx.Host != nil {
		s.Arch = ctx.Host.Arch()
		if s.Arch == "x86_64" {
			s.Arch = "amd64"
		} else if s.Arch == "aarch64" {
			s.Arch = "arm64"
		}
	}
	if s.DownloadDir == "" {
		defaultBaseDownloadDir := "/tmp/kubexms_downloads"
		if ctx != nil && ctx.WorkDir != "" {
			defaultBaseDownloadDir = filepath.Join(ctx.WorkDir, "downloads")
		}
		s.DownloadDir = filepath.Join(defaultBaseDownloadDir, "containerd")
	}

	if s.OutputFilePathKey == "" {s.OutputFilePathKey = ContainerdDownloadedPathKey}
	if s.OutputFileNameKey == "" {s.OutputFileNameKey = ContainerdDownloadedFileNameKey}
	if s.OutputComponentTypeKey == "" {s.OutputComponentTypeKey = ContainerdComponentTypeKey}
	if s.OutputVersionKey == "" {s.OutputVersionKey = ContainerdVersionKey}
	if s.OutputArchKey == "" {s.OutputArchKey = ContainerdArchKey}
	if s.OutputChecksumKey == "" {s.OutputChecksumKey = ContainerdChecksumKey}
	if s.OutputURLKey == "" {s.OutputURLKey = ContainerdDownloadURLKey}
}

// DownloadContainerdStepExecutor implements the download logic for containerd.
type DownloadContainerdStepExecutor struct{}

func (e *DownloadContainerdStepExecutor) determineContainerdFileName(version, arch string) string {
	// Containerd release files usually don't have 'v' in version string.
	return fmt.Sprintf("containerd-%s-linux-%s.tar.gz", strings.TrimPrefix(version, "v"), arch)
}

func (e *DownloadContainerdStepExecutor) determineContainerdURL(version, arch, fileName, zone string) string {
	// URL usually requires 'v' prefix for version.
	versionWithV := version
	if !strings.HasPrefix(versionWithV, "v") {
		versionWithV = "v" + versionWithV
	}
	url := fmt.Sprintf("https://github.com/containerd/containerd/releases/download/%s/%s", versionWithV, fileName)
	if zone == "cn" {
		// Example CN mirror, adjust if official one is known
		url = fmt.Sprintf("https://containerd-release.pek3b.qingstor.com/containerd/%s/%s", versionWithV, fileName)
	}
	return url
}

// Check sees if containerd tarball already exists and optionally verifies checksum.
func (e *DownloadContainerdStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (bool, error) {
	stepSpec, ok := s.(*DownloadContainerdStepSpec)
	if !ok {return false, fmt.Errorf("unexpected spec type %T", s)}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", stepSpec.GetName())

	fileName := e.determineContainerdFileName(stepSpec.Version, stepSpec.Arch)
	expectedFilePath := filepath.Join(stepSpec.DownloadDir, fileName)

	exists, err := ctx.Host.Runner.Exists(ctx.GoContext, expectedFilePath)
	if err != nil {return false, fmt.Errorf("failed to check existence of %s: %w", expectedFilePath, err)}
	if !exists {
		logger.Infof("Containerd archive %s does not exist.", expectedFilePath)
		return false, nil
	}
	logger.Infof("Containerd archive %s exists.", expectedFilePath)

	if stepSpec.Checksum != "" {
		checksumValue := stepSpec.Checksum; checksumType := "sha256"
		if strings.Contains(stepSpec.Checksum, ":") {
			parts := strings.SplitN(stepSpec.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Infof("Verifying %s checksum for %s", checksumType, expectedFilePath)
		actualHash, err := ctx.Host.Runner.GetFileChecksum(ctx.GoContext, expectedFilePath, checksumType)
		if err != nil {
			logger.Warnf("Failed to get %s checksum for %s: %v. Assuming invalid.", checksumType, expectedFilePath, err)
			return false, nil
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			logger.Warnf("%s checksum mismatch for %s. Expected: %s, Got: %s.", checksumType, expectedFilePath, checksumValue, actualHash)
			return false, nil
		}
		logger.Infof("%s checksum for %s verified.", checksumType, expectedFilePath)
	}

	ctx.SharedData.Store(stepSpec.OutputFilePathKey, expectedFilePath)
	ctx.SharedData.Store(stepSpec.OutputFileNameKey, fileName)
	ctx.SharedData.Store(stepSpec.OutputComponentTypeKey, "CONTAINER_RUNTIME") // Changed from "CONTAINERD" to "CONTAINER_RUNTIME"
	ctx.SharedData.Store(stepSpec.OutputVersionKey, stepSpec.Version)
	ctx.SharedData.Store(stepSpec.OutputArchKey, stepSpec.Arch)
	if stepSpec.Checksum != "" {ctx.SharedData.Store(stepSpec.OutputChecksumKey, stepSpec.Checksum)}
	url := e.determineContainerdURL(stepSpec.Version, stepSpec.Arch, fileName, stepSpec.Zone)
	ctx.SharedData.Store(stepSpec.OutputURLKey, url)
	return true, nil
}

// Execute downloads containerd.
func (e *DownloadContainerdStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepSpec, ok := s.(*DownloadContainerdStepSpec)
	if !ok {return step.NewResultForSpec(s, fmt.Errorf("unexpected spec type %T", s))}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", stepSpec.GetName())
	res := step.NewResultForSpec(s, nil)

	fileName := e.determineContainerdFileName(stepSpec.Version, stepSpec.Arch)
	componentType := "CONTAINER_RUNTIME" // Changed from "CONTAINERD"
	effectiveZone := stepSpec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")}
	url := e.determineContainerdURL(stepSpec.Version, stepSpec.Arch, fileName, effectiveZone)

	logger.Infof("Attempting to download containerd from %s to %s/%s", url, stepSpec.DownloadDir, fileName)

	downloadedPath, err := utils.DownloadFile(ctx, url, stepSpec.DownloadDir, fileName, false, stepSpec.Checksum, "sha256")
	if err != nil {
		res.Error = fmt.Errorf("failed to download containerd: %w", err)
		res.SetFailed(); return res
	}
	logger.Successf("Containerd downloaded successfully to %s", downloadedPath)

	ctx.SharedData.Store(stepSpec.OutputFilePathKey, downloadedPath)
	ctx.SharedData.Store(stepSpec.OutputFileNameKey, fileName)
	ctx.SharedData.Store(stepSpec.OutputComponentTypeKey, componentType)
	ctx.SharedData.Store(stepSpec.OutputVersionKey, stepSpec.Version)
	ctx.SharedData.Store(stepSpec.OutputArchKey, stepSpec.Arch)
	if stepSpec.Checksum != "" {ctx.SharedData.Store(stepSpec.OutputChecksumKey, stepSpec.Checksum)}
	ctx.SharedData.Store(stepSpec.OutputURLKey, url)

	res.SetSucceeded(); return res
}

func init() {
	step.Register(&DownloadContainerdStepSpec{}, &DownloadContainerdStepExecutor{})
}
