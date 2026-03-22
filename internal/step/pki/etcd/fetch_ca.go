package etcd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestoreCAFromRemoteStep struct {
	step.Base
	remoteCertsDir   string
	localCertsDir    string
	localCaCertPath  string
	localCaKeyPath   string
	remoteCaCertPath string
	remoteCaKeyPath  string
}

type RestoreCAFromRemoteStepBuilder struct {
	step.Builder[RestoreCAFromRemoteStepBuilder, *RestoreCAFromRemoteStep]
}

func NewRestoreCAFromRemoteStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestoreCAFromRemoteStepBuilder {
	localCertsDir := ctx.GetEtcdCertsDir()
	s := &RestoreCAFromRemoteStep{
		remoteCertsDir:   DefaultRemoteEtcdCertsDir,
		localCertsDir:    localCertsDir,
		localCaCertPath:  filepath.Join(localCertsDir, common.EtcdCaPemFileName),
		localCaKeyPath:   filepath.Join(localCertsDir, common.EtcdCaKeyPemFileName),
		remoteCaCertPath: filepath.Join(DefaultRemoteEtcdCertsDir, common.EtcdCaPemFileName),
		remoteCaKeyPath:  filepath.Join(DefaultRemoteEtcdCertsDir, common.EtcdCaKeyPemFileName),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restore etcd CA from a live node if it's missing locally"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute
	b := new(RestoreCAFromRemoteStepBuilder).Init(s)
	return b
}

func (s *RestoreCAFromRemoteStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestoreCAFromRemoteStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if CA requires restoration...")
	if helpers.IsFileExist(s.localCaCertPath) && helpers.IsFileExist(s.localCaKeyPath) {
		logger.Info("Local etcd CA certificate and key already exist. Step is done.")
		return true, nil
	}
	return false, nil
}

func (s *RestoreCAFromRemoteStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Warn("Local etcd CA is missing. Attempting to restore from a remote node.")

	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 {
		result.MarkFailed(fmt.Errorf("no etcd nodes found"), "no etcd nodes found in context to restore CA from")
		return result, fmt.Errorf("no etcd nodes found in context to restore CA from")
	}
	sourceNode := etcdNodes[0]
	logger.Infof("Using node '%s' as the restore source.", sourceNode.GetName())

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to get connector for remote host '%s'", sourceNode.GetName()))
		return result, err
	}

	runner := ctx.GetRunner()

	certExists, err := runner.Exists(ctx.GoContext(), conn, s.remoteCaCertPath)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to check for remote CA certificate on node '%s'", sourceNode.GetName()))
		return result, err
	}
	if !certExists {
		result.MarkFailed(fmt.Errorf("certificate not found"), fmt.Sprintf("remote CA certificate '%s' not found on node '%s'. Cannot restore", s.remoteCaCertPath, sourceNode.GetName()))
		return result, fmt.Errorf("remote CA certificate '%s' not found on node '%s'. Cannot restore", s.remoteCaCertPath, sourceNode.GetName())
	}

	if err := os.MkdirAll(s.localCertsDir, 0755); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to create local certs directory '%s'", s.localCertsDir))
		return result, err
	}

	if !helpers.IsFileExist(s.localCaCertPath) {
		logger.Infof("Restoring CA certificate from remote:%s to local:%s", s.remoteCaCertPath, s.localCaCertPath)
		if err := runner.Fetch(ctx.GoContext(), conn, s.remoteCaCertPath, s.localCaCertPath, s.Sudo); err != nil {
			result.MarkFailed(err, "failed to fetch CA certificate")
			return result, err
		}
		logger.Info("CA certificate restored successfully.")
	}

	if !helpers.IsFileExist(s.localCaKeyPath) {
		keyExists, _ := runner.Exists(ctx.GoContext(), conn, s.remoteCaKeyPath)
		if keyExists {
			logger.Infof("Restoring CA key from remote:%s to local:%s", s.remoteCaKeyPath, s.localCaKeyPath)
			if err := runner.Fetch(ctx.GoContext(), conn, s.remoteCaKeyPath, s.localCaKeyPath, s.Sudo); err != nil {
				result.MarkFailed(err, "failed to fetch CA key")
				return result, err
			}
			logger.Info("CA key restored successfully.")
		} else {
			logger.Warnf("CA key was not found on the remote node '%s' and could not be restored.", sourceNode.GetName())
		}
	}

	result.MarkCompleted("CA restored successfully")
	return result, nil
}

func (s *RestoreCAFromRemoteStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by deleting restored CA files from local workspace...")

	if err := os.Remove(s.localCaCertPath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove restored CA certificate during rollback: %v", err)
	}
	if err := os.Remove(s.localCaKeyPath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove restored CA key during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*RestoreCAFromRemoteStep)(nil)
