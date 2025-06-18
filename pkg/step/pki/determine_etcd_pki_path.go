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
func (e *DetermineEtcdPKIPathStepExecutor) resolvePKIPath(ctx *runtime.Context, stepSpec *DetermineEtcdPKIPathStepSpec) (string, error) {
	workDir := stepSpec.BaseWorkDir // Prioritize explicitly set BaseWorkDir in spec

	if workDir == "" { // If not in spec, try SharedData
		if val, exists := ctx.SharedData.Load(stepSpec.BaseWorkDirSharedDataKey); exists {
			if wd, ok := val.(string); ok {
				workDir = wd
				ctx.Logger.Debugf("Using BaseWorkDir '%s' from SharedData key '%s'", workDir, stepSpec.BaseWorkDirSharedDataKey)
			} else {
				ctx.Logger.Warnf("Invalid value type for BaseWorkDir in SharedData key '%s'. Expected string.", stepSpec.BaseWorkDirSharedDataKey)
			}
		}
	}

	if workDir == "" {
		// Fallback if no BaseWorkDir is provided via spec or SharedData.
		// This path should ideally be configurable globally for the cluster/process.
		// Using a path relative to the current user's home or a common system temp area might be safer
		// than a fixed root path if not running as root or if permissions are a concern.
		// For this example, using a path that implies it's part of the application's data area.
		// This could also come from `ctx.KubeConf.WorkDir` if such a field is standard.
		// For this step, we are determining a local path on the control/execution node.
		// This is NOT a path on a remote ctx.Host.
		workDir = filepath.Join(os.TempDir(), "kubexms_cluster_pki") // Example default
		ctx.Logger.Debugf("BaseWorkDir not found in spec or SharedData. Defaulting to: %s", workDir)

		// A note on ctx.WorkDir vs ctx.Host.WorkDir:
		// ctx.Host.WorkDir is typically for temporary work on a *remote* host.
		// ctx.WorkDir (if it existed on runtime.Context directly) would be for the *local* process.
		// The original `runtime.GetWorkDir()` implies a local path.
		// If this step runs on the same node where KubeKey/KubeXMS process is running, then os.MkdirAll is fine.
	}

	if stepSpec.EtcdPKISubPath == "" { // Should be set by PopulateDefaults, but double check
		return "", fmt.Errorf("EtcdPKISubPath is empty, cannot determine full PKI path")
	}

	return filepath.Join(workDir, stepSpec.EtcdPKISubPath), nil
}

// Check determines if the etcd PKI path has already been determined and directory exists.
func (e *DetermineEtcdPKIPathStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	stepSpec, ok := s.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for %s", s, stepSpec.GetName())
	}
	stepSpec.PopulateDefaults() // Ensure defaults are applied
	logger := ctx.Logger.SugaredLogger.With("step", stepSpec.GetName()) // Assuming no host for local path determination

	pkiPath, err := e.resolvePKIPath(ctx, stepSpec)
	if err != nil {
		logger.Warnf("Could not resolve PKI path for check: %v", err)
		return false, nil // Cannot resolve, so not done
	}
	if pkiPath == "" { // Should be caught by err != nil, but as a safeguard
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

	// Check if it's already in SharedData with the correct value
	if val, exists := ctx.SharedData.Load(stepSpec.OutputPKIPathSharedDataKey); exists {
		if storedPath, okStr := val.(string); okStr && storedPath == pkiPath {
			logger.Infof("Etcd PKI path %s already determined and matches SharedData.", pkiPath)
			return true, nil
		}
		logger.Infof("Etcd PKI path in SharedData (%s) does not match resolved path (%s) or is invalid type.", val, pkiPath)
	}

	return false, nil // Not in SharedData or mismatch, ensure Execute runs to set/confirm it.
}

// Execute determines and creates the etcd PKI path, storing it in SharedData.
func (e *DetermineEtcdPKIPathStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepSpec, ok := s.(*DetermineEtcdPKIPathStepSpec)
	if !ok {
		return step.NewResultForSpec(s, fmt.Errorf("unexpected spec type %T", s))
	}
	stepSpec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger.With("step", stepSpec.GetName())
	// This step operates on the local filesystem, so Host in result might be misleading if set from ctx.Host.
	// For now, use a generic result or indicate it's a local operation.
	res := step.NewResult(stepSpec.GetName(), "localhost", time.Now(), nil)


	pkiPath, err := e.resolvePKIPath(ctx, stepSpec)
	if err != nil {
		res.Error = fmt.Errorf("failed to resolve etcd PKI path: %w", err)
		res.SetFailed(); return res
	}
	if pkiPath == "" { // Should be caught by err != nil
		res.Error = fmt.Errorf("resolved etcd PKI path is empty")
		res.SetFailed(); return res
	}

	logger.Infof("Ensuring etcd PKI directory exists: %s", pkiPath)
	// This is a local filesystem operation.
	if err := os.MkdirAll(pkiPath, 0700); err != nil { // Permissions 0700 as PKI material is sensitive.
		res.Error = fmt.Errorf("failed to create etcd PKI directory %s: %w", pkiPath, err)
		res.SetFailed(); return res
	}
	logger.Infof("Etcd PKI directory ensured at %s", pkiPath)

	ctx.SharedData.Store(stepSpec.OutputPKIPathSharedDataKey, pkiPath)
	logger.Infof("Determined and stored etcd PKI path in SharedData ('%s'): %s", stepSpec.OutputPKIPathSharedDataKey, pkiPath)

	res.SetSucceeded()
	return res
}

func init() {
	step.Register(&DetermineEtcdPKIPathStepSpec{}, &DetermineEtcdPKIPathStepExecutor{})
}
