package pki

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// SetupEtcdPkiDataContextStepSpec holds the data prepared by the module to be put into the cache.
// This data is derived from the main cluster configuration (cfg *config.Cluster).
type SetupEtcdPkiDataContextStepSpec struct {
	// Data to be cached, directly populated by the module
	KubeConfToCache    *KubexmsKubeConf // Contains AppFSBaseDir, ClusterName, PKIDirectory (cluster PKI root)
	HostsToCache       []HostSpecForPKI
	EtcdSpecificSubPath string // e.g., "etcd" - to be appended to KubeConfToCache.PKIDirectory

	// SharedData Keys to use when setting these values in the cache (module scoped)
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
		s.KubeConfOutputKey = DefaultKubeConfKey // from generate_etcd_ca.go
	}
	if s.HostsOutputKey == "" {
		s.HostsOutputKey = DefaultHostsKey // from generate_etcd_node_certs.go
	}
	if s.EtcdPkiPathOutputKey == "" {
		s.EtcdPkiPathOutputKey = DefaultEtcdPKIPathKey // from determine_etcd_pki_path.go
	}
	if s.EtcdSpecificSubPath == "" {
		s.EtcdSpecificSubPath = "etcd" // The specific directory name for etcd PKI under cluster PKI root
	}
}

// SetupEtcdPkiDataContextStepExecutor implements the logic.
type SetupEtcdPkiDataContextStepExecutor struct{}

// Check always returns false for now, to ensure data is fresh from config each time.
// Alternatively, it could check if all keys are populated in ctx.Module() with correct types.
func (e *SetupEtcdPkiDataContextStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	// For simplicity, let this step always run to ensure the module cache reflects the current cfg.
	// A more sophisticated check could verify if the data in cache matches KubeConfForCache etc.
	// but that would require passing the original cfg values into the spec for comparison,
	// or assuming KubeConfForCache in spec is the source of truth.
	return false, nil
}

// Execute populates the module cache with necessary PKI data.
func (e *SetupEtcdPkiDataContextStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context"))
	}
	spec, ok := currentFullSpec.(*SetupEtcdPkiDataContextStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type: %T", currentFullSpec))
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil)

	if spec.KubeConfToCache == nil {
		res.Error = fmt.Errorf("KubeConfToCache is nil in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; return res
	}
	// spec.HostsToCache can be nil/empty if no hosts are defined, which is acceptable.

	// Derive the specific etcd PKI path
	// KubeConfToCache.PKIDirectory is the cluster's general PKI root (e.g., .../.kubexm/pki/cluster-name)
	if spec.KubeConfToCache.PKIDirectory == "" {
		res.Error = fmt.Errorf("KubeConfToCache.PKIDirectory is empty in spec for %s", spec.GetName())
		res.Status = step.StatusFailed; return res
	}
	etcdSpecificPkiPath := filepath.Join(spec.KubeConfToCache.PKIDirectory, spec.EtcdSpecificSubPath)

	// Store in Module Cache, as this data is relevant for the whole etcd module's operations.
	ctx.Module().Set(spec.KubeConfOutputKey, spec.KubeConfToCache)
	logger.Infof("Stored KubexmsKubeConf in module cache under key '%s'", spec.KubeConfOutputKey)

	ctx.Module().Set(spec.HostsOutputKey, spec.HostsToCache)
	logger.Infof("Stored []HostSpecForPKI in module cache under key '%s'", spec.HostsOutputKey)

	ctx.Module().Set(spec.EtcdPkiPathOutputKey, etcdSpecificPkiPath)
	logger.Infof("Derived and stored etcd-specific PKI path '%s' in module cache under key '%s'", etcdSpecificPkiPath, spec.EtcdPkiPathOutputKey)

	res.Message = "Etcd PKI data context successfully populated into module cache."
	return res
}

func init() {
	step.Register(&SetupEtcdPkiDataContextStepSpec{}, &SetupEtcdPkiDataContextStepExecutor{})
}
