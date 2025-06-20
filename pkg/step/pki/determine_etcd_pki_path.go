package pki

import (
	"fmt"
	"os"
	// "path/filepath" // No longer needed for path joining here as full path is received
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/spec"
)

// DefaultEtcdPKIPathKey is used as default for both input and output key for the etcd PKI path.
// It's also used by SetupEtcdPkiDataContextStep to store the path into the module cache.
const DefaultEtcdPKIPathKey = "etcdPKIPath"

// DetermineEtcdPKIPathStepSpec defines parameters for ensuring the etcd PKI path exists.
// This step now expects the full etcd PKI path to be provided via module cache.
type DetermineEtcdPKIPathStepSpec struct {
	// PKIPathToEnsureSharedDataKey is the key to read the fully-determined etcd PKI path from Module Cache.
	// This path is typically set by a setup step (e.g., SetupEtcdPkiDataContextStep) based on global config.
	PKIPathToEnsureSharedDataKey string `json:"pkiPathToEnsureSharedDataKey,omitempty"`

	// OutputPKIPathSharedDataKey is the key to store the same PKI path into Task Cache
	// for subsequent steps within the same task to consume.
	OutputPKIPathSharedDataKey string `json:"outputPKIPathSharedDataKey,omitempty"`
}

// GetName returns the name of the step.
func (s *DetermineEtcdPKIPathStepSpec) GetName() string {
	return "Ensure Etcd PKI Path Exists"
}

// PopulateDefaults sets default values for the spec's cache keys.
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
		return false, fmt.Errorf("StepSpec not found in context for %s Check", "DetermineEtcdPKIPathStep")
	}
	spec, ok := currentFullSpec.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for %s Check: %T", "DetermineEtcdPKIPathStep", currentFullSpec)
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())

	if ctx.Host == nil || ctx.Host.Runner == nil {
		return false, fmt.Errorf("host or runner not available in context for %s Check", spec.GetName())
	}

	pkiPathVal, pathOk := ctx.Module().Get(spec.PKIPathToEnsureSharedDataKey)
	if !pathOk {
		logger.Debugf("Etcd PKI path not found in Module Cache key '%s'. Path determination/setup likely pending.", spec.PKIPathToEnsureSharedDataKey)
		return false, nil // Not an error, just not ready
	}
	pkiPath, ok := pkiPathVal.(string)
	if !ok || pkiPath == "" {
		logger.Warnf("Invalid or empty Etcd PKI path in Module Cache key '%s'.", spec.PKIPathToEnsureSharedDataKey)
		return false, nil // Data issue, but not a hard error for Check, Execute will fail
	}

	// Check if the directory exists using the runner
	exists, err := ctx.Host.Runner.Exists(ctx.GoContext, pkiPath)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of etcd PKI path %s: %w", pkiPath, err)
	}
	if !exists {
		logger.Debugf("Etcd PKI path %s (from Module Cache) does not exist.", pkiPath)
		return false, nil
	}
	// Note: We are not checking if it's a directory here. Mkdirp in Execute handles this.
	// If it exists as a file, Mkdirp will fail, which is desired.
	logger.Debugf("Etcd PKI path %s exists.", pkiPath)

	if val, taskCacheExists := ctx.Task().Get(spec.OutputPKIPathSharedDataKey); taskCacheExists {
		if storedPath, okStr := val.(string); okStr && storedPath == pkiPath {
			logger.Infof("Etcd PKI path %s already available in Task Cache and matches.", pkiPath)
			return true, nil
		}
		logger.Infof("Etcd PKI path in Task Cache (%v) does not match expected path (%s) or is invalid type.", val, pkiPath)
	}
	return false, nil
}

// Execute ensures the pre-determined etcd PKI path exists and stores it in Task Cache.
func (e *DetermineEtcdPKIPathStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for %s Execute", "DetermineEtcdPKIPathStep"))
	}
	spec, ok := currentFullSpec.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for %s Execute: %T", "DetermineEtcdPKIPathStep", currentFullSpec))
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil)

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
	// Use Runner.Mkdirp, assuming pkiPath is on the target host defined by ctx.Host
	// Mode "0700" is appropriate for PKI directories. Sudo set to true as these are often system paths.
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, pkiPath, "0700", true); err != nil {
		res.Error = fmt.Errorf("failed to create etcd PKI directory %s on host %s: %w", pkiPath, ctx.Host.Name, err)
		res.Status = step.StatusFailed; return res
	}
	logger.Infof("Etcd PKI directory ensured at %s on host %s", pkiPath, ctx.Host.Name)

	ctx.Task().Set(spec.OutputPKIPathSharedDataKey, pkiPath)
	logger.Infof("Stored etcd PKI path in Task Cache ('%s'): %s", spec.OutputPKIPathSharedDataKey, pkiPath)

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
	res.Status = step.StatusSucceeded
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&DetermineEtcdPKIPathStepSpec{}), &DetermineEtcdPKIPathStepExecutor{})
}
