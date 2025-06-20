package pki

import (
	"fmt"
	"path/filepath"
	// "time" // No longer used

	"github.com/mensylisir/kubexm/pkg/connector" // Keep for host parameter in interface methods, though not directly used by logic
	"github.com/mensylisir/kubexm/pkg/runtime"   // For runtime.StepContext
	// "github.com/mensylisir/kubexm/pkg/spec" // No longer needed
	"github.com/mensylisir/kubexm/pkg/step"
)

// Define default keys if not imported from a central PKI consts file
const (
	DefaultKubeConfKey = "pkiKubeConfig"
	DefaultHostsKey    = "pkiHosts"
	// DefaultEtcdPKIPathKey is defined in determine_etcd_pki_path.go.
	// We'll use its string value directly in populateDefaults if not importing that file.
	// For this example, using the string literal "etcdPKIPath" as a fallback.
)

// SetupEtcdPkiDataContextStep prepares and caches PKI-related data from the main
// cluster configuration into ModuleCache for use by other PKI steps.
type SetupEtcdPkiDataContextStep struct {
	KubeConfToCache     *KubexmsKubeConf // Input: Prepared KubeConfig (stub) data
	HostsToCache        []HostSpecForPKI // Input: Prepared Host list for PKI
	EtcdSpecificSubPath string           // Input: e.g., "etcd"

	KubeConfOutputKey    string // Output cache key
	HostsOutputKey       string // Output cache key
	EtcdPkiPathOutputKey string // Output cache key for the derived etcd PKI path

	StepName string
}

// NewSetupEtcdPkiDataContextStep creates a new SetupEtcdPkiDataContextStep.
// The KubeConfToCache and HostsToCache are provided directly, prepared by the module.
func NewSetupEtcdPkiDataContextStep(
	kubeConf *KubexmsKubeConf,
	hosts []HostSpecForPKI,
	etcdSubPath string,
	kubeConfKey, hostsKey, etcdPkiPathKey string, // Allow specifying cache keys
	stepName string,
) step.Step {
	s := &SetupEtcdPkiDataContextStep{
		KubeConfToCache:      kubeConf,
		HostsToCache:         hosts,
		EtcdSpecificSubPath:  etcdSubPath,
		KubeConfOutputKey:    kubeConfKey,
		HostsOutputKey:       hostsKey,
		EtcdPkiPathOutputKey: etcdPkiPathKey,
		StepName:             stepName,
	}
	s.populateDefaults()
	return s
}

func (s *SetupEtcdPkiDataContextStep) populateDefaults() {
	if s.KubeConfOutputKey == "" {
		s.KubeConfOutputKey = DefaultKubeConfKey
	}
	if s.HostsOutputKey == "" {
		s.HostsOutputKey = DefaultHostsKey
	}
	if s.EtcdPkiPathOutputKey == "" {
		// This refers to the constant from determine_etcd_pki_path.go
		// If that file/constant isn't directly accessible via import, use the string value.
		s.EtcdPkiPathOutputKey = "etcdPKIPath" // Fallback to string literal if DefaultEtcdPKIPathKey const is not imported
	}
	if s.EtcdSpecificSubPath == "" {
		s.EtcdSpecificSubPath = "etcd" // Default sub-directory for etcd PKI under cluster PKI root
	}
	if s.StepName == "" {
		s.StepName = "Setup Etcd PKI Data Context In Cache"
	}
}

func (s *SetupEtcdPkiDataContextStep) Name() string {
	return s.StepName
}

func (s *SetupEtcdPkiDataContextStep) Description() string {
	return fmt.Sprintf("Caches etcd PKI configuration: KubeConf to key '%s', Hosts to key '%s', EtcdPKIPath to key '%s'.",
		s.KubeConfOutputKey, s.HostsOutputKey, s.EtcdPkiPathOutputKey)
}

func (s *SetupEtcdPkiDataContextStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name())
	if host != nil { // Host is not used by logic but good to include in log if available
		logger = logger.With("host", host.GetName())
	}
	logger = logger.With("phase", "Precheck")

	// This step's purpose is to populate the cache.
	// It should generally always run to ensure the cache reflects the desired state
	// passed to its constructor. A sophisticated check for deep equality of cached complex objects
	// is usually overkill for Precheck.
	logger.Debug("Precheck always returns false to ensure cache is populated/updated with potentially new data.")
	return false, nil
}

func (s *SetupEtcdPkiDataContextStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name())
	if host != nil { // Host is not used by logic but good to include in log if available
		logger = logger.With("host", host.GetName())
	}
	logger = logger.With("phase", "Run")
	// s.populateDefaults(); // Called in constructor

	if s.KubeConfToCache == nil {
		return fmt.Errorf("KubeConfToCache is nil for step %s; it must be provided during step creation", s.Name())
	}
	if s.KubeConfToCache.PKIDirectory == "" {
		return fmt.Errorf("KubeConfToCache.PKIDirectory is empty for step %s; this is needed as the base for Etcd PKI path", s.Name())
	}

	etcdSpecificPkiPath := filepath.Join(s.KubeConfToCache.PKIDirectory, s.EtcdSpecificSubPath)

	ctx.ModuleCache().Set(s.KubeConfOutputKey, s.KubeConfToCache)
	logger.Info("Stored KubeConf (PKI stub) in module cache.", "key", s.KubeConfOutputKey)

	ctx.ModuleCache().Set(s.HostsOutputKey, s.HostsToCache)
	logger.Info("Stored HostSpecForPKI list in module cache.", "key", s.HostsOutputKey)

	ctx.ModuleCache().Set(s.EtcdPkiPathOutputKey, etcdSpecificPkiPath)
	logger.Info("Derived and stored etcd-specific PKI path in module cache.", "key", s.EtcdPkiPathOutputKey, "path", etcdSpecificPkiPath)

	logger.Info("Etcd PKI data context successfully populated into module cache.")
	return nil
}

func (s *SetupEtcdPkiDataContextStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name())
	if host != nil { // Host is not used by logic but good to include in log if available
		logger = logger.With("host", host.GetName())
	}
	logger = logger.With("phase", "Rollback")
	// s.populateDefaults(); // Called in constructor

	logger.Info("Attempting to remove PKI data from ModuleCache.",
		"kubeConfKey", s.KubeConfOutputKey,
		"hostsKey", s.HostsOutputKey,
		"etcdPkiPathKey", s.EtcdPkiPathOutputKey)

	ctx.ModuleCache().Delete(s.KubeConfOutputKey)
	ctx.ModuleCache().Delete(s.HostsOutputKey)
	ctx.ModuleCache().Delete(s.EtcdPkiPathOutputKey)
	logger.Info("Rollback for SetupEtcdPkiDataContextStep: Cleared ModuleCache keys.")
	return nil
}

// Ensure SetupEtcdPkiDataContextStep implements the step.Step interface.
var _ step.Step = (*SetupEtcdPkiDataContextStep)(nil)
