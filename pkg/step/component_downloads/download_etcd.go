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
	"github.com/kubexms/kubexms/pkg/utils" // For DownloadFile utility
	// Assuming constants might be centralized, e.g.:
	// "github.com/kubexms/kubexms/pkg/common/constants"
)

// Define SharedData keys for this component.
// These could be centralized in a constants package if preferred.
const (
	EtcdDownloadedPathKey      = "EtcdDownloadedPath"
	EtcdDownloadedFileNameKey  = "EtcdDownloadedFileName"
	EtcdComponentTypeKey       = "EtcdComponentType"     // To store "ETCD"
	EtcdVersionKey             = "EtcdVersion"           // To store spec.Version
	EtcdArchKey                = "EtcdArch"              // To store spec.Arch
	EtcdChecksumKey            = "EtcdChecksum"          // To store spec.Checksum if provided
	EtcdDownloadURLKey         = "EtcdDownloadURL"       // To store the calculated URL
)

// DownloadEtcdStepSpec defines parameters for downloading the etcd binary.
type DownloadEtcdStepSpec struct {
	Version              string `json:"version"` // e.g., "v3.5.0"
	Arch                 string `json:"arch"`    // e.g., "amd64", "arm64"
	Zone                 string `json:"zone,omitempty"`
	DownloadDir          string `json:"downloadDir,omitempty"`
	Checksum             string `json:"checksum,omitempty"` // e.g., "sha256:value" or just "value"
	OutputFilePathKey    string `json:"outputFilePathKey,omitempty"`
	OutputFileNameKey    string `json:"outputFileNameKey,omitempty"`
	OutputComponentTypeKey string `json:"outputComponentTypeKey,omitempty"`
	OutputVersionKey     string `json:"outputVersionKey,omitempty"`
	OutputArchKey        string `json:"outputArchKey,omitempty"`
	OutputChecksumKey    string `json:"outputChecksumKey,omitempty"`
	OutputURLKey         string `json:"outputURLKey,omitempty"`
}

// GetName returns the step name.
func (s *DownloadEtcdStepSpec) GetName() string {
	return "Download etcd"
}

// PopulateDefaults sets default values.
func (s *DownloadEtcdStepSpec) PopulateDefaults(ctx *runtime.Context) {
	if s.Arch == "" && ctx != nil && ctx.Host != nil {
		s.Arch = ctx.Host.Arch() // Default to host architecture
		// Normalize arch names if needed, e.g., x86_64 to amd64
		if s.Arch == "x86_64" {
			s.Arch = "amd64"
		} else if s.Arch == "aarch64" {
			s.Arch = "arm64"
		}
	}
	// DownloadDir is now expected to be set by the module.
	// if s.DownloadDir == "" {
	// 	defaultBaseDownloadDir := "/tmp/kubexms_downloads"
	// 	if ctx != nil && ctx.WorkDir != "" {
	// 		defaultBaseDownloadDir = filepath.Join(ctx.WorkDir, "downloads")
	// 	}
	// 	s.DownloadDir = filepath.Join(defaultBaseDownloadDir, "etcd")
	// }
	if s.OutputFilePathKey == "" {
		s.OutputFilePathKey = EtcdDownloadedPathKey
	}
	if s.OutputFileNameKey == "" {
		s.OutputFileNameKey = EtcdDownloadedFileNameKey
	}
	if s.OutputComponentTypeKey == "" {
		s.OutputComponentTypeKey = EtcdComponentTypeKey
	}
	if s.OutputVersionKey == "" {
		s.OutputVersionKey = EtcdVersionKey
	}
	if s.OutputArchKey == "" {
		s.OutputArchKey = EtcdArchKey
	}
	if s.OutputChecksumKey == "" {
		s.OutputChecksumKey = EtcdChecksumKey
	}
	if s.OutputURLKey == "" {
		s.OutputURLKey = EtcdDownloadURLKey
	}
}

// DownloadEtcdStepExecutor implements the download logic for etcd.
type DownloadEtcdStepExecutor struct{}

func (e *DownloadEtcdStepExecutor) determineEtcdFileName(version, arch string) string {
	return fmt.Sprintf("etcd-%s-linux-%s.tar.gz", version, arch)
}

func (e *DownloadEtcdStepExecutor) determineEtcdURL(version, arch, fileName, zone string) string {
	if zone == "cn" {
		return fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/etcd/release/download/%s/%s", version, fileName)
	}
	return fmt.Sprintf("https://github.com/coreos/etcd/releases/download/%s/%s", version, fileName)
}

// Check sees if the etcd tarball already exists and optionally verifies checksum.
func (e *DownloadEtcdStepExecutor) Check(ctx runtime.Context) (bool, error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for DownloadEtcdStep Check")
	}
	stepSpec, ok := currentFullSpec.(*DownloadEtcdStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for DownloadEtcdStep Check: %T", currentFullSpec)
	}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", stepSpec.GetName())

	fileName := e.determineEtcdFileName(stepSpec.Version, stepSpec.Arch)
	expectedFilePath := filepath.Join(stepSpec.DownloadDir, fileName)

	exists, err := ctx.Host.Runner.Exists(ctx.GoContext, expectedFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of %s: %w", expectedFilePath, err)
	}
	if !exists {
		logger.Infof("Etcd archive %s does not exist.", expectedFilePath)
		return false, nil
	}
	logger.Infof("Etcd archive %s exists.", expectedFilePath)

	if stepSpec.Checksum != "" {
		checksumValue := stepSpec.Checksum
		checksumType := "sha256" // Default or parse from Checksum string
		if strings.Contains(stepSpec.Checksum, ":") {
			parts := strings.SplitN(stepSpec.Checksum, ":", 2)
			checksumType = parts[0]
			checksumValue = parts[1]
		}
		logger.Infof("Verifying %s checksum for %s", checksumType, expectedFilePath)
		actualHash, err := ctx.Host.Runner.GetFileChecksum(ctx.GoContext, expectedFilePath, checksumType)
		if err != nil {
			logger.Warnf("Failed to get %s checksum for %s: %v. Assuming invalid.", checksumType, expectedFilePath, err)
			return false, nil // Cannot verify, assume not OK
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			logger.Warnf("%s checksum mismatch for %s. Expected: %s, Got: %s.", checksumType, expectedFilePath, checksumValue, actualHash)
			return false, nil // Checksum mismatch
		}
		logger.Infof("%s checksum for %s verified.", checksumType, expectedFilePath)
	}

	// If file exists and checksum matches (or no checksum specified), it's done.
	// Store details in Task Cache for subsequent steps even if Execute is skipped.
	ctx.Task().Set(stepSpec.OutputFilePathKey, expectedFilePath)
	ctx.Task().Set(stepSpec.OutputFileNameKey, fileName)
	ctx.Task().Set(stepSpec.OutputComponentTypeKey, "ETCD")
	ctx.Task().Set(stepSpec.OutputVersionKey, stepSpec.Version)
	ctx.Task().Set(stepSpec.OutputArchKey, stepSpec.Arch)
	if stepSpec.Checksum != "" {
		ctx.Task().Set(stepSpec.OutputChecksumKey, stepSpec.Checksum)
	}
	url := e.determineEtcdURL(stepSpec.Version, stepSpec.Arch, fileName, stepSpec.Zone)
	ctx.Task().Set(stepSpec.OutputURLKey, url)

	return true, nil
}

// Execute downloads the etcd tarball.
func (e *DownloadEtcdStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for DownloadEtcdStep Execute"))
	}
	stepSpec, ok := currentFullSpec.(*DownloadEtcdStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for DownloadEtcdStep Execute: %T", currentFullSpec))
	}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", stepSpec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	fileName := e.determineEtcdFileName(stepSpec.Version, stepSpec.Arch)
	componentType := "ETCD"
	effectiveZone := stepSpec.Zone
	if effectiveZone == "" {
		effectiveZone = os.Getenv("KKZONE")
	}
	url := e.determineEtcdURL(stepSpec.Version, stepSpec.Arch, fileName, effectiveZone)

	logger.Infof("Attempting to download etcd from %s to %s/%s", url, stepSpec.DownloadDir, fileName)

	downloadedPath, err := utils.DownloadFile(ctx, url, stepSpec.DownloadDir, fileName, false, stepSpec.Checksum, "sha256")
	if err != nil {
		res.Error = fmt.Errorf("failed to download etcd: %w", err)
		res.Status = step.StatusFailed
		return res
	}
	logger.Successf("Etcd downloaded successfully to %s", downloadedPath)

	ctx.Task().Set(stepSpec.OutputFilePathKey, downloadedPath)
	ctx.Task().Set(stepSpec.OutputFileNameKey, fileName)
	ctx.Task().Set(stepSpec.OutputComponentTypeKey, componentType)
	ctx.Task().Set(stepSpec.OutputVersionKey, stepSpec.Version)
	ctx.Task().Set(stepSpec.OutputArchKey, stepSpec.Arch)
	if stepSpec.Checksum != "" {
		ctx.Task().Set(stepSpec.OutputChecksumKey, stepSpec.Checksum)
	}
	ctx.Task().Set(stepSpec.OutputURLKey, url)

	// res.SetSucceeded() // Status is set by NewResult if err is nil
	return res
}

func init() {
	step.Register(&DownloadEtcdStepSpec{}, &DownloadEtcdStepExecutor{})
}
