package pki

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
	// Assuming DefaultEtcdPKIPathKey is available from another file in this package or a constants package
)

// SharedData key for the list of copied external etcd certificate file basenames.
const (
	DefaultEtcdExternalCopiedFilesListKey = "etcdExternalCopiedCertFiles"
)

// PrepareExternalEtcdCertsStepSpec defines parameters for preparing external etcd certificates.
type PrepareExternalEtcdCertsStepSpec struct {
	TargetPKIPathSharedDataKey string `json:"targetPKIPathSharedDataKey,omitempty"` // Local PKI path (input)
	ExternalEtcdCAFile         string `json:"externalEtcdCAFile,omitempty"`         // Path to user's external CA file
	ExternalEtcdCertFile       string `json:"externalEtcdCertFile,omitempty"`       // Path to user's external Etcd cert file
	ExternalEtcdKeyFile        string `json:"externalEtcdKeyFile,omitempty"`        // Path to user's external Etcd key file
	OutputCopiedFilesListKey   string `json:"outputCopiedFilesListKey,omitempty"`   // SharedData key for list of copied file basenames
}

// GetName returns the name of the step.
func (s *PrepareExternalEtcdCertsStepSpec) GetName() string {
	return "Prepare External Etcd Certificates"
}

// PopulateDefaults sets default values for the spec.
func (s *PrepareExternalEtcdCertsStepSpec) PopulateDefaults() {
	if s.TargetPKIPathSharedDataKey == "" {
		s.TargetPKIPathSharedDataKey = DefaultEtcdPKIPathKey // From determine_etcd_pki_path.go
	}
	if s.OutputCopiedFilesListKey == "" {
		s.OutputCopiedFilesListKey = DefaultEtcdExternalCopiedFilesListKey
	}
	// ExternalEtcd*File paths must be provided by the user via configuration; no defaults here.
}

// PrepareExternalEtcdCertsStepExecutor implements the logic.
type PrepareExternalEtcdCertsStepExecutor struct{}

// Check determines if external etcd certificates seem to have been already copied and configured.
func (e *PrepareExternalEtcdCertsStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for PrepareExternalEtcdCertsStep Check")
	}
	spec, ok := currentFullSpec.(*PrepareExternalEtcdCertsStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for PrepareExternalEtcdCertsStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName()) // No host context for local operations

	// If no external files are specified, this step is effectively "done" or "not applicable".
	if spec.ExternalEtcdCAFile == "" && spec.ExternalEtcdCertFile == "" && spec.ExternalEtcdKeyFile == "" {
		logger.Info("No external etcd certificate files specified in spec. Step is considered done/not applicable.")
		val, listExists := ctx.Task().Get(spec.OutputCopiedFilesListKey)
		if !listExists {
			ctx.Task().Set(spec.OutputCopiedFilesListKey, []string{})
		} else {
			list, ok := val.([]string)
			if !ok || len(list) != 0 {
				logger.Debug("OutputCopiedFilesListKey in Task Cache contains items or is wrong type, but no external files specified. Re-running Execute to clarify state.")
				return false, nil
			}
		}
		return true, nil
	}

	targetPKIPathVal, pkiPathOk := ctx.Task().Get(spec.TargetPKIPathSharedDataKey)
	if !pkiPathOk {
		logger.Debugf("Target PKI path not found in Task Cache key '%s'. Cannot check.", spec.TargetPKIPathSharedDataKey)
		return false, nil
	}
	targetPKIPath, ok := targetPKIPathVal.(string)
	if !ok || targetPKIPath == "" {
		logger.Warnf("Invalid or empty target PKI path in Task Cache key '%s'.", spec.TargetPKIPathSharedDataKey)
		return false, nil
	}

	expectedFileBaseNames := []string{}
	sourceFileMap := map[string]string{
		"CA":   stepSpec.ExternalEtcdCAFile,
		"Cert": stepSpec.ExternalEtcdCertFile,
		"Key":  stepSpec.ExternalEtcdKeyFile,
	}

	for fileType, relativeSrcPath := range sourceFileMap {
		if relativeSrcPath == "" {
			continue
		}
		// Source path is relative to where KubeKey is run or an absolute path.
		// For Check, we only care about the destination.
		dstFileName := filepath.Base(relativeSrcPath)
		expectedFileBaseNames = append(expectedFileBaseNames, dstFileName)
		localPath := filepath.Join(targetPKIPath, dstFileName)

		stat, statErr := os.Stat(localPath) // Local filesystem check
		if os.IsNotExist(statErr) {
			logger.Infof("Expected external etcd file %s (for %s) not found at local path %s.", dstFileName, fileType, localPath)
			return false, nil
		}
		if statErr != nil {
			return false, fmt.Errorf("failed to stat external etcd file %s at %s: %w", dstFileName, localPath, statErr)
		}
		// Check permissions (mode 0600)
		if stat.Mode().Perm() != 0600 {
			logger.Warnf("External etcd file %s at %s has incorrect permissions (%s), expected 0600.", dstFileName, localPath, stat.Mode().Perm().String())
			return false, nil // Strict check for permissions
		}
	}
	sort.Strings(expectedFileBaseNames) // Sort for consistent comparison later

	// Check if Task Cache output list is populated and matches
	fetchedFilesRaw, listExists := ctx.Task().Get(spec.OutputCopiedFilesListKey)
	if !listExists {
		logger.Debugf("List of copied files (key: '%s') not found in Task Cache. Re-run needed.", spec.OutputCopiedFilesListKey)
		return false, nil
	}
	actualCopiedFiles, ok := fetchedFilesRaw.([]string)
	if !ok {
		return false, fmt.Errorf("invalid type for copied files list in Task Cache key '%s', expected []string", spec.OutputCopiedFilesListKey)
	}
	sort.Strings(actualCopiedFiles)

	if len(expectedFileBaseNames) != len(actualCopiedFiles) {
		logger.Infof("Mismatch in number of expected (%d) vs actual (%d) copied files in Task Cache. Expected: %v, Actual: %v",
			len(expectedFileBaseNames), len(actualCopiedFiles), expectedFileBaseNames, actualCopiedFiles)
		return false, nil
	}
	for i := range expectedFileBaseNames {
		if expectedFileBaseNames[i] != actualCopiedFiles[i] {
			logger.Infof("Mismatch in content of copied files list in Task Cache. Expected: %v, Actual: %v", expectedFileBaseNames, actualCopiedFiles)
			return false, nil
		}
	}

	logger.Infof("All specified external etcd certificate files exist in %s with correct permissions and Task Cache is up-to-date.", targetPKIPath)
	return true, nil
}

// Execute copies user-provided external etcd certificates to the local PKI directory.
func (e *PrepareExternalEtcdCertsStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for PrepareExternalEtcdCertsStep Execute"))
	}
	spec, ok := currentFullSpec.(*PrepareExternalEtcdCertsStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for PrepareExternalEtcdCertsStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil) // Local operation

	// Handle the case where no external certs are provided.
	if spec.ExternalEtcdCAFile == "" && spec.ExternalEtcdCertFile == "" && spec.ExternalEtcdKeyFile == "" {
		logger.Info("No external etcd certificate files specified. Nothing to copy.")
		ctx.Task().Set(spec.OutputCopiedFilesListKey, []string{}) // Store empty list
		// res.SetSucceeded() // Status is set by NewResult
		return res
	}

	targetPKIPathVal, pkiPathOk := ctx.Task().Get(spec.TargetPKIPathSharedDataKey)
	if !pkiPathOk {
		res.Error = fmt.Errorf("target PKI path for storing certs not found in Task Cache key '%s'", spec.TargetPKIPathSharedDataKey)
		res.Status = step.StatusFailed; return res
	}
	targetPKIPath, typeOk := targetPKIPathVal.(string)
	if !typeOk || targetPKIPath == "" {
		res.Error = fmt.Errorf("invalid or empty target PKI path in Task Cache key '%s'", spec.TargetPKIPathSharedDataKey)
		res.Status = step.StatusFailed; return res
	}

	if err := os.MkdirAll(targetPKIPath, 0700); err != nil { // Local filesystem operation
		res.Error = fmt.Errorf("failed to create local target PKI directory %s: %w", targetPKIPath, err)
		res.Status = step.StatusFailed; return res
	}
	logger.Infof("Ensured local PKI directory exists: %s", targetPKIPath)

	sourceFiles := map[string]string{
		"CA":   stepSpec.ExternalEtcdCAFile,
		"Cert": stepSpec.ExternalEtcdCertFile,
		"Key":  stepSpec.ExternalEtcdKeyFile,
	}
	copiedFileBaseNames := []string{}
	var errorsEncountered []string

	for fileType, srcFileRelativePath := range sourceFiles {
		if srcFileRelativePath == "" {
			logger.Debugf("No source path provided for external etcd file type '%s'. Skipping.", fileType)
			continue
		}

		absSrcPath, err := filepath.Abs(srcFileRelativePath)
		if err != nil {
			errMsg := fmt.Sprintf("failed to get absolute path for source %s file %s: %v", fileType, srcFileRelativePath, err)
			logger.Warn(errMsg)
			errorsEncountered = append(errorsEncountered, errMsg)
			continue
		}

		if _, statErr := os.Stat(absSrcPath); os.IsNotExist(statErr) {
			errMsg := fmt.Sprintf("source %s file %s (resolved to %s) does not exist", fileType, srcFileRelativePath, absSrcPath)
			logger.Warn(errMsg)
			errorsEncountered = append(errorsEncountered, errMsg)
			continue
		} else if statErr != nil {
			errMsg := fmt.Sprintf("failed to stat source %s file %s: %v", fileType, absSrcPath, statErr)
			logger.Warn(errMsg)
			errorsEncountered = append(errorsEncountered, errMsg)
			continue
		}

		dstFileName := filepath.Base(absSrcPath) // Use basename of source for destination filename
		dstPath := filepath.Join(targetPKIPath, dstFileName)

		logger.Infof("Copying external etcd %s file from %s to %s", fileType, absSrcPath, dstPath)
		data, readErr := os.ReadFile(absSrcPath)
		if readErr != nil {
			errMsg := fmt.Sprintf("failed to read source %s file %s: %v", fileType, absSrcPath, readErr)
			logger.Warn(errMsg)
			errorsEncountered = append(errorsEncountered, errMsg)
			continue
		}

		// Write with 0600 permissions as these are sensitive PKI files.
		writeErr := os.WriteFile(dstPath, data, 0600)
		if writeErr != nil {
			errMsg := fmt.Sprintf("failed to write destination %s file %s: %v", fileType, dstPath, writeErr)
			logger.Warn(errMsg)
			errorsEncountered = append(errorsEncountered, errMsg)
			continue
		}
		logger.Successf("Successfully copied external etcd %s file to %s", fileType, dstPath)
		copiedFileBaseNames = append(copiedFileBaseNames, dstFileName)
	}

	sort.Strings(copiedFileBaseNames) // Sort for consistent storage and checking
	ctx.Task().Set(spec.OutputCopiedFilesListKey, copiedFileBaseNames)
	logger.Infof("List of copied external etcd file basenames stored in Task Cache key '%s': %v", spec.OutputCopiedFilesListKey, copiedFileBaseNames)

	if len(errorsEncountered) > 0 {
		// If any source file was specified but failed to be copied, it's an issue.
		res.Error = fmt.Errorf("encountered %d error(s) while preparing external etcd certificates: %s", len(errorsEncountered), strings.Join(errorsEncountered, "; "))
		// Set as failed if any errors occurred that prevented copying a specified file.
		res.Status = step.StatusFailed; return res
	}

	// Perform post-execution check
	done, checkErr := e.Check(ctx) // Pass context
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed; return res
	}
	if !done {
		res.Error = fmt.Errorf("post-execution check indicates external etcd certs preparation was not fully successful")
		res.Status = step.StatusFailed; return res
	}

	res.Message = fmt.Sprintf("Prepared %d external etcd certificate files in %s.", len(copiedFileBaseNames), targetPKIPath)
	// res.SetSucceeded() // Status is set by NewResult if err is nil
	return res
}

func init() {
	step.Register(&PrepareExternalEtcdCertsStepSpec{}, &PrepareExternalEtcdCertsStepExecutor{})
}
