package pki

import (
	"fmt"
	// "path/filepath" // Not strictly needed if path is taken as is
	// "time" // No longer needed for step.Result

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// DefaultEtcdPKIPathKey is used as default for both input and output key for the etcd PKI path.
const DefaultEtcdPKIPathKey = "etcdPKIPath"

// DetermineEtcdPKIPathStep ensures the etcd PKI directory (read from ModuleCache)
// exists on the target host and stores its path in TaskCache.
type DetermineEtcdPKIPathStep struct {
	meta                         spec.StepMeta
	PKIPathToEnsureSharedDataKey string // Key to read etcd PKI path from ModuleCache
	OutputPKIPathSharedDataKey   string // Key to store etcd PKI path into TaskCache
	Sudo                         bool   // Whether sudo is needed for directory creation
}

// NewDetermineEtcdPKIPathStep creates a new DetermineEtcdPKIPathStep.
func NewDetermineEtcdPKIPathStep(instanceName, pkiPathInputKey, pkiPathOutputKey string, sudo bool) step.Step {
	s := &DetermineEtcdPKIPathStep{
		PKIPathToEnsureSharedDataKey: pkiPathInputKey,
		OutputPKIPathSharedDataKey:   pkiPathOutputKey,
		Sudo:                         sudo,
	}
	s.populateDefaults(instanceName) // Pass instanceName to populateDefaults
	return s
}

func (s *DetermineEtcdPKIPathStep) populateDefaults(instanceName string) {
	if s.PKIPathToEnsureSharedDataKey == "" {
		s.PKIPathToEnsureSharedDataKey = DefaultEtcdPKIPathKey
	}
	if s.OutputPKIPathSharedDataKey == "" {
		s.OutputPKIPathSharedDataKey = DefaultEtcdPKIPathKey
	}
	name := instanceName
	if name == "" {
		name = "EnsureEtcdPKIPathExists"
	}
	s.meta.Name = name
	s.meta.Description = fmt.Sprintf("Ensures etcd PKI path (from ModuleCache key '%s') exists and stores it to TaskCache key '%s'.",
		s.PKIPathToEnsureSharedDataKey, s.OutputPKIPathSharedDataKey)
}

// Meta returns the step's metadata.
func (s *DetermineEtcdPKIPathStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DetermineEtcdPKIPathStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

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

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, pkiPath)
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
	return false, nil
}

func (s *DetermineEtcdPKIPathStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	pkiPathVal, pathOk := ctx.ModuleCache().Get(s.PKIPathToEnsureSharedDataKey)
	if !pathOk {
		return fmt.Errorf("etcd PKI path not found in Module Cache using key '%s' for step %s on host %s. Ensure a prior step (like SetupEtcdPkiDataContextStep) sets this value in ModuleCache", s.PKIPathToEnsureSharedDataKey, s.meta.Name, host.GetName())
	}
	pkiPath, typeOk := pkiPathVal.(string)
	if !typeOk || pkiPath == "" {
		return fmt.Errorf("invalid or empty etcd PKI path in Module Cache (key '%s', value '%v') for step %s on host %s", s.PKIPathToEnsureSharedDataKey, pkiPathVal, s.meta.Name, host.GetName())
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	logger.Info("Ensuring etcd PKI directory (from Module Cache) exists.", "path", pkiPath)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, pkiPath, "0700", s.Sudo); err != nil { // Using "0700" for PKI dir
		return fmt.Errorf("failed to create etcd PKI directory %s on host %s for step %s: %w", pkiPath, host.GetName(), s.meta.Name, err)
	}
	logger.Info("Etcd PKI directory ensured.", "path", pkiPath)

	ctx.TaskCache().Set(s.OutputPKIPathSharedDataKey, pkiPath)
	logger.Info("Stored etcd PKI path in Task Cache.", "key", s.OutputPKIPathSharedDataKey, "path", pkiPath)
	return nil
}

func (s *DetermineEtcdPKIPathStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	logger.Info("Attempting to remove TaskCache key for etcd PKI path.", "key", s.OutputPKIPathSharedDataKey)
	ctx.TaskCache().Delete(s.OutputPKIPathSharedDataKey)
	logger.Info("Rollback for DetermineEtcdPKIPathStep: Cleared TaskCache key.", "key", s.OutputPKIPathSharedDataKey)
	return nil
}

// Ensure DetermineEtcdPKIPathStep implements the step.Step interface.
var _ step.Step = (*DetermineEtcdPKIPathStep)(nil)
