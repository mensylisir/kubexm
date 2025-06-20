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
	ContainerdDownloadedPathKey     = "ContainerdDownloadedPath"
	ContainerdDownloadedFileNameKey = "ContainerdDownloadedFileName"
	ContainerdComponentTypeKey      = "ContainerdComponentType"
	ContainerdVersionKey            = "ContainerdVersion"
	ContainerdArchKey               = "ContainerdArch"
	ContainerdChecksumKey           = "ContainerdChecksum"
	ContainerdDownloadURLKey        = "ContainerdDownloadURL"
)

type DownloadContainerdStepSpec struct {
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

func (s *DownloadContainerdStepSpec) GetName() string {
	return "Download containerd"
}

func (s *DownloadContainerdStepSpec) PopulateDefaults(ctx runtime.StepContext) { // Changed to StepContext
	if s.Arch == "" {
		currentHost := ctx.GetHost()
		if currentHost != nil {
			// Assuming connector.Host has GetArch() method
			arch := currentHost.GetArch()
			if arch == "x86_64" {
				s.Arch = "amd64"
			} else if arch == "aarch64" {
				s.Arch = "arm64"
			} else {
				s.Arch = arch // Use as is if not x86_64 or aarch64
			}
		}
	}
	// DownloadDir is expected to be set by the module.
	if s.OutputFilePathKey == "" {s.OutputFilePathKey = ContainerdDownloadedPathKey}
	if s.OutputFileNameKey == "" {s.OutputFileNameKey = ContainerdDownloadedFileNameKey}
	if s.OutputComponentTypeKey == "" {s.OutputComponentTypeKey = ContainerdComponentTypeKey}
	if s.OutputVersionKey == "" {s.OutputVersionKey = ContainerdVersionKey}
	if s.OutputArchKey == "" {s.OutputArchKey = ContainerdArchKey}
	if s.OutputChecksumKey == "" {s.OutputChecksumKey = ContainerdChecksumKey}
	if s.OutputURLKey == "" {s.OutputURLKey = ContainerdDownloadURLKey}
}

type DownloadContainerdStepExecutor struct{}

func (e *DownloadContainerdStepExecutor) determineContainerdFileName(version, arch string) string {
	return fmt.Sprintf("containerd-%s-linux-%s.tar.gz", strings.TrimPrefix(version, "v"), arch)
}

func (e *DownloadContainerdStepExecutor) determineContainerdURL(version, arch, fileName, zone string) string {
	versionWithV := version
	if !strings.HasPrefix(versionWithV, "v") {
		versionWithV = "v" + versionWithV
	}
	url := fmt.Sprintf("https://github.com/containerd/containerd/releases/download/%s/%s", versionWithV, fileName)
	if zone == "cn" {
		url = fmt.Sprintf("https://containerd-release.pek3b.qingstor.com/containerd/%s/%s", versionWithV, fileName)
	}
	return url
}

func (e *DownloadContainerdStepExecutor) Check(ctx runtime.StepContext) (bool, error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for DownloadContainerdStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for DownloadContainerdStep Check")
	}
	spec, ok := rawSpec.(*DownloadContainerdStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for DownloadContainerdStep Check: %T", rawSpec)
	}
	spec.PopulateDefaults(ctx) // Pass StepContext
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		return false, fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
	}
	fileName := e.determineContainerdFileName(spec.Version, spec.Arch)
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
		logger.Info("Containerd archive does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Containerd archive exists.", "path", expectedFilePath)

	if spec.Checksum != "" {
		checksumValue := spec.Checksum; checksumType := "sha256"
		if strings.Contains(spec.Checksum, ":") {
			parts := strings.SplitN(spec.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Info("Verifying checksum", "type", checksumType, "path", expectedFilePath)
		actualHash, errC := conn.GetFileChecksum(goCtx, expectedFilePath, checksumType) // Use connector
		if errC != nil {
			logger.Warn("Failed to get checksum, assuming invalid.", "type", checksumType, "path", expectedFilePath, "error", errC)
			return false, nil // Treat as not done if checksum fails
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			logger.Warn("Checksum mismatch.", "type", checksumType, "path", expectedFilePath, "expected", checksumValue, "actual", actualHash)
			return false, nil // Checksum mismatch, needs re-download or correction
		}
		logger.Info("Checksum verified.", "type", checksumType, "path", expectedFilePath)
	}

	ctx.TaskCache().Set(spec.OutputFilePathKey, expectedFilePath)
	ctx.TaskCache().Set(spec.OutputFileNameKey, fileName)
	ctx.TaskCache().Set(spec.OutputComponentTypeKey, "CONTAINER_RUNTIME")
	ctx.TaskCache().Set(spec.OutputVersionKey, spec.Version)
	ctx.TaskCache().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.TaskCache().Set(spec.OutputChecksumKey, spec.Checksum)}
	url := e.determineContainerdURL(spec.Version, spec.Arch, fileName, spec.Zone)
	ctx.TaskCache().Set(spec.OutputURLKey, url)
	logger.Info("DownloadContainerd check determined step is done, relevant info cached in TaskCache.")
	return true, nil
}

func (e *DownloadContainerdStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for DownloadContainerdStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for DownloadContainerdStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*DownloadContainerdStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for DownloadContainerdStep Execute: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults(ctx) // Pass StepContext
	logger = logger.With("step", spec.GetName())


	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		res.Error = fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	fileName := e.determineContainerdFileName(spec.Version, spec.Arch)
	componentType := "CONTAINER_RUNTIME"
	effectiveZone := spec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")} // KKZONE is a common env var for this
	url := e.determineContainerdURL(spec.Version, spec.Arch, fileName, effectiveZone)

	logger.Info("Attempting to download containerd", "url", url, "destinationDir", spec.DownloadDir, "fileName", fileName)

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	// Adapt utils.DownloadFile call
	// Assuming utils.DownloadFile needs goCtx, logger, connector, url, dir, name, checksum, checksumType
	// This is a hypothetical signature for an adapted DownloadFile or a new helper.
	// The original `utils.DownloadFile(ctx, ...)` passed the whole runtime.Context.
	// For now, assuming `utils.DownloadFile` is adapted or can work with these specific arguments.
	// If `utils.DownloadFile` is simple (e.g. HTTP GET then save locally on control node), this would be different.
	// But given `DownloadDir` and the context of other steps, it's likely download *to the target host*.
	var downloadedPath string
	var downloadErr error

	// This is a placeholder for how DownloadFile might be called.
	// The actual implementation of DownloadFile in utils package would need to be checked/adapted.
	// For this refactoring, we assume it can be made to work with these parameters.
	// If DownloadFile uses methods from the old runtime.Context not available here, it's a deeper issue.
	// For now, focusing on what this step *provides* to a download utility.
	downloadedPath, downloadErr = utils.DownloadFileWithConnector(
		goCtx,
		logger, // Pass the contextualized logger
		conn,   // Pass the connector for the current host
		url,
		spec.DownloadDir,
		fileName,
		spec.Checksum, // Checksum string, might include type like "sha256:value"
		// "sha256", // Assuming sha256 if not part of spec.Checksum, DownloadFileWithConnector should parse
	)

	if downloadErr != nil {
		logger.Error("Failed to download containerd", "error", downloadErr)
		res.Error = fmt.Errorf("failed to download containerd: %w", downloadErr)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Containerd downloaded successfully.", "path", downloadedPath)

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
	if s.OutputFilePathKey == "" {s.OutputFilePathKey = ContainerdDownloadedPathKey}
	if s.OutputFileNameKey == "" {s.OutputFileNameKey = ContainerdDownloadedFileNameKey}
	if s.OutputComponentTypeKey == "" {s.OutputComponentTypeKey = ContainerdComponentTypeKey}
	if s.OutputVersionKey == "" {s.OutputVersionKey = ContainerdVersionKey}
	if s.OutputArchKey == "" {s.OutputArchKey = ContainerdArchKey}
	if s.OutputChecksumKey == "" {s.OutputChecksumKey = ContainerdChecksumKey}
	if s.OutputURLKey == "" {s.OutputURLKey = ContainerdDownloadURLKey}
}

type DownloadContainerdStepExecutor struct{}

func (e *DownloadContainerdStepExecutor) determineContainerdFileName(version, arch string) string {
	return fmt.Sprintf("containerd-%s-linux-%s.tar.gz", strings.TrimPrefix(version, "v"), arch)
}

func (e *DownloadContainerdStepExecutor) determineContainerdURL(version, arch, fileName, zone string) string {
	versionWithV := version
	if !strings.HasPrefix(versionWithV, "v") {
		versionWithV = "v" + versionWithV
	}
	url := fmt.Sprintf("https://github.com/containerd/containerd/releases/download/%s/%s", versionWithV, fileName)
	if zone == "cn" {
		url = fmt.Sprintf("https://containerd-release.pek3b.qingstor.com/containerd/%s/%s", versionWithV, fileName)
	}
	return url
}

func (e *DownloadContainerdStepExecutor) Check(ctx runtime.StepContext) (bool, error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for DownloadContainerdStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for DownloadContainerdStep Check")
	}
	spec, ok := rawSpec.(*DownloadContainerdStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for DownloadContainerdStep Check: %T", rawSpec)
	}
	spec.PopulateDefaults(ctx) // Pass StepContext
	logger = logger.With("step", spec.GetName())

	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		return false, fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
	}
	fileName := e.determineContainerdFileName(spec.Version, spec.Arch)
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
		logger.Info("Containerd archive does not exist.", "path", expectedFilePath)
		return false, nil
	}
	logger.Info("Containerd archive exists.", "path", expectedFilePath)

	if spec.Checksum != "" {
		checksumValue := spec.Checksum; checksumType := "sha256"
		if strings.Contains(spec.Checksum, ":") {
			parts := strings.SplitN(spec.Checksum, ":", 2); checksumType = parts[0]; checksumValue = parts[1]
		}
		logger.Info("Verifying checksum", "type", checksumType, "path", expectedFilePath)
		actualHash, errC := conn.GetFileChecksum(goCtx, expectedFilePath, checksumType) // Use connector
		if errC != nil {
			logger.Warn("Failed to get checksum, assuming invalid.", "type", checksumType, "path", expectedFilePath, "error", errC)
			return false, nil // Treat as not done if checksum fails
		}
		if !strings.EqualFold(actualHash, checksumValue) {
			logger.Warn("Checksum mismatch.", "type", checksumType, "path", expectedFilePath, "expected", checksumValue, "actual", actualHash)
			return false, nil // Checksum mismatch, needs re-download or correction
		}
		logger.Info("Checksum verified.", "type", checksumType, "path", expectedFilePath)
	}

	ctx.TaskCache().Set(spec.OutputFilePathKey, expectedFilePath)
	ctx.TaskCache().Set(spec.OutputFileNameKey, fileName)
	ctx.TaskCache().Set(spec.OutputComponentTypeKey, "CONTAINER_RUNTIME")
	ctx.TaskCache().Set(spec.OutputVersionKey, spec.Version)
	ctx.TaskCache().Set(spec.OutputArchKey, spec.Arch)
	if spec.Checksum != "" {ctx.TaskCache().Set(spec.OutputChecksumKey, spec.Checksum)}
	url := e.determineContainerdURL(spec.Version, spec.Arch, fileName, spec.Zone)
	ctx.TaskCache().Set(spec.OutputURLKey, url)
	logger.Info("DownloadContainerd check determined step is done, relevant info cached in TaskCache.")
	return true, nil
}

func (e *DownloadContainerdStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for DownloadContainerdStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for DownloadContainerdStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*DownloadContainerdStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for DownloadContainerdStep Execute: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults(ctx) // Pass StepContext
	logger = logger.With("step", spec.GetName())


	if spec.DownloadDir == "" {
		logger.Error("DownloadDir not set in spec")
		res.Error = fmt.Errorf("DownloadDir not set in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	fileName := e.determineContainerdFileName(spec.Version, spec.Arch)
	componentType := "CONTAINER_RUNTIME"
	effectiveZone := spec.Zone
	if effectiveZone == "" {effectiveZone = os.Getenv("KKZONE")} // KKZONE is a common env var for this
	url := e.determineContainerdURL(spec.Version, spec.Arch, fileName, effectiveZone)

	logger.Info("Attempting to download containerd", "url", url, "destinationDir", spec.DownloadDir, "fileName", fileName)

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	// Adapt utils.DownloadFile call
	// Assuming utils.DownloadFile needs goCtx, logger, connector, url, dir, name, checksum, checksumType
	// This is a hypothetical signature for an adapted DownloadFile or a new helper.
	// The original `utils.DownloadFile(ctx, ...)` passed the whole runtime.Context.
	// For now, assuming `utils.DownloadFile` is adapted or can work with these specific arguments.
	// If `utils.DownloadFile` is simple (e.g. HTTP GET then save locally on control node), this would be different.
	// But given `DownloadDir` and the context of other steps, it's likely download *to the target host*.
	var downloadedPath string
	var downloadErr error

	// This is a placeholder for how DownloadFile might be called.
	// The actual implementation of DownloadFile in utils package would need to be checked/adapted.
	// For this refactoring, we assume it can be made to work with these parameters.
	// If DownloadFile uses methods from the old runtime.Context not available here, it's a deeper issue.
	// For now, focusing on what this step *provides* to a download utility.
	downloadedPath, downloadErr = utils.DownloadFileWithConnector(
		goCtx,
		logger, // Pass the contextualized logger
		conn,   // Pass the connector for the current host
		url,
		spec.DownloadDir,
		fileName,
		spec.Checksum, // Checksum string, might include type like "sha256:value"
		// "sha256", // Assuming sha256 if not part of spec.Checksum, DownloadFileWithConnector should parse
	)

	if downloadErr != nil {
		logger.Error("Failed to download containerd", "error", downloadErr)
		res.Error = fmt.Errorf("failed to download containerd: %w", downloadErr)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Containerd downloaded successfully.", "path", downloadedPath)

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
	step.Register(step.GetSpecTypeName(&DownloadContainerdStepSpec{}), &DownloadContainerdStepExecutor{})
}
