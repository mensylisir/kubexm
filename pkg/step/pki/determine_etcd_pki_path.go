package pki

import (
	"fmt"
	// "path/filepath" // Not strictly needed if path is taken as is
	// "time" // No longer needed for step.Result

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	// spec is no longer needed
)

// DefaultEtcdPKIPathKey is used as default for both input and output key for the etcd PKI path.
const DefaultEtcdPKIPathKey = "etcdPKIPath"

// DetermineEtcdPKIPathStep ensures the etcd PKI directory (read from ModuleCache)
// exists on the target host and stores its path in TaskCache.
type DetermineEtcdPKIPathStep struct {
	PKIPathToEnsureSharedDataKey string // Key to read etcd PKI path from ModuleCache
	OutputPKIPathSharedDataKey string // Key to store etcd PKI path into TaskCache
	StepName                     string
}

// NewDetermineEtcdPKIPathStep creates a new DetermineEtcdPKIPathStep.
func NewDetermineEtcdPKIPathStep(pkiPathInputKey, pkiPathOutputKey, stepName string) step.Step {
	s := &DetermineEtcdPKIPathStep{
		PKIPathToEnsureSharedDataKey: pkiPathInputKey,
		OutputPKIPathSharedDataKey: pkiPathOutputKey,
		StepName:                     stepName,
	}
	s.populateDefaults()
	return s
}

func (s *DetermineEtcdPKIPathStep) populateDefaults() {
	if s.PKIPathToEnsureSharedDataKey == "" {
		s.PKIPathToEnsureSharedDataKey = DefaultEtcdPKIPathKey
	}
	if s.OutputPKIPathSharedDataKey == "" {
		s.OutputPKIPathSharedDataKey = DefaultEtcdPKIPathKey
	}
	if s.StepName == "" {
		s.StepName = "Ensure Etcd PKI Path Exists"
	}
}

func (s *DetermineEtcdPKIPathStep) Name() string {
	return s.StepName
}

func (s *DetermineEtcdPKIPathStep) Description() string {
	return fmt.Sprintf("Ensures etcd PKI path (from ModuleCache key '%s') exists and stores it to TaskCache key '%s'.",
		s.PKIPathToEnsureSharedDataKey, s.OutputPKIPathSharedDataKey)
}

func (s *DetermineEtcdPKIPathStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")
	// s.populateDefaults(); // Called in constructor

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
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
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

func (s *DetermineEtcdPKIPathStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")
	// s.populateDefaults(); // Called in constructor

	pkiPathVal, pathOk := ctx.ModuleCache().Get(s.PKIPathToEnsureSharedDataKey)
	if !pathOk {
		return fmt.Errorf("etcd PKI path not found in Module Cache using key '%s' for step %s on host %s. Ensure a prior step (like SetupEtcdPkiDataContextStep) sets this value in ModuleCache", s.PKIPathToEnsureSharedDataKey, s.Name(), host.GetName())
	}
	pkiPath, typeOk := pkiPathVal.(string)
	if !typeOk || pkiPath == "" {
		return fmt.Errorf("invalid or empty etcd PKI path in Module Cache (key '%s', value '%v') for step %s on host %s", s.PKIPathToEnsureSharedDataKey, pkiPathVal, s.Name(), host.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	logger.Info("Ensuring etcd PKI directory (from Module Cache) exists.", "path", pkiPath)
	// Mode "0700" is appropriate for PKI directories. Sudo may be needed.
	// Assuming conn.Mkdir handles sudo appropriately if the connector is sudo-enabled.
	if err := conn.Mkdir(ctx.GoContext(), pkiPath, "0700"); err != nil {
		return fmt.Errorf("failed to create etcd PKI directory %s on host %s for step %s: %w", pkiPath, host.GetName(), s.Name(), err)
	}
	logger.Info("Etcd PKI directory ensured.", "path", pkiPath)

	ctx.TaskCache().Set(s.OutputPKIPathSharedDataKey, pkiPath)
	logger.Info("Stored etcd PKI path in Task Cache.", "key", s.OutputPKIPathSharedDataKey, "path", pkiPath)
	return nil
}

func (s *DetermineEtcdPKIPathStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")
	// s.populateDefaults(); // Called in constructor

	logger.Info("Attempting to remove TaskCache key for etcd PKI path.", "key", s.OutputPKIPathSharedDataKey)
	ctx.TaskCache().Delete(s.OutputPKIPathSharedDataKey)
	logger.Info("Rollback for DetermineEtcdPKIPathStep: Cleared TaskCache key.", "key", s.OutputPKIPathSharedDataKey)
	return nil
}

// Ensure DetermineEtcdPKIPathStep implements the step.Step interface.
var _ step.Step = (*DetermineEtcdPKIPathStep)(nil)
