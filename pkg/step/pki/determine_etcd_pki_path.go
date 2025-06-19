package pki

import (
	"fmt"
	"os"
	// "path/filepath" // No longer needed for path joining here
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// SharedData keys related to PKI paths.
const (
	DefaultEtcdPKIPathKey = "etcdPKIPath" // Used as default for both input and output key
)

// DetermineEtcdPKIPathStepSpec defines parameters for ensuring the etcd PKI path exists.
type DetermineEtcdPKIPathStepSpec struct {
	// Input key to read the already fully-determined etcd PKI path from Module Cache.
	PKIPathToEnsureSharedDataKey string `json:"pkiPathToEnsureSharedDataKey,omitempty"`
	// Output key to store the same PKI path into Task Cache for subsequent steps in the same task.
	OutputPKIPathSharedDataKey string `json:"outputPKIPathSharedDataKey,omitempty"`
}

// GetName returns the name of the step.
func (s *DetermineEtcdPKIPathStepSpec) GetName() string {
	return "Ensure Etcd PKI Path Exists" // Name changed to reflect new role
}

// PopulateDefaults sets default values for the spec.
func (s *DetermineEtcdPKIPathStepSpec) PopulateDefaults() {
	if s.PKIPathToEnsureSharedDataKey == "" {
		s.PKIPathToEnsureSharedDataKey = DefaultEtcdPKIPathKey
	}
	if s.OutputPKIPathSharedDataKey == "" {
		s.OutputPKIPathSharedDataKey = DefaultEtcdPKIPathKey
	}
}

// DetermineEtcdPKIPathStepExecutor implements the logic.
type DetermineEtcdPKIPathStepExecutor struct{}

// Check determines if the etcd PKI path (read from module cache) exists and is in task cache.
func (e *DetermineEtcdPKIPathStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for %s Check", DefaultEtcdPKIPathKey) // Updated step name
	}
	spec, ok := currentFullSpec.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for %s Check: %T", DefaultEtcdPKIPathKey, currentFullSpec)
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())

	// Get the pre-determined etcd PKI path from Module Cache
	pkiPathVal, pathOk := ctx.Module().Get(spec.PKIPathToEnsureSharedDataKey)
	if !pathOk {
		logger.Debugf("Etcd PKI path not found in Module Cache key '%s'. Path determination/setup likely pending.", spec.PKIPathToEnsureSharedDataKey)
		return false, nil
	}
	pkiPath, ok := pkiPathVal.(string)
	if !ok || pkiPath == "" {
		logger.Warnf("Invalid or empty Etcd PKI path in Module Cache key '%s'.", spec.PKIPathToEnsureSharedDataKey)
		return false, nil
	}

	stat, statErr := os.Stat(pkiPath) // This operates on the local filesystem
	if os.IsNotExist(statErr) {
		logger.Debugf("Etcd PKI path %s (from Module Cache) does not exist.", pkiPath)
		return false, nil
	}
	if statErr != nil {
		return false, fmt.Errorf("failed to stat etcd PKI path %s: %w", pkiPath, statErr)
	}
	if !stat.IsDir() {
		return false, fmt.Errorf("etcd PKI path %s is not a directory", pkiPath)
	}
	logger.Debugf("Etcd PKI directory %s exists.", pkiPath)

	// Check if it's also already in Task Cache with the correct value (for idempotency of Execute)
	if val, exists := ctx.Task().Get(spec.OutputPKIPathSharedDataKey); exists {
		if storedPath, okStr := val.(string); okStr && storedPath == pkiPath {
			logger.Infof("Etcd PKI path %s already available in Task Cache and matches.", pkiPath)
			return true, nil
		}
		logger.Infof("Etcd PKI path in Task Cache (%v) does not match expected path (%s) or is invalid type.", val, pkiPath)
	}
	return false, nil // Not in Task Cache or mismatch, ensure Execute runs to set/confirm it.
}

// Execute ensures the pre-determined etcd PKI path exists and stores it in Task Cache.
func (e *DetermineEtcdPKIPathStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for %s Execute", DefaultEtcdPKIPathKey))
	}
	spec, ok := currentFullSpec.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for %s Execute: %T", DefaultEtcdPKIPathKey, currentFullSpec))
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	// Get the pre-determined etcd PKI path from Module Cache
	pkiPathVal, pathOk := ctx.Module().Get(spec.PKIPathToEnsureSharedDataKey)
	if !pathOk {
		res.Error = fmt.Errorf("etcd PKI path not found in Module Cache key '%s'. Ensure SetupEtcdPkiDataContextStep ran successfully.", spec.PKIPathToEnsureSharedDataKey)
		res.Status = step.StatusFailed; return res
	}
	pkiPath, typeOk := pkiPathVal.(string)
	if !typeOk || pkiPath == "" {
		res.Error = fmt.Errorf("invalid or empty etcd PKI path in Module Cache key '%s'", spec.PKIPathToEnsureSharedDataKey)
		res.Status = step.StatusFailed; return res
	}

	logger.Infof("Ensuring etcd PKI directory (from Module Cache) exists: %s", pkiPath)
	if err := os.MkdirAll(pkiPath, 0700); err != nil { // Permissions 0700 as PKI material is sensitive.
		res.Error = fmt.Errorf("failed to create etcd PKI directory %s: %w", pkiPath, err)
		res.Status = step.StatusFailed; return res
	}
	logger.Infof("Etcd PKI directory ensured at %s", pkiPath)

	// Store the path in Task Cache for subsequent steps within the same task.
	ctx.Task().Set(spec.OutputPKIPathSharedDataKey, pkiPath)
	logger.Infof("Stored etcd PKI path in Task Cache ('%s'): %s", spec.OutputPKIPathSharedDataKey, pkiPath)

	// Perform post-execution check
	done, checkErr := e.Check(ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed; return res
	}
	if !done {
		res.Error = fmt.Errorf("post-execution check indicates Etcd PKI Path was not correctly ensured or cached in Task Cache")
		res.Status = step.StatusFailed; return res
	}

	res.Message = fmt.Sprintf("Etcd PKI path %s ensured and available in Task Cache.", pkiPath)
	return res
}

func init() {
	step.Register(&DetermineEtcdPKIPathStepSpec{}, &DetermineEtcdPKIPathStepExecutor{})
}
