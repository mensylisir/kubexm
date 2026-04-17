package preflight

import (
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/util/helm"
	"github.com/mensylisir/kubexm/internal/types"
)

// templateLogger matches the actual *logger.Logger.Infof signature.
type templateLogger interface {
	Infof(template string, args ...interface{})
}

var _ step.Step = (*CheckVersionCompatibilityStep)(nil)

// CheckVersionCompatibilityStep validates that selected component versions
// are compatible with the target Kubernetes version before installation begins.
// This catches version mismatch errors early rather than failing during deployment.
type CheckVersionCompatibilityStep struct {
	step.Base
}

type CheckVersionCompatibilityStepBuilder struct {
	step.Builder[CheckVersionCompatibilityStepBuilder, *CheckVersionCompatibilityStep]
}

func NewCheckVersionCompatibilityStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CheckVersionCompatibilityStepBuilder {
	s := &CheckVersionCompatibilityStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "[Preflight] Validate component versions are compatible with Kubernetes version"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(CheckVersionCompatibilityStepBuilder).Init(s)
}

func (s *CheckVersionCompatibilityStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckVersionCompatibilityStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	checkErr := s.checkRequirement(ctx)
	if checkErr == nil {
		return true, nil
	}
	return false, nil
}

func (s *CheckVersionCompatibilityStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	if err := s.checkRequirement(ctx); err != nil {
		result.MarkFailed(err, "Version compatibility check failed")
		return result, err
	}
	result.MarkCompleted("All component versions are compatible")
	return result, nil
}

func (s *CheckVersionCompatibilityStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

// checkRequirement performs all version compatibility validations.
func (s *CheckVersionCompatibilityStep) checkRequirement(ctx runtime.ExecutionContext) error {
	cfg := ctx.GetClusterConfig()
	log := ctx.GetLogger().With("step", s.Base.Meta.Name)

	kubeVersionStr := cfg.Spec.Kubernetes.Version
	if kubeVersionStr == "" {
		return fmt.Errorf("Kubernetes version is not configured")
	}

	kubeVersion, err := semver.NewVersion(kubeVersionStr)
	if err != nil {
		return fmt.Errorf("invalid Kubernetes version format '%s': %w", kubeVersionStr, err)
	}

	var failures []string

	// 1. Container runtime version compatibility
	if err := s.checkContainerRuntime(cfg, kubeVersion, log); err != nil {
		failures = append(failures, err.Error())
	}

	// 2. etcd version compatibility
	if err := s.checkEtcd(cfg, kubeVersion, log); err != nil {
		failures = append(failures, err.Error())
	}

	// 3. CNI plugin version compatibility
	if err := s.checkCNI(cfg, kubeVersion, log); err != nil {
		failures = append(failures, err.Error())
	}

	if len(failures) > 0 {
		return fmt.Errorf("version compatibility check failed:\n  - %s", strings.Join(failures, "\n  - "))
	}

	log.Infof("All component versions are compatible with Kubernetes version: kubeVersion=%s", kubeVersionStr)
	return nil
}

// checkContainerRuntime validates the configured container runtime version
// matches the BOM recommendation for the given Kubernetes version.
func (s *CheckVersionCompatibilityStep) checkContainerRuntime(cfg *v1alpha1.Cluster, kubeVersion *semver.Version, log templateLogger) error {
	if cfg.Spec.Kubernetes == nil || cfg.Spec.Kubernetes.ContainerRuntime == nil {
		return nil
	}

	runtimeType := cfg.Spec.Kubernetes.ContainerRuntime.Type
	if runtimeType == "" {
		return nil
	}

	bomVersion := binary.GetBinaryVersionFromBOM(string(runtimeType), kubeVersion.Original())
	if bomVersion == "" {
		return nil
	}

	configuredVersion := cfg.Spec.Kubernetes.ContainerRuntime.Version
	if configuredVersion != "" && configuredVersion != bomVersion {
		return fmt.Errorf("container runtime %s: configured version '%s' does not match BOM recommendation '%s' for K8s %s; update to '%s' or remove the version field",
			runtimeType, configuredVersion, bomVersion, kubeVersion.Original(), bomVersion)
	}

	log.Infof("Container runtime version check passed: type=%s bomVersion=%s kubeVersion=%s", runtimeType, bomVersion, kubeVersion.Original())
	return nil
}

// checkEtcd validates that the etcd version is compatible with the target
// Kubernetes version. Skipped for external or kubeadm-managed etcd.
func (s *CheckVersionCompatibilityStep) checkEtcd(cfg *v1alpha1.Cluster, kubeVersion *semver.Version, log templateLogger) error {
	if cfg.Spec.Etcd == nil {
		return nil
	}

	etcdType := cfg.Spec.Etcd.Type
	if etcdType == "" || etcdType == string(common.EtcdDeploymentTypeExternal) || etcdType == string(common.EtcdDeploymentTypeKubeadm) {
		return nil
	}

	bomVersion := binary.GetBinaryVersionFromBOM(binary.ComponentEtcd, kubeVersion.Original())
	if bomVersion == "" {
		return fmt.Errorf("etcd: no compatible version found in BOM for Kubernetes %s", kubeVersion.Original())
	}

	configuredVersion := cfg.Spec.Etcd.Version
	if configuredVersion != "" && configuredVersion != bomVersion {
		return fmt.Errorf("etcd: configured version '%s' does not match BOM recommendation '%s' for K8s %s; update to '%s' or remove the version field",
			configuredVersion, bomVersion, kubeVersion.Original(), bomVersion)
	}

	log.Infof("Etcd version check passed: bomVersion=%s kubeVersion=%s", bomVersion, kubeVersion.Original())
	return nil
}

// checkCNI validates that the configured CNI plugin has a BOM entry for
// the target Kubernetes version by checking the helm chart BOM.
func (s *CheckVersionCompatibilityStep) checkCNI(cfg *v1alpha1.Cluster, kubeVersion *semver.Version, log templateLogger) error {
	if cfg.Spec.Network == nil || cfg.Spec.Network.Plugin == "" {
		return nil
	}

	cniPlugin := cfg.Spec.Network.Plugin
	chartInfo := helm.GetChartInfo(cniPlugin, kubeVersion.Original())
	if chartInfo == nil {
		return fmt.Errorf("CNI plugin '%s': no compatible helm chart found in BOM for Kubernetes %s; ensure the plugin is supported for this K8s version",
			cniPlugin, kubeVersion.Original())
	}

	log.Infof("CNI helm chart BOM entry found: plugin=%s chart=%s version=%s kubeVersion=%s", cniPlugin, chartInfo.Name, chartInfo.Version, kubeVersion.Original())
	return nil
}
