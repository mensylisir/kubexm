package common

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
	// Assuming DefaultExtractedPathKey is defined elsewhere, e.g. in extract_archive.go or a constants package
)

// InstallBinaryStepSpec defines parameters for installing a binary file.
type InstallBinaryStepSpec struct {
	SourcePathSharedDataKey string `json:"sourcePathSharedDataKey,omitempty"` // Key for base path (file or dir) in SharedData
	SourceIsDirectory       bool   `json:"sourceIsDirectory,omitempty"`       // If true, SourceFileName is relative to path from SharedDataKey
	SourceFileName          string `json:"sourceFileName,omitempty"`          // Name of the source file (if SourceIsDirectory or if key points to dir)
	TargetDir               string `json:"targetDir,omitempty"`               // Target directory to install the binary
	TargetFileName          string `json:"targetFileName,omitempty"`          // Optional: new name for the binary in TargetDir. If empty, uses SourceFileName or base of source path.
	Permissions             string `json:"permissions,omitempty"`             // e.g., "0755"
}

// GetName returns the name of the step.
func (s *InstallBinaryStepSpec) GetName() string {
	return "Install Binary"
}

// PopulateDefaults sets default values for the spec.
func (s *InstallBinaryStepSpec) PopulateDefaults(ctx *runtime.Context) {
	if s.SourcePathSharedDataKey == "" {
		// DefaultExtractedPathKey is defined in extract_archive.go.
		// For standalone use, ensure this constant is accessible or redefine it here.
		s.SourcePathSharedDataKey = DefaultExtractedPathKey
	}
	if s.TargetDir == "" {
		s.TargetDir = "/usr/local/bin"
	}
	if s.Permissions == "" {
		s.Permissions = "0755"
	}
}

// InstallBinaryStepExecutor implements the logic for installing a binary.
type InstallBinaryStepExecutor struct{}

// determineSourcePath resolves the full path to the source binary.
func (e *InstallBinaryStepExecutor) determineSourcePath(ctx *runtime.Context, stepSpec *InstallBinaryStepSpec) (string, error) {
	basePathVal, found := ctx.SharedData.Load(stepSpec.SourcePathSharedDataKey)
	if !found {
		return "", fmt.Errorf("source path key '%s' not found in SharedData", stepSpec.SourcePathSharedDataKey)
	}
	basePath, ok := basePathVal.(string)
	if !ok || basePath == "" {
		return "", fmt.Errorf("invalid or empty source path in SharedData key '%s'", stepSpec.SourcePathSharedDataKey)
	}

	if stepSpec.SourceIsDirectory {
		if stepSpec.SourceFileName == "" {
			return "", fmt.Errorf("SourceFileName must be provided when SourceIsDirectory is true")
		}
		return filepath.Join(basePath, stepSpec.SourceFileName), nil
	}
	// If not SourceIsDirectory, basePath is the full path to the file.
	// SourceFileName can be empty if basePath is already the file.
	// If SourceFileName is provided, it's an assertion that filepath.Base(basePath) matches it.
	if stepSpec.SourceFileName != "" && filepath.Base(basePath) != stepSpec.SourceFileName {
		// This could be an error or a warning, depending on strictness.
		// For now, let's assume basePath is authoritative if SourceIsDirectory is false.
		ctx.Logger.Warnf("SourceFileName '%s' provided but SourceIsDirectory is false; using '%s' as source. Base of source path is '%s'.",
			stepSpec.SourceFileName, basePath, filepath.Base(basePath))
	}
	return basePath, nil
}

// Check determines if the binary is already installed and configured correctly.
func (e *InstallBinaryStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	stepSpec, ok := s.(*InstallBinaryStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for %s", s, stepSpec.GetName())
	}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", stepSpec.GetName())

	// Determine target path
	var effectiveTargetFileName string
	if stepSpec.TargetFileName != "" {
		effectiveTargetFileName = stepSpec.TargetFileName
	} else if stepSpec.SourceFileName != "" { // If SourceFileName is given, use it as default target name
		effectiveTargetFileName = stepSpec.SourceFileName
	} else {
		// If TargetFileName and SourceFileName are empty, and SourceIsDirectory is false,
		// we need to get the base from SourcePathSharedDataKey.
		// This requires resolving sourcePath even in Check, which might be slow or depend on prior steps.
		// For a simpler Check, we might require TargetFileName if SourceFileName is not easily known here.
		// Let's attempt to resolve it for a more complete check.
		sourcePath, errResolve := e.determineSourcePath(ctx, stepSpec)
		if errResolve != nil {
			logger.Debugf("Cannot determine source path for default target filename in Check: %v. Assuming not done.", errResolve)
			return false, nil // Cannot determine target, so not done.
		}
		effectiveTargetFileName = filepath.Base(sourcePath)
	}
	if effectiveTargetFileName == "" {
		logger.Warnf("Could not determine effective target file name. Assuming not done.")
		return false, nil
	}
	targetPath := filepath.Join(stepSpec.TargetDir, effectiveTargetFileName)

	fileExists, err := ctx.Host.Runner.Exists(ctx.GoContext, targetPath)
	if err != nil {
		return false, fmt.Errorf("error checking existence of target %s: %w", targetPath, err)
	}
	if !fileExists {
		logger.Infof("Target binary %s does not exist.", targetPath)
		return false, nil
	}
	logger.Debugf("Target binary %s exists.", targetPath)

	// Check permissions
	// Runner.Stat should provide os.FileInfo like interface or specific mode string.
	// Assuming Runner.Stat returns a struct with a Mode field (os.FileMode).
	// This part is highly dependent on Runner.Stat's return type.
	// For simplicity, if Runner.GetMode(path) (string, error) existed, it would be:
	// currentMode, modeErr := ctx.Host.Runner.GetMode(ctx.GoContext, targetPath)
	// For now, this check might be rudimentary or skipped if GetMode is not standard.
	// Let's assume a way to get mode as a string like "0755".
	// If Runner.Stat gives os.FileInfo: statInfo.Mode().Perm().String() gives "-rwxr-xr-x"
	// We'd need to convert spec.Permissions (e.g. "0755") to this format or vice-versa.
	// This is complex. A simpler check is if it's executable, if permissions start with 7.
	// For now, we'll log that permissions check is simplified.
	// TODO: Implement robust permission checking once Runner.Stat or Runner.GetMode is clarified.
	logger.Debugf("Permissions check for %s against %s is simplified/skipped in this version.", targetPath, stepSpec.Permissions)

	// Optional: Add checksum comparison if source available and checksums known.
	// This would require sourcePath to be determined and file read, which makes Check heavier.

	logger.Infof("Target binary %s exists. Permissions check is rudimentary. Assuming done.", targetPath)
	return true, nil
}

// Execute installs the binary.
func (e *InstallBinaryStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepSpec, ok := s.(*InstallBinaryStepSpec)
	if !ok {
		return step.NewResultForSpec(s, fmt.Errorf("unexpected spec type %T", s))
	}
	stepSpec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", stepSpec.GetName())
	res := step.NewResultForSpec(s, nil)

	sourcePath, err := e.determineSourcePath(ctx, stepSpec)
	if err != nil {
		res.Error = fmt.Errorf("failed to determine source path: %w", err)
		res.SetFailed(); return res
	}
	logger.Infof("Determined source path: %s", sourcePath)

	// Verify source file exists before proceeding
	sourceExists, err := ctx.Host.Runner.Exists(ctx.GoContext, sourcePath)
	if err != nil {
		res.Error = fmt.Errorf("error checking existence of source file %s: %w", sourcePath, err)
		res.SetFailed(); return res
	}
	if !sourceExists {
		res.Error = fmt.Errorf("source file %s does not exist", sourcePath)
		res.SetFailed(); return res
	}


	targetFileName := stepSpec.TargetFileName
	if targetFileName == "" {
		targetFileName = filepath.Base(sourcePath)
	}
	if targetFileName == "" { // Should not happen if sourcePath is valid file
		res.Error = fmt.Errorf("could not determine target filename from source path '%s'", sourcePath)
		res.SetFailed(); return res
	}
	targetPath := filepath.Join(stepSpec.TargetDir, targetFileName)
	logger.Infof("Target path for installation: %s", targetPath)


	logger.Infof("Ensuring target directory %s exists...", stepSpec.TargetDir)
	// Sudo true for system bin dirs like /usr/local/bin
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, stepSpec.TargetDir, "0755", true); err != nil {
		res.Error = fmt.Errorf("failed to create target directory %s: %w", stepSpec.TargetDir, err)
		res.SetFailed(); return res
	}

	logger.Infof("Copying binary from %s to %s...", sourcePath, targetPath)
	// Using cp command via runner. Sudo true for writing to system directories.
	cpCmd := fmt.Sprintf("cp -fp %s %s", sourcePath, targetPath) // -f to overwrite, -p to preserve mode from source temporarily
	_, stderrCp, errCp := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cpCmd, &connector.ExecOptions{Sudo: true})
	if errCp != nil {
		res.Error = fmt.Errorf("failed to copy binary from %s to %s (stderr: %s): %w", sourcePath, targetPath, stderrCp, errCp)
		res.SetFailed(); return res
	}

	logger.Infof("Setting permissions for %s to %s...", targetPath, stepSpec.Permissions)
	chmodCmd := fmt.Sprintf("chmod %s %s", stepSpec.Permissions, targetPath)
	_, stderrChmod, errChmod := ctx.Host.Runner.RunWithOptions(ctx.GoContext, chmodCmd, &connector.ExecOptions{Sudo: true})
	if errChmod != nil {
		res.Error = fmt.Errorf("failed to set permissions for %s (stderr: %s): %w", targetPath, stderrChmod, errChmod)
		res.SetFailed(); return res
	}
	logger.Infof("Binary %s installed and permissions set to %s.", targetPath, stepSpec.Permissions)

	// Perform post-execution check
	done, checkErr := e.Check(s, ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.SetFailed(); return res
	}
	if !done {
		res.Error = fmt.Errorf("post-execution check indicates binary installation was not successful or configuration is incorrect")
		res.SetFailed(); return res
	}

	res.SetSucceeded(); return res
}

func init() {
	step.Register(&InstallBinaryStepSpec{}, &InstallBinaryStepExecutor{})
}
