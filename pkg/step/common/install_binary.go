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
func (e *InstallBinaryStepExecutor) determineSourcePath(ctx runtime.Context, stepSpec *InstallBinaryStepSpec) (string, error) {
	basePathVal, found := ctx.Task().Get(stepSpec.SourcePathSharedDataKey)
	if !found {
		return "", fmt.Errorf("source path key '%s' not found in Task Cache", stepSpec.SourcePathSharedDataKey)
	}
	basePath, ok := basePathVal.(string)
	if !ok || basePath == "" {
		return "", fmt.Errorf("invalid or empty source path in Task Cache key '%s'", stepSpec.SourcePathSharedDataKey)
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
func (e *InstallBinaryStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for InstallBinaryStep Check")
	}
	spec, ok := currentFullSpec.(*InstallBinaryStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for InstallBinaryStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())

	// Determine target path
	var effectiveTargetFileName string
	if spec.TargetFileName != "" {
		effectiveTargetFileName = spec.TargetFileName
	} else if spec.SourceFileName != "" { // If SourceFileName is given, use it as default target name
		effectiveTargetFileName = spec.SourceFileName
	} else {
		sourcePath, errResolve := e.determineSourcePath(ctx, spec)
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
	targetPath := filepath.Join(spec.TargetDir, effectiveTargetFileName)

	fileExists, err := ctx.Host.Runner.Exists(ctx.GoContext, targetPath)
	if err != nil {
		return false, fmt.Errorf("error checking existence of target %s: %w", targetPath, err)
	}
	if !fileExists {
		logger.Infof("Target binary %s does not exist.", targetPath)
		return false, nil
	}
	logger.Debugf("Target binary %s exists.", targetPath)

	logger.Debugf("Permissions check for %s against %s is simplified/skipped in this version.", targetPath, spec.Permissions)

	logger.Infof("Target binary %s exists. Permissions check is rudimentary. Assuming done.", targetPath)
	return true, nil
}

// Execute installs the binary.
func (e *InstallBinaryStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for InstallBinaryStep Execute"))
	}
	spec, ok := currentFullSpec.(*InstallBinaryStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for InstallBinaryStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults(ctx)
	logger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	sourcePath, err := e.determineSourcePath(ctx, spec)
	if err != nil {
		res.Error = fmt.Errorf("failed to determine source path: %w", err)
		res.Status = step.StatusFailed; return res
	}
	logger.Infof("Determined source path: %s", sourcePath)

	sourceExists, err := ctx.Host.Runner.Exists(ctx.GoContext, sourcePath)
	if err != nil {
		res.Error = fmt.Errorf("error checking existence of source file %s: %w", sourcePath, err)
		res.Status = step.StatusFailed; return res
	}
	if !sourceExists {
		res.Error = fmt.Errorf("source file %s does not exist", sourcePath)
		res.Status = step.StatusFailed; return res
	}

	targetFileName := spec.TargetFileName
	if targetFileName == "" {
		targetFileName = filepath.Base(sourcePath)
	}
	if targetFileName == "" {
		res.Error = fmt.Errorf("could not determine target filename from source path '%s'", sourcePath)
		res.Status = step.StatusFailed; return res
	}
	targetPath := filepath.Join(spec.TargetDir, targetFileName)
	logger.Infof("Target path for installation: %s", targetPath)

	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, spec.TargetDir, "0755", true); err != nil {
		res.Error = fmt.Errorf("failed to create target directory %s: %w", spec.TargetDir, err)
		res.Status = step.StatusFailed; return res
	}

	logger.Infof("Copying binary from %s to %s...", sourcePath, targetPath)
	cpCmd := fmt.Sprintf("cp -fp %s %s", sourcePath, targetPath)
	_, stderrCp, errCp := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cpCmd, &connector.ExecOptions{Sudo: true})
	if errCp != nil {
		res.Error = fmt.Errorf("failed to copy binary from %s to %s (stderr: %s): %w", sourcePath, targetPath, stderrCp, errCp)
		res.Status = step.StatusFailed; return res
	}

	logger.Infof("Setting permissions for %s to %s...", targetPath, spec.Permissions)
	chmodCmd := fmt.Sprintf("chmod %s %s", spec.Permissions, targetPath)
	_, stderrChmod, errChmod := ctx.Host.Runner.RunWithOptions(ctx.GoContext, chmodCmd, &connector.ExecOptions{Sudo: true})
	if errChmod != nil {
		res.Error = fmt.Errorf("failed to set permissions for %s (stderr: %s): %w", targetPath, stderrChmod, errChmod)
		res.Status = step.StatusFailed; return res
	}
	logger.Infof("Binary %s installed and permissions set to %s.", targetPath, spec.Permissions)

	done, checkErr := e.Check(ctx) // Pass context
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed; return res
	}
	if !done {
		res.Error = fmt.Errorf("post-execution check indicates binary installation was not successful or configuration is incorrect")
		res.Status = step.StatusFailed; return res
	}

	// res.SetSucceeded(); // Status is set by NewResult if err is nil
	return res
}

func init() {
	step.Register(&InstallBinaryStepSpec{}, &InstallBinaryStepExecutor{})
}
