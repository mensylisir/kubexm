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
	KubeadmDownloadedPathKey     = "KubeadmDownloadedPath"
	KubeadmDownloadedFileNameKey = "KubeadmDownloadedFileName"
	KubeadmComponentTypeKey      = "KubeadmComponentType"
	KubeadmVersionKey            = "KubeadmVersion"
	KubeadmArchKey               = "KubeadmArch"
	KubeadmChecksumKey           = "KubeadmChecksum"
	KubeadmDownloadURLKey        = "KubeadmDownloadURL"
)

// DownloadKubeadmStepSpec defines parameters for downloading kubeadm.
type DownloadKubeadmStepSpec struct {
	Version              string `json:"version"` // e.g., "v1.23.5"
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
func (s *DownloadKubeadmStepSpec) GetName() string {
	return "Download kubeadm"
}

// PopulateDefaults sets default values.
func (s *DownloadKubeadmStepSpec) PopulateDefaults(ctx *runtime.Context) {
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
		s.DownloadDir = filepath.Join(defaultBaseDownloadDir, "kube") // Group KUBE components
	}
	if s.OutputFilePathKey == "" {s.OutputFilePathKey = KubeadmDownloadedPathKey}
	if s.OutputFileNameKey == "" {s.OutputFileNameKey = KubeadmDownloadedFileNameKey}
	if s.OutputComponentTypeKey == "" {s.OutputComponentTypeKey = KubeadmComponentTypeKey}
	if s.OutputVersionKey == "" {s.OutputVersionKey = KubeadmVersionKey}
	if s.OutputArchKey == "" {s.OutputArchKey = KubeadmArchKey}
	if s.OutputChecksumKey == "" {s.OutputChecksumKey = KubeadmChecksumKey}
	if s.OutputURLKey == "" {s.OutputURLKey = KubeadmDownloadURLKey}
}

// DownloadKubeadmStepExecutor implements the download logic for kubeadm.
type DownloadKubeadmStepExecutor struct{}

func (e *DownloadKubeadmStepExecutor) determineKubeadmFileName(version, arch string) string {
	// Kubeadm binary typically doesn't include version or arch in its filename post-download.
	// The downloaded file from official sources might have version/arch, but is renamed.
	// For consistency with how it's used, the filename is just "kubeadm".
	return "kubeadm"
}

func (e *DownloadKubeadmStepExecutor) determineKubeadmURL(version, arch, zone string) string {
	// The actual downloaded file is just 'kubeadm', not 'kubeadm-version-linux-arch'
	url := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/%s/kubeadm", version, arch)
	if zone == "cn" {
		url = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/release/%s/bin/linux/%s/kubeadm", version, arch)
	}
	return url
}

// Check sees if kubeadm already exists and optionally verifies checksum.
func (e *DownloadKubeadmStepExecutor) Check(ctx runtime.Context) (bool, error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for DownloadKubeadmStep Check")
	}
	stepSpec, ok := currentFullSpec.(*DownloadKubeadmStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for DownloadKubeadmStep Check: %T", currentFullSpec)
	}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", stepSpec.GetName())

	fileName := e.determineKubeadmFileName(stepSpec.Version, stepSpec.Arch)
	expectedFilePath := filepath.Join(stepSpec.DownloadDir, fileName)

	exists, err := ctx.Host.Runner.Exists(ctx.GoContext, expectedFilePath)
	if err != nil {return false, fmt.Errorf("failed to check existence of %s: %w", expectedFilePath, err)}
	if !exists {
		logger.Infof("Kubeadm binary %s does not exist.", expectedFilePath)
		return false, nil
	}
	logger.Infof("Kubeadm binary %s exists.", expectedFilePath)

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

	ctx.Task().Set(stepSpec.OutputFilePathKey, expectedFilePath)
	ctx.Task().Set(stepSpec.OutputFileNameKey, fileName)
	ctx.Task().Set(stepSpec.OutputComponentTypeKey, "KUBE")
	ctx.Task().Set(stepSpec.OutputVersionKey, stepSpec.Version)
	ctx.Task().Set(stepSpec.OutputArchKey, stepSpec.Arch)
	if stepSpec.Checksum != "" {ctx.Task().Set(stepSpec.OutputChecksumKey, stepSpec.Checksum)}
	url := e.determineKubeadmURL(stepSpec.Version, stepSpec.Arch, stepSpec.Zone)
	ctx.Task().Set(stepSpec.OutputURLKey, url)
	return true, nil
}

// Execute downloads kubeadm.
func (e *DownloadKubeadmStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for DownloadKubeadmStep Execute"))
	}
	stepSpec, ok := currentFullSpec.(*DownloadKubeadmStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for DownloadKubeadmStep Execute: %T", currentFullSpec))
	}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", stepSpec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	// Note: The downloaded filename from GCS is "kubeadm", but the effective filename on disk after download
	// for storage might be "kubeadm-<version>-<arch>" if we chose to rename it.
	// Here, determineKubeadmFileName returns "kubeadm", so that's the target name.
	fileName := e.determineKubeadmFileName(stepSpec.Version, stepSpec.Arch)
	componentType := "KUBE"
	effectiveZone := stepSpec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")}
	url := e.determineKubeadmURL(stepSpec.Version, stepSpec.Arch, effectiveZone)

	logger.Infof("Attempting to download kubeadm from %s to %s/%s", url, stepSpec.DownloadDir, fileName)

	downloadedPath, err := utils.DownloadFile(ctx, url, stepSpec.DownloadDir, fileName, false, stepSpec.Checksum, "sha256")
	if err != nil {
		res.Error = fmt.Errorf("failed to download kubeadm: %w", err)
		res.Status = step.StatusFailed; return res
	}
	logger.Successf("Kubeadm downloaded successfully to %s", downloadedPath)

	ctx.Task().Set(stepSpec.OutputFilePathKey, downloadedPath)
	ctx.Task().Set(stepSpec.OutputFileNameKey, fileName)
	ctx.Task().Set(stepSpec.OutputComponentTypeKey, componentType)
	ctx.Task().Set(stepSpec.OutputVersionKey, stepSpec.Version)
	ctx.Task().Set(stepSpec.OutputArchKey, stepSpec.Arch)
	if stepSpec.Checksum != "" {ctx.Task().Set(stepSpec.OutputChecksumKey, stepSpec.Checksum)}
	ctx.Task().Set(stepSpec.OutputURLKey, url)

	// res.SetSucceeded(); // Status is set by NewResult if err is nil
	return res
}

func init() {
	step.Register(&DownloadKubeadmStepSpec{}, &DownloadKubeadmStepExecutor{})
}
