package containerd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"os" // For os.Stat in Check

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
func (e *InstallContainerdStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for InstallContainerdStep Check")
	}
	spec, ok := currentFullSpec.(*InstallContainerdStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for InstallContainerdStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())

	// Check if all target binaries exist
	for _, targetPath := range spec.BinariesToCopy {
		exists, errExists := ctx.Host.Runner.Exists(ctx.GoContext, targetPath)
		if errExists != nil {
			return false, fmt.Errorf("failed to check existence of %s: %w", targetPath, errExists)
		}
		if !exists {
			logger.Debugf("Target binary %s does not exist.", targetPath)
			return false, nil
		}
		// TODO: Could add permission check here if critical for Check phase.
	}
	logger.Debug("All target binaries exist.")

	// Check if systemd unit file exists
	if spec.SystemdUnitFileTargetPath != "" {
		exists, errExists := ctx.Host.Runner.Exists(ctx.GoContext, spec.SystemdUnitFileTargetPath)
		if errExists != nil {
			return false, fmt.Errorf("failed to check existence of systemd unit file %s: %w", spec.SystemdUnitFileTargetPath, errExists)
		}
		if !exists {
			logger.Debugf("Systemd unit file %s does not exist.", spec.SystemdUnitFileTargetPath)
			return false, nil
		}
		logger.Debugf("Systemd unit file %s exists.", spec.SystemdUnitFileTargetPath)
	}

	logger.Infof("Containerd installation (binaries and service file) appears complete.")
	return true, nil
}

// Execute installs containerd from pre-extracted files.
func (e *InstallContainerdStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for InstallContainerdStep Execute"))
	}
	spec, ok := currentFullSpec.(*InstallContainerdStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for InstallContainerdStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	if ctx.Host == nil || ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("host or runner not available in context"); res.Status = step.StatusFailed; return res
	}

	extractedPathVal, found := ctx.Task().Get(spec.SourceExtractedPathSharedDataKey)
	if !found {
		res.Error = fmt.Errorf("path to extracted containerd not found in Task Cache using key: %s", spec.SourceExtractedPathSharedDataKey)
		res.Status = step.StatusFailed; return res
	}
	extractedPath, typeOk := extractedPathVal.(string)
	if !typeOk || extractedPath == "" {
		res.Error = fmt.Errorf("invalid extracted containerd path in Task Cache (not a string or empty) for key: %s", spec.SourceExtractedPathSharedDataKey)
		res.Status = step.StatusFailed; return res
	}
	logger.Infof("Using extracted containerd files from: %s", extractedPath)

	// Install binaries
	for srcRelPath, targetSystemPath := range spec.BinariesToCopy {
		sourceBinaryPath := filepath.Join(extractedPath, srcRelPath)
		targetDir := filepath.Dir(targetSystemPath)
		binaryName := filepath.Base(targetSystemPath) // Should match key in map if map value is full path

		logger.Infof("Ensuring target directory %s for binary %s exists...", targetDir, binaryName)
		if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, targetDir, "0755", true); err != nil {
			res.Error = fmt.Errorf("failed to create target directory %s for %s: %w", targetDir, binaryName, err)
			res.Status = step.StatusFailed; return res
		}

		logger.Infof("Copying binary from %s to %s...", sourceBinaryPath, targetSystemPath)
		cpCmd := fmt.Sprintf("cp -fp %s %s", sourceBinaryPath, targetSystemPath)
		_, stderrCp, errCp := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cpCmd, &connector.ExecOptions{Sudo: true})
		if errCp != nil {
			res.Error = fmt.Errorf("failed to copy binary %s to %s (stderr: %s): %w", sourceBinaryPath, targetSystemPath, stderrCp, errCp)
			res.Status = step.StatusFailed; return res
		}

		chmodCmd := fmt.Sprintf("chmod 0755 %s", targetSystemPath)
		_, stderrChmod, errChmod := ctx.Host.Runner.RunWithOptions(ctx.GoContext, chmodCmd, &connector.ExecOptions{Sudo: true})
		if errChmod != nil {
			res.Error = fmt.Errorf("failed to set permissions for %s (stderr: %s): %w", targetSystemPath, stderrChmod, errChmod)
			res.Status = step.StatusFailed; return res
		}
		logger.Infof("Binary %s installed to %s with permissions 0755.", binaryName, targetSystemPath)
	}

	// Install systemd unit file
	if spec.SystemdUnitFileTargetPath != "" && spec.SystemdUnitFileSourceRelPath != "" {
		sourceServiceFile := filepath.Join(extractedPath, spec.SystemdUnitFileSourceRelPath)
		targetServiceFileDir := filepath.Dir(spec.SystemdUnitFileTargetPath)

		logger.Infof("Ensuring target directory %s for systemd unit file exists...", targetServiceFileDir)
		if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, targetServiceFileDir, "0755", true); err != nil {
			res.Error = fmt.Errorf("failed to create target directory %s for systemd unit file: %w", targetServiceFileDir, err)
			res.Status = step.StatusFailed; return res
		}

		logger.Infof("Copying systemd unit file from %s to %s...", sourceServiceFile, spec.SystemdUnitFileTargetPath)
		// Check if source service file exists first
		srcServiceFileExists, errExist := ctx.Host.Runner.Exists(ctx.GoContext, sourceServiceFile) // This checks remote path, source is local to extracted archive
		if errExist != nil {
			// This check is problematic if extractedPath is remote, but for containerd it's usually local after download.
			// Assuming extractedPath is accessible for a stat/check from where this runner op is executed.
			// If utils.DownloadFile puts it on target host, then this check is fine.
			logger.Warnf("Could not verify existence of source systemd file %s: %v", sourceServiceFile, errExist)
		}
		if srcServiceFileExists { // Only copy if source exists
			cpCmd := fmt.Sprintf("cp -f %s %s", sourceServiceFile, spec.SystemdUnitFileTargetPath)
			_, stderrCpSvc, errCpSvc := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cpCmd, &connector.ExecOptions{Sudo: true})
			if errCpSvc != nil {
				res.Error = fmt.Errorf("failed to copy systemd unit file from %s to %s (stderr: %s): %w", sourceServiceFile, spec.SystemdUnitFileTargetPath, stderrCpSvc, errCpSvc)
				res.Status = step.StatusFailed; return res
			}
			chmodCmdSvc := fmt.Sprintf("chmod 0644 %s", spec.SystemdUnitFileTargetPath)
			_, stderrChmodSvc, errChmodSvc := ctx.Host.Runner.RunWithOptions(ctx.GoContext, chmodCmdSvc, &connector.ExecOptions{Sudo: true})
			if errChmodSvc != nil {
				res.Error = fmt.Errorf("failed to set permissions for systemd unit file %s (stderr: %s): %w", spec.SystemdUnitFileTargetPath, stderrChmodSvc, errChmodSvc)
				res.Status = step.StatusFailed; return res
			}
			logger.Infof("Systemd unit file %s installed successfully.", spec.SystemdUnitFileTargetPath)
		} else {
			logger.Warnf("Source systemd unit file %s not found in extracted archive at %s. Skipping installation of service file.", spec.SystemdUnitFileSourceRelPath, extractedPath)
			// This might not be a fatal error for the step if binaries are copied.
			// Depending on requirements, this could be res.Error and StatusFailed.
		}
	} else {
		logger.Info("No systemd unit file source or target path specified. Skipping systemd unit file installation.")
	}

	// Post-execution check
	done, checkErr := e.Check(ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed; return res
	}
	if !done {
		res.Error = fmt.Errorf("post-execution check indicates containerd installation was not fully successful")
		res.Status = step.StatusFailed; return res
	}

	res.Message = "Containerd installed successfully from extracted files."
	res.Status = step.StatusSucceeded
	return res
}
var _ step.StepExecutor = &InstallContainerdStepExecutor{}
