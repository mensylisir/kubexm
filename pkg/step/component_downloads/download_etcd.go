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
	EtcdDownloadedPathKey      = "EtcdDownloadedPath"
	EtcdDownloadedFileNameKey  = "EtcdDownloadedFileName"
	EtcdComponentTypeKey       = "EtcdComponentType"
	EtcdVersionKey             = "EtcdVersion"
	EtcdArchKey                = "EtcdArch"
	EtcdChecksumKey            = "EtcdChecksum"
	EtcdDownloadURLKey         = "EtcdDownloadURL"
)

type DownloadEtcdStepSpec struct {
	Version              string `json:"version"`
	Arch                 string `json:"arch"`
	Zone                 string `json:"zone,omitempty"`
	DownloadDir          string `json:"downloadDir,omitempty"` // This will be EXPLICITLY set by the module
	Checksum             string `json:"checksum,omitempty"`
	OutputFilePathKey    string `json:"outputFilePathKey,omitempty"`
	OutputFileNameKey    string `json:"outputFileNameKey,omitempty"`
	OutputComponentTypeKey string `json:"outputComponentTypeKey,omitempty"`
	OutputVersionKey     string `json:"outputVersionKey,omitempty"`
	OutputArchKey        string `json:"outputArchKey,omitempty"`
	OutputChecksumKey    string `json:"outputChecksumKey,omitempty"`
	OutputURLKey         string `json:"outputURLKey,omitempty"`
}

func (s *DownloadEtcdStepSpec) GetName() string {
	return "Download etcd"
}

func (s *DownloadEtcdStepSpec) PopulateDefaults(ctx runtime.Context) { // Changed to runtime.Context
	if s.Arch == "" && ctx != nil && ctx.Host != nil {
		s.Arch = ctx.Host.Arch()
		if s.Arch == "x86_64" {
			s.Arch = "amd64"
		} else if s.Arch == "aarch64" {
			s.Arch = "arm64"
		}
	}
	// DownloadDir is now expected to be set by the module.
	if s.OutputFilePathKey == "" {s.OutputFilePathKey = EtcdDownloadedPathKey}
	if s.OutputFileNameKey == "" {s.OutputFileNameKey = EtcdDownloadedFileNameKey}
	if s.OutputComponentTypeKey == "" {s.OutputComponentTypeKey = EtcdComponentTypeKey}
	if s.OutputVersionKey == "" {s.OutputVersionKey = EtcdVersionKey}
	if s.OutputArchKey == "" {s.OutputArchKey = EtcdArchKey}
	if s.OutputChecksumKey == "" {s.OutputChecksumKey = EtcdChecksumKey}
	if s.OutputURLKey == "" {s.OutputURLKey = EtcdDownloadURLKey}
}

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

func (e *DownloadEtcdStepExecutor) Check(ctx runtime.Context) (bool, error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for DownloadEtcdStep Check")
	}
	spec, ok := currentFullSpec.(*DownloadEtcdStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for DownloadEtcdStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults(ctx) // Pass ctx
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())

	if spec.DownloadDir == "" { // Crucial check now that module sets it
		return false, fmt.Errorf("DownloadDir not set in spec for DownloadEtcdStep")
	}
	fileName := e.determineEtcdFileName(spec.Version, spec.Arch)
	expectedFilePath := filepath.Join(spec.DownloadDir, fileName)

	exists, err := ctx.Host.Runner.Exists(ctx.GoContext, expectedFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of %s: %w", expectedFilePath, err)
	}
	if !exists {
		logger.Infof("Etcd archive %s does not exist.", expectedFilePath)
		return false, nil
	}
	logger.Infof("Etcd archive %s exists.", expectedFilePath)

	if spec.Checksum != "" {
		checksumValue := spec.Checksum
		checksumType := "sha256"
		if strings.Contains(spec.Checksum, ":") {
			parts := strings.SplitN(spec.Checksum, ":", 2)
			checksumType = parts[0]
			checksumValue = parts[1]
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

	ctx.Task().Set(spec.OutputFilePathKey, expectedFilePath)
	ctx.Task().Set(spec.OutputFileNameKey, fileName)
	ctx.Task().Set(spec.OutputComponentTypeKey, "ETCD")
	ctx.Task().Set(spec.OutputVersionKey, spec.Version)
	ctx.Task().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {
		ctx.Task().Set(spec.OutputChecksumKey, spec.Checksum)
	}
	url := e.determineEtcdURL(spec.Version, spec.Arch, fileName, spec.Zone)
	ctx.Task().Set(spec.OutputURLKey, url)

	return true, nil
}

func (e *DownloadEtcdStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for DownloadEtcdStep Execute"))
	}
	spec, ok := currentFullSpec.(*DownloadEtcdStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for DownloadEtcdStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults(ctx) // Pass ctx
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	if spec.DownloadDir == "" { // Crucial check
		res.Error = fmt.Errorf("DownloadDir not set in spec for DownloadEtcdStep")
		res.Status = step.StatusFailed; return res
	}

	fileName := e.determineEtcdFileName(spec.Version, spec.Arch)
	componentType := "ETCD"
	effectiveZone := spec.Zone
	if effectiveZone == "" {
		effectiveZone = os.Getenv("KKZONE")
	}
	url := e.determineEtcdURL(spec.Version, spec.Arch, fileName, effectiveZone)

	logger.Infof("Attempting to download etcd from %s to %s/%s", url, spec.DownloadDir, fileName)

	downloadedPath, err := utils.DownloadFile(ctx, url, spec.DownloadDir, fileName, false, spec.Checksum, "sha256")
	if err != nil {
		res.Error = fmt.Errorf("failed to download etcd: %w", err)
		res.Status = step.StatusFailed
		return res
	}
	logger.Successf("Etcd downloaded successfully to %s", downloadedPath)

	ctx.Task().Set(spec.OutputFilePathKey, downloadedPath)
	ctx.Task().Set(spec.OutputFileNameKey, fileName)
	ctx.Task().Set(spec.OutputComponentTypeKey, componentType)
	ctx.Task().Set(spec.OutputVersionKey, spec.Version)
	ctx.Task().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {
		ctx.Task().Set(spec.OutputChecksumKey, spec.Checksum)
	}
	ctx.Task().Set(spec.OutputURLKey, url)
	res.Status = step.StatusSucceeded
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&DownloadEtcdStepSpec{}), &DownloadEtcdStepExecutor{})
}
