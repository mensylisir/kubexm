package pki

import (
	"fmt"
	"path/filepath"
	// "time" // No longer used

	"github.com/mensylisir/kubexm/pkg/connector" // Keep for host parameter in interface methods
	"github.com/mensylisir/kubexm/pkg/runtime"   // For runtime.StepContext
	"github.com/mensylisir/kubexm/pkg/spec"    // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// Define default keys if not imported from a central PKI consts file
// Note: KubexmsKubeConf and HostSpecForPKI types would need to be defined in this package or imported.
// For this refactoring, assume they are defined in this 'pki' package.
const (
	DefaultKubeConfKey = "pkiKubeConfig"
	DefaultHostsKey    = "pkiHosts"
	// DefaultEtcdPKIPathKey is defined in determine_etcd_pki_path.go.
	// We'll use its string value directly in populateDefaults if not importing that file.
	// For this example, using the string literal "etcdPKIPath" as a fallback.
)

// SetupEtcdPkiDataContextStepSpec prepares and caches PKI-related data from the main
// cluster configuration into ModuleCache for use by other PKI steps.
type SetupEtcdPkiDataContextStepSpec struct {
	spec.StepMeta        `json:",inline"`
	KubeConfToCache     *KubexmsKubeConf `json:"-"` // Input: Prepared KubeConfig (stub) data - avoid direct serialization
	HostsToCache        []HostSpecForPKI `json:"-"` // Input: Prepared Host list for PKI - avoid direct serialization
	EtcdSpecificSubPath string           `json:"etcdSpecificSubPath,omitempty"` // Input: e.g., "etcd"

	KubeConfOutputKey    string `json:"kubeConfOutputKey,omitempty"`    // Output cache key
	HostsOutputKey       string `json:"hostsOutputKey,omitempty"`       // Output cache key
	EtcdPkiPathOutputKey string `json:"etcdPkiPathOutputKey,omitempty"` // Output cache key for the derived etcd PKI path
}

// NewSetupEtcdPkiDataContextStepSpec creates a new SetupEtcdPkiDataContextStepSpec.
// The KubeConfToCache and HostsToCache are provided directly, prepared by the module.
func NewSetupEtcdPkiDataContextStep(
	kubeConf *KubexmsKubeConf,
	hosts []HostSpecForPKI,
	etcdSubPath string,
	kubeConfKey, hostsKey, etcdPkiPathKey string, // Allow specifying cache keys
	name, description string, // Added for StepMeta
) *SetupEtcdPkiDataContextStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Setup Etcd PKI Data Context In Cache"
	}

	kcKey := kubeConfKey; if kcKey == "" { kcKey = DefaultKubeConfKey }
	hKey := hostsKey; if hKey == "" { hKey = DefaultHostsKey }
	epkKey := etcdPkiPathKey; if epkKey == "" { epkKey = DefaultEtcdPKIPathKey } // Using the constant from this package
	subPath := etcdSubPath; if subPath == "" { subPath = "etcd" }


	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Caches etcd PKI configuration: KubeConf to key '%s', Hosts to key '%s', EtcdPKIPath to key '%s'.",
			kcKey, hKey, epkKey)
	}

	s := &SetupEtcdPkiDataContextStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		KubeConfToCache:      kubeConf,
		HostsToCache:         hosts,
		EtcdSpecificSubPath:  subPath,
		KubeConfOutputKey:    kcKey,
		HostsOutputKey:       hKey,
		EtcdPkiPathOutputKey: epkKey,
	}
	// No separate populateDefaults needed as factory handles all defaults.
	return s
}

// Name returns the step's name (implementing step.Step).
func (s *SetupEtcdPkiDataContextStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *SetupEtcdPkiDataContextStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *SetupEtcdPkiDataContextStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *SetupEtcdPkiDataContextStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *SetupEtcdPkiDataContextStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *SetupEtcdPkiDataContextStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *SetupEtcdPkiDataContextStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName())
	// This step is host-agnostic in its execution, but host might be used for logging context.
	if host != nil { logger = logger.With("host", host.GetName()) }
	logger = logger.With("phase", "Precheck")

	logger.Debug("Precheck always returns false; this step updates cache and should run if included.")
	return false, nil
}

func (s *SetupEtcdPkiDataContextStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName())
	if host != nil { logger = logger.With("host", host.GetName()) }
	logger = logger.With("phase", "Run")

	if s.KubeConfToCache == nil {
		return fmt.Errorf("KubeConfToCache is nil for step %s; it must be provided during step creation", s.GetName())
	}
	if s.KubeConfToCache.PKIDirectory == "" {
		return fmt.Errorf("KubeConfToCache.PKIDirectory is empty for step %s; this is needed as the base for Etcd PKI path", s.GetName())
	}

	etcdSpecificPkiPath := filepath.Join(s.KubeConfToCache.PKIDirectory, s.EtcdSpecificSubPath)

	// This step writes to ModuleCache as it sets up context for other steps within the same module.
	ctx.ModuleCache().Set(s.KubeConfOutputKey, s.KubeConfToCache)
	logger.Info("Stored KubeConf (PKI stub) in module cache.", "key", s.KubeConfOutputKey)

	ctx.ModuleCache().Set(s.HostsOutputKey, s.HostsToCache)
	logger.Info("Stored HostSpecForPKI list in module cache.", "key", s.HostsOutputKey)

	ctx.ModuleCache().Set(s.EtcdPkiPathOutputKey, etcdSpecificPkiPath)
	logger.Info("Derived and stored etcd-specific PKI path in module cache.", "key", s.EtcdPkiPathOutputKey, "path", etcdSpecificPkiPath)

	logger.Info("Etcd PKI data context successfully populated into module cache.")
	return nil
}

func (s *SetupEtcdPkiDataContextStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName())
	if host != nil { logger = logger.With("host", host.GetName()) }
	logger = logger.With("phase", "Rollback")

	logger.Info("Attempting to remove PKI data from ModuleCache.",
		"kubeConfKey", s.KubeConfOutputKey,
		"hostsKey", s.HostsOutputKey,
		"etcdPkiPathKey", s.EtcdPkiPathOutputKey)

	ctx.ModuleCache().Delete(s.KubeConfOutputKey)
	ctx.ModuleCache().Delete(s.HostsOutputKey)
	ctx.ModuleCache().Delete(s.EtcdPkiPathOutputKey)
	logger.Info("Rollback for SetupEtcdPkiDataContextStepSpec: Cleared ModuleCache keys.")
	return nil
}

// Ensure SetupEtcdPkiDataContextStepSpec implements the step.Step interface.
var _ step.Step = (*SetupEtcdPkiDataContextStepSpec)(nil)
