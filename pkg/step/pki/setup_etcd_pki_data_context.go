package pki

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// Define default keys if not imported from a central PKI consts file
const (
	DefaultKubeConfKey = "pkiKubeConfig" // Consider moving to a common constants package for PKI
	DefaultHostsKey    = "pkiHosts"    // Consider moving
	// DefaultEtcdPKIPathKey is "etcdPKIPath" (from determine_etcd_pki_path.go's DefaultEtcdPKIPathKey)
)

// SetupEtcdPkiDataContextStep prepares and caches PKI-related data from the main
// cluster configuration into ModuleCache for use by other PKI steps.
type SetupEtcdPkiDataContextStep struct {
	meta                 spec.StepMeta
	KubeConfToCache      *KubexmsKubeConf // Input: Prepared KubeConfig (stub) data
	HostsToCache         []HostSpecForPKI // Input: Prepared Host list for PKI
	EtcdSpecificSubPath  string           // Input: e.g., "etcd"
	KubeConfOutputKey    string           // Output cache key
	HostsOutputKey       string           // Output cache key
	EtcdPkiPathOutputKey string           // Output cache key for the derived etcd PKI path
}

// NewSetupEtcdPkiDataContextStep creates a new SetupEtcdPkiDataContextStep.
// The KubeConfToCache and HostsToCache are provided directly, prepared by the module.
func NewSetupEtcdPkiDataContextStep(
	instanceName string,
	kubeConf *KubexmsKubeConf,
	hosts []HostSpecForPKI,
	etcdSubPath string,
	kubeConfKey, hostsKey, etcdPkiPathKey string, // Allow specifying cache keys
) step.Step {
	s := &SetupEtcdPkiDataContextStep{
		KubeConfToCache:      kubeConf,
		HostsToCache:         hosts,
		EtcdSpecificSubPath:  etcdSubPath,
		KubeConfOutputKey:    kubeConfKey,
		HostsOutputKey:       hostsKey,
		EtcdPkiPathOutputKey: etcdPkiPathKey,
	}
	s.populateDefaults(instanceName)
	return s
}

func (s *SetupEtcdPkiDataContextStep) populateDefaults(instanceName string) {
	if s.KubeConfOutputKey == "" {
		s.KubeConfOutputKey = DefaultKubeConfKey
	}
	if s.HostsOutputKey == "" {
		s.HostsOutputKey = DefaultHostsKey
	}
	if s.EtcdPkiPathOutputKey == "" {
		s.EtcdPkiPathOutputKey = DefaultEtcdPKIPathKey // Using the const from determine_etcd_pki_path.go
	}
	if s.EtcdSpecificSubPath == "" {
		s.EtcdSpecificSubPath = "etcd" // Default sub-directory for etcd PKI under cluster PKI root
	}

	name := instanceName
	if name == "" {
		name = "SetupEtcdPkiDataContextInCache"
	}
	s.meta.Name = name
	s.meta.Description = fmt.Sprintf("Caches etcd PKI configuration: KubeConf to key '%s', Hosts to key '%s', EtcdPKIPath to key '%s'.",
		s.KubeConfOutputKey, s.HostsOutputKey, s.EtcdPkiPathOutputKey)
}

// Meta returns the step's metadata.
func (s *SetupEtcdPkiDataContextStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *SetupEtcdPkiDataContextStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name)
	if host != nil {
		logger = logger.With("host", host.GetName())
	}
	logger = logger.With("phase", "Precheck")

	logger.Debug("Precheck always returns false to ensure cache is populated/updated with potentially new data.")
	return false, nil
}

func (s *SetupEtcdPkiDataContextStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name)
	if host != nil {
		logger = logger.With("host", host.GetName())
	}
	logger = logger.With("phase", "Run")

	if s.KubeConfToCache == nil {
		return fmt.Errorf("KubeConfToCache is nil for step %s; it must be provided during step creation", s.meta.Name)
	}
	if s.KubeConfToCache.PKIDirectory == "" { // PKIDirectory is part of KubexmsKubeConf stub
		return fmt.Errorf("KubeConfToCache.PKIDirectory is empty for step %s; this is needed as the base for Etcd PKI path", s.meta.Name)
	}

	etcdSpecificPkiPath := filepath.Join(s.KubeConfToCache.PKIDirectory, s.EtcdSpecificSubPath)

	ctx.GetModuleCache().Set(s.KubeConfOutputKey, s.KubeConfToCache)
	logger.Info("Stored KubeConf (PKI stub) in module cache.", "key", s.KubeConfOutputKey)

	ctx.GetModuleCache().Set(s.HostsOutputKey, s.HostsToCache)
	logger.Info("Stored HostSpecForPKI list in module cache.", "key", s.HostsOutputKey)

	ctx.GetModuleCache().Set(s.EtcdPkiPathOutputKey, etcdSpecificPkiPath)
	logger.Info("Derived and stored etcd-specific PKI path in module cache.", "key", s.EtcdPkiPathOutputKey, "path", etcdSpecificPkiPath)

	logger.Info("Etcd PKI data context successfully populated into module cache.")
	return nil
}

func (s *SetupEtcdPkiDataContextStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name)
	if host != nil {
		logger = logger.With("host", host.GetName())
	}
	logger = logger.With("phase", "Rollback")

	logger.Info("Attempting to remove PKI data from ModuleCache.",
		"kubeConfKey", s.KubeConfOutputKey,
		"hostsKey", s.HostsOutputKey,
		"etcdPkiPathKey", s.EtcdPkiPathOutputKey)

	ctx.GetModuleCache().Delete(s.KubeConfOutputKey)
	ctx.GetModuleCache().Delete(s.HostsOutputKey)
	ctx.GetModuleCache().Delete(s.EtcdPkiPathOutputKey)
	logger.Info("Rollback for SetupEtcdPkiDataContextStep: Cleared ModuleCache keys.")
	return nil
}

// Ensure SetupEtcdPkiDataContextStep implements the step.Step interface.
var _ step.Step = (*SetupEtcdPkiDataContextStep)(nil)
