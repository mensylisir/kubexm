package pki

import (
	"fmt"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// SetupEtcdPkiDataContextStepSpec holds the data prepared by the module to be put into the cache.
type SetupEtcdPkiDataContextStepSpec struct {
	// Data to be cached
	KubeConfForCache         *KubexmsKubeConf    // Using the stub defined in etcd_cert_definitions.go
	HostsForPKIForCache      []HostSpecForPKI    // Using HostSpecForPKI from generate_etcd_node_certs.go
	HostsForAltNamesForCache []HostSpecForAltNames // Using HostSpecForAltNames from generate_etcd_alt_names.go
	EtcdPkiPathForCache      string

	// SharedData Keys to use when setting these values in the cache
	KubeConfOutputKey         string
	HostsForPKIOutputKey      string
	HostsForAltNamesOutputKey string
	EtcdPkiPathOutputKey      string
}

// GetName returns the step name.
func (s *SetupEtcdPkiDataContextStepSpec) GetName() string {
	return "Setup Etcd PKI Data Context"
}

// PopulateDefaults for shared data keys
func (s *SetupEtcdPkiDataContextStepSpec) PopulateDefaults() {
	if s.KubeConfOutputKey == "" {
		s.KubeConfOutputKey = DefaultKubeConfKey // from generate_etcd_ca.go
	}
	if s.HostsForPKIOutputKey == "" {
		s.HostsForPKIOutputKey = DefaultHostsKey // from generate_etcd_node_certs.go
	}
	if s.HostsForAltNamesOutputKey == "" {
		// Assuming AltNames step uses DefaultEtcdAltNamesKey to retrieve *cert.AltNames,
		// but this context step is about providing the raw host data for GenerateEtcdAltNamesStepSpec.
		// GenerateEtcdAltNamesStepSpec.Hosts is a direct field, not from cache.
		// So, this key might not be needed if HostsForAltNamesForCache is directly passed to GenerateEtcdAltNamesStepSpec in module.
		// Let's remove HostsForAltNamesOutputKey for now as HostsForAltNamesForCache is used to directly populate the spec.
	}
	if s.EtcdPkiPathOutputKey == "" {
		s.EtcdPkiPathOutputKey = DefaultEtcdPKIPathKey // from determine_etcd_pki_path.go
	}
}

// SetupEtcdPkiDataContextStepExecutor implements the logic.
type SetupEtcdPkiDataContextStepExecutor struct{}

// Check always returns false as this step is about populating cache.
func (e *SetupEtcdPkiDataContextStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	// This step could check if all keys are already populated with expected types,
	// but for simplicity and to ensure data freshness from cfg, let it run.
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

	if spec.KubeConfForCache == nil {
		return step.NewResult(ctx, startTime, fmt.Errorf("KubeConfForCache is nil in spec"))
	}
	// HostsForPKIForCache can be empty if no hosts are defined, this is not an error itself.
	// EtcdPkiPathForCache should not be empty.
	if spec.EtcdPkiPathForCache == "" {
		return step.NewResult(ctx, startTime, fmt.Errorf("EtcdPkiPathForCache is nil in spec"))
	}

	ctx.Module().Set(spec.KubeConfOutputKey, spec.KubeConfForCache)
	logger.Infof("Stored KubexmsKubeConf in module cache under key '%s'", spec.KubeConfOutputKey)

	// Store HostsForPKIForCache (used by GenerateEtcdNodeCertsStep)
	ctx.Module().Set(spec.HostsForPKIOutputKey, spec.HostsForPKIForCache)
	logger.Infof("Stored []HostSpecForPKI in module cache under key '%s'", spec.HostsForPKIOutputKey)

	// HostsForAltNamesForCache is intended to be used directly by the module to populate
	// GenerateEtcdAltNamesStepSpec.Hosts field, not put into cache for that step to read.
	// So, no caching for spec.HostsForAltNamesForCache via this step.

	ctx.Module().Set(spec.EtcdPkiPathOutputKey, spec.EtcdPkiPathForCache)
	logger.Infof("Stored EtcdPkiPath in module cache under key '%s'", spec.EtcdPkiPathOutputKey)

	return step.NewResult(ctx, startTime, nil)
}

func init() {
	step.Register(&SetupEtcdPkiDataContextStepSpec{}, &SetupEtcdPkiDataContextStepExecutor{})
}
