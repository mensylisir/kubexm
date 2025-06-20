package containerd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	// "os" // For os.Stat in Check - not used

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common" // For DefaultExtractedPathKey
)

// InstallContainerdStepSpec defines parameters for installing containerd from extracted files.
type InstallContainerdStepSpec struct {
	SourceExtractedPathSharedDataKey string            `json:"sourceExtractedPathSharedDataKey,omitempty"` // Key for path to extracted archive root
	SystemdUnitFileSourceRelPath     string            `json:"systemdUnitFileSourceRelPath,omitempty"`   // Relative path of .service file in archive
	SystemdUnitFileTargetPath        string            `json:"systemdUnitFileTargetPath,omitempty"`      // Target path for .service file
	BinariesToCopy                   map[string]string `json:"binariesToCopy,omitempty"`                 // Map: source_rel_path_in_archive -> target_system_path
	StepName                         string            `json:"stepName,omitempty"`                       // Optional custom name
}

// GetName returns the step name.
func (s *InstallContainerdStepSpec) GetName() string {
	if s.StepName != "" { return s.StepName }
	return "Install Containerd from Extracted Files"
}

// PopulateDefaults sets default values.
func (s *InstallContainerdStepSpec) PopulateDefaults() {
	if s.SourceExtractedPathSharedDataKey == "" {
		s.SourceExtractedPathSharedDataKey = commonstep.DefaultExtractedPathKey
	}
	if s.SystemdUnitFileSourceRelPath == "" {
		// containerd.service is often at the root of the extracted archive, not in a 'bin' subdir
		s.SystemdUnitFileSourceRelPath = "containerd.service"
	}
	if s.SystemdUnitFileTargetPath == "" {
		// Common paths for systemd unit files
		s.SystemdUnitFileTargetPath = "/usr/lib/systemd/system/containerd.service"
	}
	if len(s.BinariesToCopy) == 0 {
		s.BinariesToCopy = map[string]string{
			"bin/containerd":                "/usr/local/bin/containerd",
			"bin/containerd-shim":           "/usr/local/bin/containerd-shim",
			"bin/containerd-shim-runc-v1":   "/usr/local/bin/containerd-shim-runc-v1",
			"bin/containerd-shim-runc-v2":   "/usr/local/bin/containerd-shim-runc-v2",
			"bin/ctr":                       "/usr/local/bin/ctr",
			"bin/runc":                      "/usr/local/sbin/runc", // runc often goes here or /usr/local/bin
		}
	}
}
var _ spec.StepSpec = &InstallContainerdStepSpec{}

// InstallContainerdStepExecutor implements the logic for InstallContainerdStepSpec.
type InstallContainerdStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&InstallContainerdStepSpec{}), &InstallContainerdStepExecutor{})
}

// Check determines if containerd seems installed from extracted files.
func (e *InstallContainerdStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for InstallContainerdStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for InstallContainerdStep Check")
	}
	spec, ok := rawSpec.(*InstallContainerdStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for InstallContainerdStep Check: %T", rawSpec)
	}
	spec.PopulateDefaults() // PopulateDefaults does not use context
	logger = logger.With("step", spec.GetName())

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	// Check if all target binaries exist
	for _, targetPath := range spec.BinariesToCopy {
		exists, errExists := conn.Exists(goCtx, targetPath) // Use connector
		if errExists != nil {
			logger.Error("Failed to check existence of target binary", "path", targetPath, "error", errExists)
			return false, fmt.Errorf("failed to check existence of %s: %w", targetPath, errExists)
		}
		if !exists {
			logger.Debug("Target binary does not exist.", "path", targetPath)
			return false, nil
		}
	}
	logger.Debug("All target binaries exist.")

	// Check if systemd unit file exists
	if spec.SystemdUnitFileTargetPath != "" {
		exists, errExists := conn.Exists(goCtx, spec.SystemdUnitFileTargetPath) // Use connector
		if errExists != nil {
			logger.Error("Failed to check existence of systemd unit file", "path", spec.SystemdUnitFileTargetPath, "error", errExists)
			return false, fmt.Errorf("failed to check existence of systemd unit file %s: %w", spec.SystemdUnitFileTargetPath, errExists)
		}
		if !exists {
			logger.Debug("Systemd unit file does not exist.", "path", spec.SystemdUnitFileTargetPath)
			return false, nil
		}
		logger.Debug("Systemd unit file exists.", "path", spec.SystemdUnitFileTargetPath)
	}

	logger.Info("Containerd installation (binaries and service file) appears complete.")
	return true, nil
}

// Execute installs containerd from pre-extracted files.
func (e *InstallContainerdStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("host not available in context"); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for InstallContainerdStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*InstallContainerdStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for InstallContainerdStep Execute: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults() // PopulateDefaults does not use context
	logger = logger.With("step", spec.GetName())


	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	extractedPathVal, found := ctx.TaskCache().Get(spec.SourceExtractedPathSharedDataKey) // Use TaskCache
	if !found {
		logger.Error("Path to extracted containerd not found in Task Cache", "key", spec.SourceExtractedPathSharedDataKey)
		res.Error = fmt.Errorf("path to extracted containerd not found in Task Cache using key: %s", spec.SourceExtractedPathSharedDataKey)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	extractedPath, typeOk := extractedPathVal.(string)
	if !typeOk || extractedPath == "" {
		logger.Error("Invalid extracted containerd path in Task Cache", "key", spec.SourceExtractedPathSharedDataKey, "value", extractedPathVal)
		res.Error = fmt.Errorf("invalid extracted containerd path in Task Cache (not a string or empty) for key: %s", spec.SourceExtractedPathSharedDataKey)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Using extracted containerd files from.", "path", extractedPath)

	// Install binaries
	for srcRelPath, targetSystemPath := range spec.BinariesToCopy {
		sourceBinaryPath := filepath.Join(extractedPath, srcRelPath)
		targetDir := filepath.Dir(targetSystemPath)
		binaryName := filepath.Base(targetSystemPath)

		logger.Info("Ensuring target directory exists for binary.", "path", targetDir, "binary", binaryName)
		// Assuming connector.Mkdir handles recursive and sudo if needed (based on connector setup)
		if err := conn.Mkdir(goCtx, targetDir, "0755"); err != nil {
			logger.Error("Failed to create target directory", "path", targetDir, "binary", binaryName, "error", err)
			res.Error = fmt.Errorf("failed to create target directory %s for %s: %w", targetDir, binaryName, err)
			res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
		}

		logger.Info("Copying binary.", "source", sourceBinaryPath, "destination", targetSystemPath)
		// Use RunCommand for cp and chmod as connector might not have direct CopyFile/Chmod methods,
		// or their parameters (like sudo) are more complex than RunCommand.
		cpCmd := fmt.Sprintf("cp -fp %s %s", sourceBinaryPath, targetSystemPath)
		_, stderrCp, errCp := conn.RunCommand(goCtx, cpCmd, &connector.ExecOptions{Sudo: true})
		if errCp != nil {
			logger.Error("Failed to copy binary", "source", sourceBinaryPath, "destination", targetSystemPath, "stderr", string(stderrCp), "error", errCp)
			res.Error = fmt.Errorf("failed to copy binary %s to %s (stderr: %s): %w", sourceBinaryPath, targetSystemPath, string(stderrCp), errCp)
			res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
		}

		chmodCmd := fmt.Sprintf("chmod 0755 %s", targetSystemPath)
		_, stderrChmod, errChmod := conn.RunCommand(goCtx, chmodCmd, &connector.ExecOptions{Sudo: true})
		if errChmod != nil {
			logger.Error("Failed to set permissions for binary", "path", targetSystemPath, "stderr", string(stderrChmod), "error", errChmod)
			res.Error = fmt.Errorf("failed to set permissions for %s (stderr: %s): %w", targetSystemPath, string(stderrChmod), errChmod)
			res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
		}
		logger.Info("Binary installed and permissions set.", "binary", binaryName, "path", targetSystemPath)
	}

	// Install systemd unit file
	if spec.SystemdUnitFileTargetPath != "" && spec.SystemdUnitFileSourceRelPath != "" {
		sourceServiceFile := filepath.Join(extractedPath, spec.SystemdUnitFileSourceRelPath)
		targetServiceFileDir := filepath.Dir(spec.SystemdUnitFileTargetPath)

		logger.Info("Ensuring target directory for systemd unit file exists.", "path", targetServiceFileDir)
		if err := conn.Mkdir(goCtx, targetServiceFileDir, "0755"); err != nil {
			logger.Error("Failed to create target directory for systemd unit file", "path", targetServiceFileDir, "error", err)
			res.Error = fmt.Errorf("failed to create target directory %s for systemd unit file: %w", targetServiceFileDir, err)
			res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
		}

		logger.Info("Copying systemd unit file.", "source", sourceServiceFile, "destination", spec.SystemdUnitFileTargetPath)

		// Check if source service file exists on the target host (where extractedPath is)
		srcServiceFileExists, errExist := conn.Exists(goCtx, sourceServiceFile)
		if errExist != nil {
			logger.Warn("Could not verify existence of source systemd file, attempting copy anyway.", "path", sourceServiceFile, "error", errExist)
			// Proceed with copy attempt, might fail if not found.
		}

		if srcServiceFileExists { // Only copy if source exists
			cpCmd := fmt.Sprintf("cp -f %s %s", sourceServiceFile, spec.SystemdUnitFileTargetPath)
			_, stderrCpSvc, errCpSvc := conn.RunCommand(goCtx, cpCmd, &connector.ExecOptions{Sudo: true})
			if errCpSvc != nil {
				logger.Error("Failed to copy systemd unit file", "source", sourceServiceFile, "destination", spec.SystemdUnitFileTargetPath, "stderr", string(stderrCpSvc), "error", errCpSvc)
				res.Error = fmt.Errorf("failed to copy systemd unit file from %s to %s (stderr: %s): %w", sourceServiceFile, spec.SystemdUnitFileTargetPath, string(stderrCpSvc), errCpSvc)
				res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
			}
			chmodCmdSvc := fmt.Sprintf("chmod 0644 %s", spec.SystemdUnitFileTargetPath)
			_, stderrChmodSvc, errChmodSvc := conn.RunCommand(goCtx, chmodCmdSvc, &connector.ExecOptions{Sudo: true})
			if errChmodSvc != nil {
				logger.Error("Failed to set permissions for systemd unit file", "path", spec.SystemdUnitFileTargetPath, "stderr", string(stderrChmodSvc), "error", errChmodSvc)
				res.Error = fmt.Errorf("failed to set permissions for systemd unit file %s (stderr: %s): %w", spec.SystemdUnitFileTargetPath, string(stderrChmodSvc), errChmodSvc)
				res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
			}
			logger.Info("Systemd unit file installed successfully.", "path", spec.SystemdUnitFileTargetPath)
		} else {
			logger.Warn("Source systemd unit file not found in extracted archive. Skipping installation of service file.", "sourcePath", sourceServiceFile, "archivePath", extractedPath)
		}
	} else {
		logger.Info("No systemd unit file source or target path specified. Skipping systemd unit file installation.")
	}

	res.EndTime = time.Now() // Update end time after all operations

	// Post-execution check by calling Check method again
	done, checkErr := e.Check(ctx)
	if checkErr != nil {
		logger.Error("Post-execution check failed.", "error", checkErr)
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed; return res
	}
	if !done {
		logger.Error("Post-execution check indicates containerd installation was not fully successful.")
		res.Error = fmt.Errorf("post-execution check indicates containerd installation was not fully successful")
		res.Status = step.StatusFailed; return res
	}

	res.Message = "Containerd installed successfully from extracted files."
	res.Status = step.StatusSucceeded
	return res
}
var _ step.StepExecutor = &InstallContainerdStepExecutor{}
