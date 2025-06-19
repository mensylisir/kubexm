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
	KubeletDownloadedPathKey     = "KubeletDownloadedPath"
	KubeletDownloadedFileNameKey = "KubeletDownloadedFileName"
	KubeletComponentTypeKey      = "KubeletComponentType"
	KubeletVersionKey            = "KubeletVersion"
	KubeletArchKey               = "KubeletArch"
	KubeletChecksumKey           = "KubeletChecksum"
	KubeletDownloadURLKey        = "KubeletDownloadURL"
)

// DownloadKubeletStepSpec defines parameters for downloading kubelet.
type DownloadKubeletStepSpec struct {
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
func (s *DownloadKubeletStepSpec) GetName() string {
	return "Download kubelet"
}

// PopulateDefaults sets default values.
func (s *DownloadKubeletStepSpec) PopulateDefaults(ctx *runtime.Context) {
	if s.Arch == "" && ctx != nil && ctx.Host != nil {
		s.Arch = ctx.Host.Arch()
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
	// 	s.DownloadDir = filepath.Join(defaultBaseDownloadDir, "kube")
	// }
	if s.OutputFilePathKey == "" {s.OutputFilePathKey = KubeletDownloadedPathKey}
	if s.OutputFileNameKey == "" {s.OutputFileNameKey = KubeletDownloadedFileNameKey}
	if s.OutputComponentTypeKey == "" {s.OutputComponentTypeKey = KubeletComponentTypeKey}
	if s.OutputVersionKey == "" {s.OutputVersionKey = KubeletVersionKey}
	if s.OutputArchKey == "" {s.OutputArchKey = KubeletArchKey}
	if s.OutputChecksumKey == "" {s.OutputChecksumKey = KubeletChecksumKey}
	if s.OutputURLKey == "" {s.OutputURLKey = KubeletDownloadURLKey}
}

// DownloadKubeletStepExecutor implements the download logic for kubelet.
type DownloadKubeletStepExecutor struct{}

func (e *DownloadKubeletStepExecutor) determineKubeletFileName(version, arch string) string {
	return "kubelet" // Similar to kubeadm, the binary itself is just "kubelet"
}

func (e *DownloadKubeletStepExecutor) determineKubeletURL(version, arch, zone string) string {
	url := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/%s/kubelet", version, arch)
	if zone == "cn" {
		url = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/release/%s/bin/linux/%s/kubelet", version, arch)
	}
	return url
}

// Check sees if kubelet already exists and optionally verifies checksum.
func (e *DownloadKubeletStepExecutor) Check(ctx runtime.Context) (bool, error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for DownloadKubeletStep Check")
	}
	stepSpec, ok := currentFullSpec.(*DownloadKubeletStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for DownloadKubeletStep Check: %T", currentFullSpec)
	}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", stepSpec.GetName())

	fileName := e.determineKubeletFileName(stepSpec.Version, stepSpec.Arch)
	expectedFilePath := filepath.Join(stepSpec.DownloadDir, fileName)

	exists, err := ctx.Host.Runner.Exists(ctx.GoContext, expectedFilePath)
	if err != nil {return false, fmt.Errorf("failed to check existence of %s: %w", expectedFilePath, err)}
	if !exists {
		logger.Infof("Kubelet binary %s does not exist.", expectedFilePath)
		return false, nil
	}
	logger.Infof("Kubelet binary %s exists.", expectedFilePath)

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
	url := e.determineKubeletURL(stepSpec.Version, stepSpec.Arch, stepSpec.Zone)
	ctx.Task().Set(stepSpec.OutputURLKey, url)
	return true, nil
}

// Execute downloads kubelet.
func (e *DownloadKubeletStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for DownloadKubeletStep Execute"))
	}
	stepSpec, ok := currentFullSpec.(*DownloadKubeletStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for DownloadKubeletStep Execute: %T", currentFullSpec))
	}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", stepSpec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	fileName := e.determineKubeletFileName(stepSpec.Version, stepSpec.Arch)
	componentType := "KUBE"
	effectiveZone := stepSpec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")}
	url := e.determineKubeletURL(stepSpec.Version, stepSpec.Arch, effectiveZone)

	logger.Infof("Attempting to download kubelet from %s to %s/%s", url, stepSpec.DownloadDir, fileName)

	downloadedPath, err := utils.DownloadFile(ctx, url, stepSpec.DownloadDir, fileName, false, stepSpec.Checksum, "sha256")
	if err != nil {
		res.Error = fmt.Errorf("failed to download kubelet: %w", err)
		res.Status = step.StatusFailed; return res
	}
	logger.Successf("Kubelet downloaded successfully to %s", downloadedPath)

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
	step.Register(&DownloadKubeletStepSpec{}, &DownloadKubeletStepExecutor{})
}
