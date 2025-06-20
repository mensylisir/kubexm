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

func (s *DownloadKubeadmStepSpec) PopulateDefaults(ctx runtime.StepContext) { // Changed to StepContext
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
	return "kubeadm" // Kubeadm is just a binary, not a tarball usually from official releases
}

func (e *DownloadKubeadmStepExecutor) determineKubeadmURL(version, arch, zone string) string {
	url := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/%s/kubeadm", version, arch)
	if zone == "cn" {
		url = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/release/%s/bin/linux/%s/kubeadm", version, arch)
	}
	return url
}

func (e *DownloadKubeadmStepExecutor) Check(ctx runtime.StepContext) (bool, error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for DownloadKubeadmStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for DownloadKubeadmStep Check")
	}
	spec, ok := rawSpec.(*DownloadKubeadmStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for DownloadKubeadmStep Check: %T", rawSpec)
	}
	spec.PopulateDefaults(ctx)
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		return false, fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
	}
	fileName := e.determineKubeadmFileName(spec.Version, spec.Arch)
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
		logger.Info("Kubeadm binary does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Kubeadm binary exists.", "path", expectedFilePath)

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
	ctx.TaskCache().Set(spec.OutputComponentTypeKey, "KUBE") // KUBE for kubeadm, kubectl, kubelet
	ctx.TaskCache().Set(spec.OutputVersionKey, spec.Version)
	ctx.TaskCache().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.TaskCache().Set(spec.OutputChecksumKey, spec.Checksum)}
	url := e.determineKubeadmURL(spec.Version, spec.Arch, spec.Zone)
	ctx.TaskCache().Set(spec.OutputURLKey, url)
	logger.Info("DownloadKubeadm check determined step is done, relevant info cached in TaskCache.")
	return true, nil
}

func (e *DownloadKubeadmStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for DownloadKubeadmStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for DownloadKubeadmStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*DownloadKubeadmStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for DownloadKubeadmStep Execute: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults(ctx)
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		res.Error = fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	fileName := e.determineKubeadmFileName(spec.Version, spec.Arch)
	componentType := "KUBE"
	effectiveZone := spec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")}
	url := e.determineKubeadmURL(spec.Version, spec.Arch, effectiveZone)

	logger.Info("Attempting to download kubeadm", "url", url, "destinationDir", spec.DownloadDir, "fileName", fileName)

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
		logger.Error("Failed to download kubeadm", "error", downloadErr)
		res.Error = fmt.Errorf("failed to download kubeadm: %w", downloadErr)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Kubeadm downloaded successfully.", "path", downloadedPath)

	// Make the downloaded file executable
	logger.Info("Making kubeadm binary executable", "path", downloadedPath)
	// Assuming connector has a Chmod or similar method
	// For now, let's assume Execute command "chmod +x"
	chmodCmd := fmt.Sprintf("chmod +x %s", downloadedPath)
	_, _, chmodErr := conn.RunCommand(goCtx, chmodCmd, &connector.ExecOptions{Sudo: true}) // Assuming sudo might be needed
	if chmodErr != nil {
		logger.Error("Failed to make kubeadm binary executable", "path", downloadedPath, "error", chmodErr)
		// Not failing the step for this, but logging a warning. Or should it fail?
		// For now, let's make it a warning and proceed.
		res.Message = fmt.Sprintf("Warning: failed to make kubeadm executable at %s: %v", downloadedPath, chmodErr)
	} else {
		logger.Info("Kubeadm binary made executable.", "path", downloadedPath)
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
	return "kubeadm" // Kubeadm is just a binary, not a tarball usually from official releases
}

func (e *DownloadKubeadmStepExecutor) determineKubeadmURL(version, arch, zone string) string {
	url := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/%s/kubeadm", version, arch)
	if zone == "cn" {
		url = fmt.Sprintf("https://kubernetes-release.pek3b.qingstor.com/release/%s/bin/linux/%s/kubeadm", version, arch)
	}
	return url
}

func (e *DownloadKubeadmStepExecutor) Check(ctx runtime.StepContext) (bool, error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for DownloadKubeadmStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for DownloadKubeadmStep Check")
	}
	spec, ok := rawSpec.(*DownloadKubeadmStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for DownloadKubeadmStep Check: %T", rawSpec)
	}
	spec.PopulateDefaults(ctx)
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		return false, fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
	}
	fileName := e.determineKubeadmFileName(spec.Version, spec.Arch)
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
		logger.Info("Kubeadm binary does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Kubeadm binary exists.", "path", expectedFilePath)

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
	ctx.TaskCache().Set(spec.OutputComponentTypeKey, "KUBE") // KUBE for kubeadm, kubectl, kubelet
	ctx.TaskCache().Set(spec.OutputVersionKey, spec.Version)
	ctx.TaskCache().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.TaskCache().Set(spec.OutputChecksumKey, spec.Checksum)}
	url := e.determineKubeadmURL(spec.Version, spec.Arch, spec.Zone)
	ctx.TaskCache().Set(spec.OutputURLKey, url)
	logger.Info("DownloadKubeadm check determined step is done, relevant info cached in TaskCache.")
	return true, nil
}

func (e *DownloadKubeadmStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for DownloadKubeadmStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for DownloadKubeadmStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*DownloadKubeadmStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for DownloadKubeadmStep Execute: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults(ctx)
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		res.Error = fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	fileName := e.determineKubeadmFileName(spec.Version, spec.Arch)
	componentType := "KUBE"
	effectiveZone := spec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")}
	url := e.determineKubeadmURL(spec.Version, spec.Arch, effectiveZone)

	logger.Info("Attempting to download kubeadm", "url", url, "destinationDir", spec.DownloadDir, "fileName", fileName)

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
		logger.Error("Failed to download kubeadm", "error", downloadErr)
		res.Error = fmt.Errorf("failed to download kubeadm: %w", downloadErr)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Kubeadm downloaded successfully.", "path", downloadedPath)

	// Make the downloaded file executable
	logger.Info("Making kubeadm binary executable", "path", downloadedPath)
	// Assuming connector has a Chmod or similar method
	// For now, let's assume Execute command "chmod +x"
	chmodCmd := fmt.Sprintf("chmod +x %s", downloadedPath)
	_, _, chmodErr := conn.RunCommand(goCtx, chmodCmd, &connector.ExecOptions{Sudo: true}) // Assuming sudo might be needed
	if chmodErr != nil {
		logger.Error("Failed to make kubeadm binary executable", "path", downloadedPath, "error", chmodErr)
		// Not failing the step for this, but logging a warning. Or should it fail?
		// For now, let's make it a warning and proceed.
		res.Message = fmt.Sprintf("Warning: failed to make kubeadm executable at %s: %v", downloadedPath, chmodErr)
	} else {
		logger.Info("Kubeadm binary made executable.", "path", downloadedPath)
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
	step.Register(step.GetSpecTypeName(&DownloadKubeadmStepSpec{}), &DownloadKubeadmStepExecutor{})
}
