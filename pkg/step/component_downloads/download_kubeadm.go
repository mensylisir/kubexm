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

type DownloadKubeadmStepSpec struct {
	Version              string `json:"version"`
	Arch                 string `json:"arch"`
	Zone                 string `json:"zone,omitempty"`
	DownloadDir          string `json:"downloadDir,omitempty"` // Expected to be set by module
	Checksum             string `json:"checksum,omitempty"`
	OutputFilePathKey    string `json:"outputFilePathKey,omitempty"`
	OutputFileNameKey    string `json:"outputFileNameKey,omitempty"`
	OutputComponentTypeKey string `json:"outputComponentTypeKey,omitempty"`
	OutputVersionKey     string `json:"outputVersionKey,omitempty"`
	OutputArchKey        string `json:"outputArchKey,omitempty"`
	OutputChecksumKey    string `json:"outputChecksumKey,omitempty"`
	OutputURLKey         string `json:"outputURLKey,omitempty"`
}

func (s *DownloadKubeadmStepSpec) GetName() string {
	return "Download kubeadm"
}

func (s *DownloadKubeadmStepSpec) PopulateDefaults(ctx runtime.Context) {
	if s.Arch == "" && ctx != nil && ctx.Host != nil {
		s.Arch = ctx.Host.Arch()
		if s.Arch == "x86_64" {
			s.Arch = "amd64"
		} else if s.Arch == "aarch64" {
			s.Arch = "arm64"
		}
	}
	// DownloadDir is expected to be set by the module.
	if s.OutputFilePathKey == "" {s.OutputFilePathKey = KubeadmDownloadedPathKey}
	if s.OutputFileNameKey == "" {s.OutputFileNameKey = KubeadmDownloadedFileNameKey}
	if s.OutputComponentTypeKey == "" {s.OutputComponentTypeKey = KubeadmComponentTypeKey}
	if s.OutputVersionKey == "" {s.OutputVersionKey = KubeadmVersionKey}
	if s.OutputArchKey == "" {s.OutputArchKey = KubeadmArchKey}
	if s.OutputChecksumKey == "" {s.OutputChecksumKey = KubeadmChecksumKey}
	if s.OutputURLKey == "" {s.OutputURLKey = KubeadmDownloadURLKey}
}

type DownloadKubeadmStepExecutor struct{}

func (e *DownloadKubeadmStepExecutor) determineKubeadmFileName(version, arch string) string {
	return "kubeadm"
}

func (e *DownloadKubeadmStepExecutor) determineKubeadmURL(version, arch, zone string) string {
	url := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/%s/kubeadm", version, arch)
	if zone == "cn" {
		url = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/release/%s/bin/linux/%s/kubeadm", version, arch)
	}
	return url
}

func (e *DownloadKubeadmStepExecutor) Check(ctx runtime.Context) (bool, error) {
	rawSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for DownloadKubeadmStep Check")
	}
	spec, ok := rawSpec.(*DownloadKubeadmStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for DownloadKubeadmStep Check: %T", rawSpec)
	}
	spec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())

	if spec.DownloadDir == "" { return false, fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName()) }
	fileName := e.determineKubeadmFileName(spec.Version, spec.Arch)
	expectedFilePath := filepath.Join(spec.DownloadDir, fileName)

	exists, err := ctx.Host.Runner.Exists(ctx.GoContext, expectedFilePath)
	if err != nil {return false, fmt.Errorf("failed to check existence of %s: %w", expectedFilePath, err)}
	if !exists {
		logger.Infof("Kubeadm binary %s does not exist.", expectedFilePath)
		return false, nil
	}
	logger.Infof("Kubeadm binary %s exists.", expectedFilePath)

	if spec.Checksum != "" {
		checksumValue := spec.Checksum; checksumType := "sha256"
		if strings.Contains(spec.Checksum, ":") {
			parts := strings.SplitN(spec.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Infof("Verifying %s checksum for %s", checksumType, expectedFilePath)
		actualHash, errC := ctx.Host.Runner.GetFileChecksum(ctx.GoContext, expectedFilePath, checksumType)
		if errC != nil {
			logger.Warnf("Failed to get %s checksum for %s: %v. Assuming invalid.", checksumType, expectedFilePath, errC)
			return false, nil
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			logger.Warnf("%s checksum mismatch for %s. Expected: %s, Got: %s.", checksumType, expectedFilePath, checksumValue, actualHash)
			return false, nil
		}
		logger.Infof("%s checksum for %s verified.", checksumType, expectedFilePath)
	}

	ctx.Task().Set(spec.OutputFilePathKey, expectedFilePath)
	ctx.Task().Set(spec.OutputFileNameKey, fileName)
	ctx.Task().Set(spec.OutputComponentTypeKey, "KUBE")
	ctx.Task().Set(spec.OutputVersionKey, spec.Version)
	ctx.Task().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.Task().Set(spec.OutputChecksumKey, spec.Checksum)}
	url := e.determineKubeadmURL(spec.Version, spec.Arch, spec.Zone)
	ctx.Task().Set(spec.OutputURLKey, url)
	return true, nil
}

func (e *DownloadKubeadmStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	rawSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for DownloadKubeadmStep Execute"))
	}
	spec, ok := rawSpec.(*DownloadKubeadmStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for DownloadKubeadmStep Execute: %T", rawSpec))
	}
	spec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	if spec.DownloadDir == "" {
		res.Error = fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; return res
	}
	fileName := e.determineKubeadmFileName(spec.Version, spec.Arch)
	componentType := "KUBE"
	effectiveZone := spec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")}
	url := e.determineKubeadmURL(spec.Version, spec.Arch, effectiveZone)

	logger.Infof("Attempting to download kubeadm from %s to %s/%s", url, spec.DownloadDir, fileName)

	downloadedPath, err := utils.DownloadFile(ctx, url, spec.DownloadDir, fileName, false, spec.Checksum, "sha256")
	if err != nil {
		res.Error = fmt.Errorf("failed to download kubeadm: %w", err)
		res.Status = step.StatusFailed; return res
	}
	logger.Successf("Kubeadm downloaded successfully to %s", downloadedPath)

	ctx.Task().Set(spec.OutputFilePathKey, downloadedPath)
	ctx.Task().Set(spec.OutputFileNameKey, fileName)
	ctx.Task().Set(spec.OutputComponentTypeKey, componentType)
	ctx.Task().Set(spec.OutputVersionKey, spec.Version)
	ctx.Task().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.Task().Set(spec.OutputChecksumKey, spec.Checksum)}
	ctx.Task().Set(spec.OutputURLKey, url)
	res.Status = step.StatusSucceeded
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&DownloadKubeadmStepSpec{}), &DownloadKubeadmStepExecutor{})
}
