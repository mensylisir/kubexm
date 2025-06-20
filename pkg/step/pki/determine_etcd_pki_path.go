package pki

import (
	"fmt"
	// "path/filepath" // Not strictly needed if path is taken as is
	// "time" // No longer needed for step.Result

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// DefaultEtcdPKIPathKey is used as default for both input and output key for the etcd PKI path.
const DefaultEtcdPKIPathKey = "etcdPKIPath"

// DetermineEtcdPKIPathStepSpec ensures the etcd PKI directory (read from ModuleCache)
// exists on the target host and stores its path in TaskCache.
type DetermineEtcdPKIPathStepSpec struct {
	spec.StepMeta                `json:",inline"`
	PKIPathToEnsureSharedDataKey string `json:"pkiPathToEnsureSharedDataKey,omitempty"` // Key to read etcd PKI path from ModuleCache
	OutputPKIPathSharedDataKey string `json:"outputPKIPathSharedDataKey,omitempty"` // Key to store etcd PKI path into TaskCache
}

// NewDetermineEtcdPKIPathStepSpec creates a new DetermineEtcdPKIPathStepSpec.
func NewDetermineEtcdPKIPathStep(pkiPathInputKey, pkiPathOutputKey, name string) *DetermineEtcdPKIPathStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Ensure Etcd PKI Path Exists"
	}

	inputKey := pkiPathInputKey
	if inputKey == "" { inputKey = DefaultEtcdPKIPathKey}
	outputKey := pkiPathOutputKey
	if outputKey == "" { outputKey = DefaultEtcdPKIPathKey}

	description := fmt.Sprintf("Ensures etcd PKI path (from ModuleCache key '%s') exists and stores it to TaskCache key '%s'.",
		inputKey, outputKey)

	s := &DetermineEtcdPKIPathStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: description,
		},
		PKIPathToEnsureSharedDataKey: inputKey,
		OutputPKIPathSharedDataKey: outputKey,
	}
	// No separate populateDefaults needed as factory handles it.
	return s
}

// Name returns the step's name (implementing step.Step).
func (s *DetermineEtcdPKIPathStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *DetermineEtcdPKIPathStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *DetermineEtcdPKIPathStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *DetermineEtcdPKIPathStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *DetermineEtcdPKIPathStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *DetermineEtcdPKIPathStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }


func (s *DetermineEtcdPKIPathStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	pkiPathVal, pathOk := ctx.ModuleCache().Get(s.PKIPathToEnsureSharedDataKey)
	if !pathOk {
		logger.Debug("Etcd PKI path not found in Module Cache. Path determination/setup likely pending. Run will attempt.", "key", s.PKIPathToEnsureSharedDataKey)
		return false, nil
	}
	pkiPath, ok := pkiPathVal.(string)
	if !ok || pkiPath == "" {
		logger.Warn("Invalid or empty Etcd PKI path in Module Cache. Run will likely fail or attempt to create an unintended path.", "key", s.PKIPathToEnsureSharedDataKey, "value", pkiPathVal)
		return false, nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), pkiPath)
	if err != nil {
		logger.Warn("Failed to check existence of etcd PKI path, assuming it needs to be ensured by Run.", "path", pkiPath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Debug("Etcd PKI path (from Module Cache) does not exist on disk. Run will create it.", "path", pkiPath)
		return false, nil
	}
	logger.Debug("Etcd PKI path exists on disk.", "path", pkiPath)

	if val, taskCacheExists := ctx.TaskCache().Get(s.OutputPKIPathSharedDataKey); taskCacheExists {
		if storedPath, okStr := val.(string); okStr && storedPath == pkiPath {
			logger.Info("Etcd PKI path already available in Task Cache and matches.", "path", pkiPath)
			return true, nil
		}
		logger.Debug("Etcd PKI path in Task Cache does not match or is invalid type.", "cachedValue", val, "expectedPath", pkiPath)
	} else {
		logger.Debug("Etcd PKI path not yet in Task Cache.")
	}
	// If directory exists, but not in TaskCache with matching value, Run is needed to set TaskCache.
	return false, nil
}

func (s *DetermineEtcdPKIPathStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	pkiPathVal, pathOk := ctx.ModuleCache().Get(s.PKIPathToEnsureSharedDataKey)
	if !pathOk {
		return fmt.Errorf("etcd PKI path not found in Module Cache using key '%s' for step %s on host %s. Ensure a prior step (like SetupEtcdPkiDataContextStep) sets this value in ModuleCache", s.PKIPathToEnsureSharedDataKey, s.GetName(), host.GetName())
	}
	pkiPath, typeOk := pkiPathVal.(string)
	if !typeOk || pkiPath == "" {
		return fmt.Errorf("invalid or empty etcd PKI path in Module Cache (key '%s', value '%v') for step %s on host %s", s.PKIPathToEnsureSharedDataKey, pkiPathVal, s.GetName(), host.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.GetName(), err)
	}

	logger.Info("Ensuring etcd PKI directory (from Module Cache) exists.", "path", pkiPath)
	// Mode "0700" is appropriate for PKI directories. Sudo may be needed.
	// Assuming conn.Mkdir handles sudo appropriately if the connector is sudo-enabled.
	// For PKI dirs, sudo is often required. A more explicit Exec("mkdir -p ...", sudo) might be safer.
	if err := conn.Mkdir(ctx.GoContext(), pkiPath, "0700"); err != nil { // Ensure Mkdir handles sudo if needed.
		return fmt.Errorf("failed to create etcd PKI directory %s on host %s for step %s: %w", pkiPath, host.GetName(), s.GetName(), err)
	}
	logger.Info("Etcd PKI directory ensured.", "path", pkiPath)

	ctx.TaskCache().Set(s.OutputPKIPathSharedDataKey, pkiPath)
	logger.Info("Stored etcd PKI path in Task Cache.", "key", s.OutputPKIPathSharedDataKey, "path", pkiPath)
	return nil
}

func (s *DetermineEtcdPKIPathStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")

	logger.Info("Attempting to remove TaskCache key for etcd PKI path.", "key", s.OutputPKIPathSharedDataKey)
	ctx.TaskCache().Delete(s.OutputPKIPathSharedDataKey)
	logger.Info("Rollback for DetermineEtcdPKIPathStepSpec: Cleared TaskCache key.", "key", s.OutputPKIPathSharedDataKey)
	return nil
}

// Ensure DetermineEtcdPKIPathStepSpec implements the step.Step interface.
var _ step.Step = (*DetermineEtcdPKIPathStepSpec)(nil)
