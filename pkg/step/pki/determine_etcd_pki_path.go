package pki

import (
	"fmt"
	// "os" // Not used directly
	// "path/filepath" // No longer needed for path joining here as full path is received
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // Added
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
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
func (e *DetermineEtcdPKIPathStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost() // PKI path checks are host-specific
	goCtx := ctx.GoContext()

	if currentHost == nil {
		// This step might run on a control-plane node or a node where etcd is relevant.
		// If GetHost() returns nil, it implies this step might not be host-specific in some contexts.
		// However, file operations usually are. Assuming for now it needs a host.
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for %s Check", "DetermineEtcdPKIPathStep")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for %s Check", "DetermineEtcdPKIPathStep")
	}
	spec, ok := rawSpec.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for %s Check: %T", "DetermineEtcdPKIPathStep", rawSpec)
	}
	spec.PopulateDefaults()
	logger = logger.With("step", spec.GetName())

	// PKIPathToEnsureSharedDataKey is read from ModuleCache
	pkiPathVal, pathOk := ctx.ModuleCache().Get(spec.PKIPathToEnsureSharedDataKey)
	if !pathOk {
		logger.Debug("Etcd PKI path not found in Module Cache. Path determination/setup likely pending.", "key", spec.PKIPathToEnsureSharedDataKey)
		return false, nil
	}
	pkiPath, ok := pkiPathVal.(string)
	if !ok || pkiPath == "" {
		logger.Warn("Invalid or empty Etcd PKI path in Module Cache.", "key", spec.PKIPathToEnsureSharedDataKey)
		return false, nil
	}

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	exists, err := conn.Exists(goCtx, pkiPath) // Use connector
	if err != nil {
		logger.Error("Failed to check existence of etcd PKI path", "path", pkiPath, "error", err)
		return false, fmt.Errorf("failed to check existence of etcd PKI path %s: %w", pkiPath, err)
	}
	if !exists {
		logger.Debug("Etcd PKI path (from Module Cache) does not exist.", "path", pkiPath)
		return false, nil
	}
	logger.Debug("Etcd PKI path exists.", "path", pkiPath)

	// Check if it's in TaskCache already
	if val, taskCacheExists := ctx.TaskCache().Get(spec.OutputPKIPathSharedDataKey); taskCacheExists {
		if storedPath, okStr := val.(string); okStr && storedPath == pkiPath {
			logger.Info("Etcd PKI path already available in Task Cache and matches.", "path", pkiPath)
			return true, nil
		}
		logger.Info("Etcd PKI path in Task Cache does not match expected path or is invalid type.", "cachedValue", val, "expectedPath", pkiPath)
	}
	logger.Debug("Etcd PKI path not yet in Task Cache with matching value.")
	return false, nil
}

// Execute ensures the pre-determined etcd PKI path exists and stores it in Task Cache.
func (e *DetermineEtcdPKIPathStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for %s Execute", "DetermineEtcdPKIPathStep")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for %s Execute", "DetermineEtcdPKIPathStep")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for %s Execute: %T", "DetermineEtcdPKIPathStep", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults()
	logger = logger.With("step", spec.GetName())

	pkiPathVal, pathOk := ctx.ModuleCache().Get(spec.PKIPathToEnsureSharedDataKey) // Use ModuleCache
	if !pathOk {
		logger.Error("Etcd PKI path not found in Module Cache. Ensure SetupEtcdPkiDataContextStep ran successfully.", "key", spec.PKIPathToEnsureSharedDataKey)
		res.Error = fmt.Errorf("etcd PKI path not found in Module Cache key '%s'. Ensure SetupEtcdPkiDataContextStep ran successfully.", spec.PKIPathToEnsureSharedDataKey)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	pkiPath, typeOk := pkiPathVal.(string)
	if !typeOk || pkiPath == "" {
		logger.Error("Invalid or empty etcd PKI path in Module Cache.", "key", spec.PKIPathToEnsureSharedDataKey)
		res.Error = fmt.Errorf("invalid or empty etcd PKI path in Module Cache key '%s'", spec.PKIPathToEnsureSharedDataKey)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	logger.Info("Ensuring etcd PKI directory (from Module Cache) exists.", "path", pkiPath)
	// Mode "0700" is appropriate for PKI directories. Sudo may be needed.
	if err := conn.Mkdir(goCtx, pkiPath, "0700"); err != nil { // Sudo handled by connector if needed
		logger.Error("Failed to create etcd PKI directory.", "path", pkiPath, "error", err)
		res.Error = fmt.Errorf("failed to create etcd PKI directory %s on host %s: %w", pkiPath, currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Etcd PKI directory ensured.", "path", pkiPath, "host", currentHost.GetName())

	ctx.TaskCache().Set(spec.OutputPKIPathSharedDataKey, pkiPath) // Use TaskCache
	logger.Info("Stored etcd PKI path in Task Cache.", "key", spec.OutputPKIPathSharedDataKey, "path", pkiPath)

	res.EndTime = time.Now() // Update end time

	done, checkErr := e.Check(ctx)
	if checkErr != nil {
		logger.Error("Post-execution check failed.", "error", checkErr)
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed; return res
	}
	if !done {
		logger.Error("Post-execution check indicates Etcd PKI Path was not correctly ensured or cached.")
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
