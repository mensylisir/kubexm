package component_downloads

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // Added
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils"
)

const (
	KubectlDownloadedPathKey     = "KubectlDownloadedPath"
	KubectlDownloadedFileNameKey = "KubectlDownloadedFileName"
	KubectlComponentTypeKey      = "KubectlComponentType"
	KubectlVersionKey            = "KubectlVersion"
	KubectlArchKey               = "KubectlArch"
	KubectlChecksumKey           = "KubectlChecksum"
	KubectlDownloadURLKey        = "KubectlDownloadURL"
)

type DownloadKubectlStepSpec struct {
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

func (s *DownloadKubectlStepSpec) GetName() string {
	return "Download kubectl"
}

func (s *DownloadKubectlStepSpec) PopulateDefaults(ctx runtime.StepContext) { // Changed to StepContext
	if s.Arch == "" {
		currentHost := ctx.GetHost()
		if currentHost != nil {
			arch := currentHost.GetArch() // Assuming connector.Host has GetArch()
			if arch == "x86_64" {
				s.Arch = "amd64"
			} else if arch == "aarch64" {
				s.Arch = "arm64"
			} else {
				s.Arch = arch
			}
		}
	}
	// DownloadDir is expected to be set by the module.
	if s.OutputFilePathKey == "" {s.OutputFilePathKey = KubectlDownloadedPathKey}
	if s.OutputFileNameKey == "" {s.OutputFileNameKey = KubectlDownloadedFileNameKey}
	if s.OutputComponentTypeKey == "" {s.OutputComponentTypeKey = KubectlComponentTypeKey}
	if s.OutputVersionKey == "" {s.OutputVersionKey = KubectlVersionKey}
	if s.OutputArchKey == "" {s.OutputArchKey = KubectlArchKey}
	if s.OutputChecksumKey == "" {s.OutputChecksumKey = KubectlChecksumKey}
	if s.OutputURLKey == "" {s.OutputURLKey = KubectlDownloadURLKey}
}

type DownloadKubectlStepExecutor struct{}

func (e *DownloadKubectlStepExecutor) determineKubectlFileName(version, arch string) string {
	return "kubectl"
}

func (e *DownloadKubectlStepExecutor) determineKubectlURL(version, arch, zone string) string {
	url := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/%s/kubectl", version, arch)
	if zone == "cn" {
		url = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/release/%s/bin/linux/%s/kubectl", version, arch)
	}
	return url
}

func (e *DownloadKubectlStepExecutor) Check(ctx runtime.StepContext) (bool, error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for DownloadKubectlStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for DownloadKubectlStep Check")
	}
	spec, ok := rawSpec.(*DownloadKubectlStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for DownloadKubectlStep Check: %T", rawSpec)
	}
	spec.PopulateDefaults(ctx)
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		return false, fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
	}
	fileName := e.determineKubectlFileName(spec.Version, spec.Arch)
	expectedFilePath := filepath.Join(spec.DownloadDir, fileName)

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	exists, err := conn.Exists(goCtx, expectedFilePath) // Use connector
	if err != nil {
		logger.Error("Failed to check existence of file", "path", expectedFilePath, "error", err)
		return false, fmt.Errorf("failed to check existence of %s: %w", expectedFilePath, err)
	}
	if !exists {
		logger.Info("Kubectl binary does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Kubectl binary exists.", "path", expectedFilePath)

	if spec.Checksum != "" {
		checksumValue := spec.Checksum; checksumType := "sha256"
		if strings.Contains(spec.Checksum, ":") {
			parts := strings.SplitN(spec.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Info("Verifying checksum", "type", checksumType, "path", expectedFilePath)
		actualHash, errC := conn.GetFileChecksum(goCtx, expectedFilePath, checksumType) // Use connector
		if errC != nil {
			logger.Warn("Failed to get checksum, assuming invalid.", "type", checksumType, "path", expectedFilePath, "error", errC)
			return false, nil
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			logger.Warn("Checksum mismatch.", "type", checksumType, "path", expectedFilePath, "expected", checksumValue, "actual", actualHash)
			return false, nil
		}
		logger.Info("Checksum verified.", "type", checksumType, "path", expectedFilePath)
	}

	ctx.TaskCache().Set(spec.OutputFilePathKey, expectedFilePath)
	ctx.TaskCache().Set(spec.OutputFileNameKey, fileName)
	ctx.TaskCache().Set(spec.OutputComponentTypeKey, "KUBE")
	ctx.TaskCache().Set(spec.OutputVersionKey, spec.Version)
	ctx.TaskCache().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.TaskCache().Set(spec.OutputChecksumKey, spec.Checksum)}
	url := e.determineKubectlURL(spec.Version, spec.Arch, spec.Zone)
	ctx.TaskCache().Set(spec.OutputURLKey, url)
	logger.Info("DownloadKubectl check determined step is done, relevant info cached in TaskCache.")
	return true, nil
}

func (e *DownloadKubectlStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for DownloadKubectlStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for DownloadKubectlStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*DownloadKubectlStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for DownloadKubectlStep Execute: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults(ctx)
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		res.Error = fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	fileName := e.determineKubectlFileName(spec.Version, spec.Arch)
	componentType := "KUBE"
	effectiveZone := spec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")}
	url := e.determineKubectlURL(spec.Version, spec.Arch, effectiveZone)

	logger.Info("Attempting to download kubectl", "url", url, "destinationDir", spec.DownloadDir, "fileName", fileName)

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	downloadedPath, downloadErr := utils.DownloadFileWithConnector( // Assuming adapted util function
		goCtx,
		logger,
		conn,
		url,
		spec.DownloadDir,
		fileName,
		spec.Checksum,
	)

	if downloadErr != nil {
		logger.Error("Failed to download kubectl", "error", downloadErr)
		res.Error = fmt.Errorf("failed to download kubectl: %w", downloadErr)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Kubectl downloaded successfully.", "path", downloadedPath)

	logger.Info("Making kubectl binary executable", "path", downloadedPath)
	chmodCmd := fmt.Sprintf("chmod +x %s", downloadedPath)
	_, _, chmodErr := conn.RunCommand(goCtx, chmodCmd, &connector.ExecOptions{Sudo: true})
	if chmodErr != nil {
		logger.Error("Failed to make kubectl binary executable", "path", downloadedPath, "error", chmodErr)
		res.Message = fmt.Sprintf("Warning: failed to make kubectl executable at %s: %v", downloadedPath, chmodErr)
	} else {
		logger.Info("Kubectl binary made executable.", "path", downloadedPath)
	}

	ctx.TaskCache().Set(spec.OutputFilePathKey, downloadedPath)
	ctx.TaskCache().Set(spec.OutputFileNameKey, fileName)
	ctx.TaskCache().Set(spec.OutputComponentTypeKey, componentType)
	ctx.TaskCache().Set(spec.OutputVersionKey, spec.Version)
	ctx.TaskCache().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.TaskCache().Set(spec.OutputChecksumKey, spec.Checksum)}
	ctx.TaskCache().Set(spec.OutputURLKey, url)

	res.Status = step.StatusSucceeded
	res.EndTime = time.Now()
	return res
}

func init() {
		} else if s.Arch == "aarch64" {
			s.Arch = "arm64"
		}
	}
	// DownloadDir is expected to be set by the module.
	if s.OutputFilePathKey == "" {s.OutputFilePathKey = KubectlDownloadedPathKey}
	if s.OutputFileNameKey == "" {s.OutputFileNameKey = KubectlDownloadedFileNameKey}
	if s.OutputComponentTypeKey == "" {s.OutputComponentTypeKey = KubectlComponentTypeKey}
	if s.OutputVersionKey == "" {s.OutputVersionKey = KubectlVersionKey}
	if s.OutputArchKey == "" {s.OutputArchKey = KubectlArchKey}
	if s.OutputChecksumKey == "" {s.OutputChecksumKey = KubectlChecksumKey}
	if s.OutputURLKey == "" {s.OutputURLKey = KubectlDownloadURLKey}
}

type DownloadKubectlStepExecutor struct{}

func (e *DownloadKubectlStepExecutor) determineKubectlFileName(version, arch string) string {
	return "kubectl"
}

func (e *DownloadKubectlStepExecutor) determineKubectlURL(version, arch, zone string) string {
	url := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/%s/kubectl", version, arch)
	if zone == "cn" {
		url = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/release/%s/bin/linux/%s/kubectl", version, arch)
	}
	return url
}

func (e *DownloadKubectlStepExecutor) Check(ctx runtime.StepContext) (bool, error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for DownloadKubectlStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for DownloadKubectlStep Check")
	}
	spec, ok := rawSpec.(*DownloadKubectlStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for DownloadKubectlStep Check: %T", rawSpec)
	}
	spec.PopulateDefaults(ctx)
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		return false, fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
	}
	fileName := e.determineKubectlFileName(spec.Version, spec.Arch)
	expectedFilePath := filepath.Join(spec.DownloadDir, fileName)

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	exists, err := conn.Exists(goCtx, expectedFilePath) // Use connector
	if err != nil {
		logger.Error("Failed to check existence of file", "path", expectedFilePath, "error", err)
		return false, fmt.Errorf("failed to check existence of %s: %w", expectedFilePath, err)
	}
	if !exists {
		logger.Info("Kubectl binary does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Kubectl binary exists.", "path", expectedFilePath)

	if spec.Checksum != "" {
		checksumValue := spec.Checksum; checksumType := "sha256"
		if strings.Contains(spec.Checksum, ":") {
			parts := strings.SplitN(spec.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Info("Verifying checksum", "type", checksumType, "path", expectedFilePath)
		actualHash, errC := conn.GetFileChecksum(goCtx, expectedFilePath, checksumType) // Use connector
		if errC != nil {
			logger.Warn("Failed to get checksum, assuming invalid.", "type", checksumType, "path", expectedFilePath, "error", errC)
			return false, nil
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			logger.Warn("Checksum mismatch.", "type", checksumType, "path", expectedFilePath, "expected", checksumValue, "actual", actualHash)
			return false, nil
		}
		logger.Info("Checksum verified.", "type", checksumType, "path", expectedFilePath)
	}

	ctx.TaskCache().Set(spec.OutputFilePathKey, expectedFilePath)
	ctx.TaskCache().Set(spec.OutputFileNameKey, fileName)
	ctx.TaskCache().Set(spec.OutputComponentTypeKey, "KUBE")
	ctx.TaskCache().Set(spec.OutputVersionKey, spec.Version)
	ctx.TaskCache().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.TaskCache().Set(spec.OutputChecksumKey, spec.Checksum)}
	url := e.determineKubectlURL(spec.Version, spec.Arch, spec.Zone)
	ctx.TaskCache().Set(spec.OutputURLKey, url)
	logger.Info("DownloadKubectl check determined step is done, relevant info cached in TaskCache.")
	return true, nil
}

func (e *DownloadKubectlStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for DownloadKubectlStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for DownloadKubectlStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*DownloadKubectlStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for DownloadKubectlStep Execute: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults(ctx)
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		res.Error = fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	fileName := e.determineKubectlFileName(spec.Version, spec.Arch)
	componentType := "KUBE"
	effectiveZone := spec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")}
	url := e.determineKubectlURL(spec.Version, spec.Arch, effectiveZone)

	logger.Info("Attempting to download kubectl", "url", url, "destinationDir", spec.DownloadDir, "fileName", fileName)

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	downloadedPath, downloadErr := utils.DownloadFileWithConnector( // Assuming adapted util function
		goCtx,
		logger,
		conn,
		url,
		spec.DownloadDir,
		fileName,
		spec.Checksum,
	)

	if downloadErr != nil {
		logger.Error("Failed to download kubectl", "error", downloadErr)
		res.Error = fmt.Errorf("failed to download kubectl: %w", downloadErr)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Kubectl downloaded successfully.", "path", downloadedPath)

	logger.Info("Making kubectl binary executable", "path", downloadedPath)
	chmodCmd := fmt.Sprintf("chmod +x %s", downloadedPath)
	_, _, chmodErr := conn.RunCommand(goCtx, chmodCmd, &connector.ExecOptions{Sudo: true})
	if chmodErr != nil {
		logger.Error("Failed to make kubectl binary executable", "path", downloadedPath, "error", chmodErr)
		res.Message = fmt.Sprintf("Warning: failed to make kubectl executable at %s: %v", downloadedPath, chmodErr)
	} else {
		logger.Info("Kubectl binary made executable.", "path", downloadedPath)
	}

	ctx.TaskCache().Set(spec.OutputFilePathKey, downloadedPath)
	ctx.TaskCache().Set(spec.OutputFileNameKey, fileName)
	ctx.TaskCache().Set(spec.OutputComponentTypeKey, componentType)
	ctx.TaskCache().Set(spec.OutputVersionKey, spec.Version)
	ctx.TaskCache().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.TaskCache().Set(spec.OutputChecksumKey, spec.Checksum)}
	ctx.TaskCache().Set(spec.OutputURLKey, url)

	res.Status = step.StatusSucceeded
	res.EndTime = time.Now()
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&DownloadKubectlStepSpec{}), &DownloadKubectlStepExecutor{})
}
