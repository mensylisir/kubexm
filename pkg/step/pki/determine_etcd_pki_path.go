package pki

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
	// Assuming a constants package might exist for shared keys
	// "github.com/kubexms/kubexms/pkg/common/constants"
)

// SharedData keys related to PKI paths.
const (
	DefaultEtcdPKIPathKey = "etcdPKIPath"       // Stores the determined path for etcd PKI
	DefaultBaseWorkDirKey = "clusterBaseWorkDir" // Key for the cluster's base working directory (e.g., from KubeConf)
)

// DetermineEtcdPKIPathStepSpec defines parameters for determining the etcd PKI path.
type DetermineEtcdPKIPathStepSpec struct {
	BaseWorkDirSharedDataKey string `json:"baseWorkDirSharedDataKey,omitempty"` // Key to get base work dir from SharedData
	BaseWorkDir              string `json:"baseWorkDir,omitempty"`              // Explicit base work directory
	EtcdPKISubPath           string `json:"etcdPKISubPath,omitempty"`           // Subpath for etcd PKI relative to work dir
	OutputPKIPathSharedDataKey string `json:"outputPKIPathSharedDataKey,omitempty"` // Key to store the determined PKI path
}

// GetName returns the name of the step.
func (s *DetermineEtcdPKIPathStepSpec) GetName() string {
	return "Determine Etcd PKI Path"
}

// PopulateDefaults sets default values for the spec.
func (s *DetermineEtcdPKIPathStepSpec) PopulateDefaults() {
	if s.EtcdPKISubPath == "" {
		s.EtcdPKISubPath = "pki/etcd" // Default subpath within the work/cluster directory
	}
	if s.OutputPKIPathSharedDataKey == "" {
		s.OutputPKIPathSharedDataKey = DefaultEtcdPKIPathKey
	}
	if s.BaseWorkDirSharedDataKey == "" {
		s.BaseWorkDirSharedDataKey = DefaultBaseWorkDirKey
	}
}

// DetermineEtcdPKIPathStepExecutor implements the logic.
type DetermineEtcdPKIPathStepExecutor struct{}

// resolvePKIPath determines the absolute path for etcd PKI materials.
func (e *DetermineEtcdPKIPathStepExecutor) resolvePKIPath(ctx runtime.Context, stepSpec *DetermineEtcdPKIPathStepSpec) (string, error) {
	workDir := stepSpec.BaseWorkDir // Prioritize explicitly set BaseWorkDir in spec

	if workDir == "" { // If not in spec, try Task Cache
		if val, exists := ctx.Task().Get(stepSpec.BaseWorkDirSharedDataKey); exists {
			if wd, ok := val.(string); ok {
				workDir = wd
				ctx.Logger.Debugf("Using BaseWorkDir '%s' from Task Cache key '%s'", workDir, stepSpec.BaseWorkDirSharedDataKey)
			} else {
				ctx.Logger.Warnf("Invalid value type for BaseWorkDir in Task Cache key '%s'. Expected string.", stepSpec.BaseWorkDirSharedDataKey)
			}
		}
	}

	if workDir == "" {
		workDir = filepath.Join(os.TempDir(), "kubexms_cluster_pki") // Example default
		ctx.Logger.Debugf("BaseWorkDir not found in spec or Task Cache. Defaulting to: %s", workDir)
	}

	if stepSpec.EtcdPKISubPath == "" { // Should be set by PopulateDefaults, but double check
		return "", fmt.Errorf("EtcdPKISubPath is empty, cannot determine full PKI path")
	}

	return filepath.Join(workDir, stepSpec.EtcdPKISubPath), nil
}

// Check determines if the etcd PKI path has already been determined and directory exists.
func (e *DetermineEtcdPKIPathStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for DetermineEtcdPKIPathStep Check")
	}
	spec, ok := currentFullSpec.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for DetermineEtcdPKIPathStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults() // Ensure defaults are applied
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())

	pkiPath, err := e.resolvePKIPath(ctx, spec)
	if err != nil {
		logger.Warnf("Could not resolve PKI path for check: %v", err)
		return false, nil // Cannot resolve, so not done
	}
	if pkiPath == "" {
		logger.Warnf("Resolved PKI path is empty.")
		return false, nil
	}

	stat, statErr := os.Stat(pkiPath) // This operates on the local filesystem
	if os.IsNotExist(statErr) {
		logger.Debugf("Etcd PKI path %s does not exist.", pkiPath)
		return false, nil
	}
	if statErr != nil {
		return false, fmt.Errorf("failed to stat etcd PKI path %s: %w", pkiPath, statErr)
	}
	if !stat.IsDir() {
		return false, fmt.Errorf("etcd PKI path %s is not a directory", pkiPath)
	}
	logger.Debugf("Etcd PKI directory %s exists.", pkiPath)

	if val, exists := ctx.Task().Get(spec.OutputPKIPathSharedDataKey); exists {
		if storedPath, okStr := val.(string); okStr && storedPath == pkiPath {
			logger.Infof("Etcd PKI path %s already determined and matches Task Cache.", pkiPath)
			return true, nil
		}
		logger.Infof("Etcd PKI path in Task Cache (%v) does not match resolved path (%s) or is invalid type.", val, pkiPath)
	}

	return false, nil // Not in Task Cache or mismatch, ensure Execute runs to set/confirm it.
}

// Execute determines and creates the etcd PKI path, storing it in Task Cache.
func (e *DetermineEtcdPKIPathStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for DetermineEtcdPKIPathStep Execute"))
	}
	spec, ok := currentFullSpec.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for DetermineEtcdPKIPathStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil) // Use new constructor

	pkiPath, err := e.resolvePKIPath(ctx, spec)
	if err != nil {
		res.Error = fmt.Errorf("failed to resolve etcd PKI path: %w", err)
		res.SetFailed(); return res // SetFailed is not a method on step.Result, use direct assignment
	}
	if pkiPath == "" {
		res.Error = fmt.Errorf("resolved etcd PKI path is empty")
		res.Status = step.StatusFailed; return res
	}

	logger.Infof("Ensuring etcd PKI directory exists: %s", pkiPath)
	if err := os.MkdirAll(pkiPath, 0700); err != nil {
		res.Error = fmt.Errorf("failed to create etcd PKI directory %s: %w", pkiPath, err)
		res.Status = step.StatusFailed; return res
	}
	logger.Infof("Etcd PKI directory ensured at %s", pkiPath)

	ctx.Task().Set(spec.OutputPKIPathSharedDataKey, pkiPath)
	logger.Infof("Determined and stored etcd PKI path in Task Cache ('%s'): %s", spec.OutputPKIPathSharedDataKey, pkiPath)

	// SetSucceeded is not a method on step.Result, status is set by NewResult or direct assignment
	// res.SetSucceeded() // Status already set by NewResult if err is nil
	return res
}

func init() {
	step.Register(&DetermineEtcdPKIPathStepSpec{}, &DetermineEtcdPKIPathStepExecutor{})
}
