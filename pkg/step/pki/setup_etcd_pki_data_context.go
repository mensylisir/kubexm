package pki

import (
	"fmt"
	"path/filepath"
	"time"

	// connector import not strictly needed if no direct host operations beyond what StepContext provides
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.StepContext
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// SetupEtcdPkiDataContextStepSpec holds the data prepared by the module to be put into the cache.
// This data is derived from the main cluster configuration (cfg *config.Cluster).
type SetupEtcdPkiDataContextStepSpec struct {
	// Data to be cached, directly populated by the module
	KubeConfToCache    *KubexmsKubeConf // Contains AppFSBaseDir, ClusterName, PKIDirectory (cluster PKI root)
	HostsToCache       []HostSpecForPKI // List of hosts with roles for node certificate generation
	EtcdSpecificSubPath string           // e.g., "etcd" - to be appended to KubeConfToCache.PKIDirectory

	// SharedData Keys to use when setting these values in the module cache
	KubeConfOutputKey      string // Defaults to pki.DefaultKubeConfKey
	HostsOutputKey         string // Defaults to pki.DefaultHostsKey
	EtcdPkiPathOutputKey string // Defaults to pki.DefaultEtcdPKIPathKey
}

// GetName returns the step name.
func (s *SetupEtcdPkiDataContextStepSpec) GetName() string {
	return "Setup Etcd PKI Data Context In Cache"
}

// PopulateDefaults for shared data keys and specific subpath.
func (s *SetupEtcdPkiDataContextStepSpec) PopulateDefaults() {
	if s.KubeConfOutputKey == "" {
		s.KubeConfOutputKey = DefaultKubeConfKey
	}
	if s.HostsOutputKey == "" {
		s.HostsOutputKey = DefaultHostsKey
	}
	if s.EtcdPkiPathOutputKey == "" {
		s.EtcdPkiPathOutputKey = DefaultEtcdPKIPathKey
	}
	if s.EtcdSpecificSubPath == "" {
		s.EtcdSpecificSubPath = "etcd"
	}
}

// SetupEtcdPkiDataContextStepExecutor implements the logic.
type SetupEtcdPkiDataContextStepExecutor struct{}

// Check always returns false to ensure this setup step runs if included in a task.
// It could alternatively check if all its target keys are already populated in the module cache.
func (e *SetupEtcdPkiDataContextStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	// Forcing execution to ensure cache reflects current desired state from config.
	// If this step were more expensive, a more detailed check might be warranted.
	// No host-specific checks needed here.
	return false, nil
}

// Execute populates the module cache with necessary PKI data.
func (e *SetupEtcdPkiDataContextStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost() // May be nil if this step is not host-specific

	// This step operates locally or sets global/module-level context,
	// so currentHost might be nil. NewResult handles nil host.
	res := step.NewResult(ctx, currentHost, startTime, nil)

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for %s", "SetupEtcdPkiDataContextStep")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*SetupEtcdPkiDataContextStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type: %T for %s", rawSpec, "SetupEtcdPkiDataContextStep")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec.PopulateDefaults()
	logger = logger.With("step", spec.GetName())


	if spec.KubeConfToCache == nil {
		logger.Error("KubeConfToCache is nil in spec")
		res.Error = fmt.Errorf("KubeConfToCache is nil in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	if spec.KubeConfToCache.PKIDirectory == "" {
		logger.Error("KubeConfToCache.PKIDirectory is empty in spec; this is needed as the base for Etcd PKI path")
		res.Error = fmt.Errorf("KubeConfToCache.PKIDirectory is empty in spec for %s; this is needed as the base for Etcd PKI path", spec.GetName())
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	etcdSpecificPkiPath := filepath.Join(spec.KubeConfToCache.PKIDirectory, spec.EtcdSpecificSubPath)

	ctx.ModuleCache().Set(spec.KubeConfOutputKey, spec.KubeConfToCache) // Use ModuleCache
	logger.Info("Stored KubexmsKubeConf in module cache.", "key", spec.KubeConfOutputKey)

	ctx.ModuleCache().Set(spec.HostsOutputKey, spec.HostsToCache) // Use ModuleCache
	logger.Info("Stored []HostSpecForPKI in module cache.", "key", spec.HostsOutputKey)

	ctx.ModuleCache().Set(spec.EtcdPkiPathOutputKey, etcdSpecificPkiPath) // Use ModuleCache
	logger.Info("Derived and stored etcd-specific PKI path in module cache.", "key", spec.EtcdPkiPathOutputKey, "path", etcdSpecificPkiPath)

	res.EndTime = time.Now()
	res.Message = "Etcd PKI data context successfully populated into module cache."
	res.Status = step.StatusSucceeded
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&SetupEtcdPkiDataContextStepSpec{}), &SetupEtcdPkiDataContextStepExecutor{})
}
