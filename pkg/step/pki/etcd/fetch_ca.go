package etcd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
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

func NewRestoreCAFromRemoteStep(ctx runtime.Context, instanceName string) *RestoreCAFromRemoteStep {
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
	return s
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

func (s *RestoreCAFromRemoteStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Warn("Local etcd CA is missing. Attempting to restore from a remote node.")

	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 {
		return fmt.Errorf("no etcd nodes found in context to restore CA from")
	}
	sourceNode := etcdNodes[0]
	logger.Infof("Using node '%s' as the restore source.", sourceNode.GetName())

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("failed to get connector for remote host '%s': %w", sourceNode.GetName(), err)
	}

	runner := ctx.GetRunner()

	certExists, err := runner.Exists(ctx.GoContext(), conn, s.remoteCaCertPath)
	if err != nil {
		return fmt.Errorf("failed to check for remote CA certificate on node '%s': %w", sourceNode.GetName(), err)
	}
	if !certExists {
		return fmt.Errorf("remote CA certificate '%s' not found on node '%s'. Cannot restore", s.remoteCaCertPath, sourceNode.GetName())
	}

	if err := os.MkdirAll(s.localCertsDir, 0755); err != nil {
		return fmt.Errorf("failed to create local certs directory '%s': %w", s.localCertsDir, err)
	}

	if !helpers.IsFileExist(s.localCaCertPath) {
		logger.Infof("Restoring CA certificate from remote:%s to local:%s", s.remoteCaCertPath, s.localCaCertPath)
		if err := runner.Fetch(ctx.GoContext(), conn, s.remoteCaCertPath, s.localCaCertPath, s.Sudo); err != nil {
			return fmt.Errorf("failed to fetch CA certificate: %w", err)
		}
		logger.Info("CA certificate restored successfully.")
	}

	if !helpers.IsFileExist(s.localCaKeyPath) {
		keyExists, _ := runner.Exists(ctx.GoContext(), conn, s.remoteCaKeyPath)
		if keyExists {
			logger.Infof("Restoring CA key from remote:%s to local:%s", s.remoteCaKeyPath, s.localCaKeyPath)
			if err := runner.Fetch(ctx.GoContext(), conn, s.remoteCaKeyPath, s.localCaKeyPath, s.Sudo); err != nil {
				return fmt.Errorf("failed to fetch CA key: %w", err)
			}
			logger.Info("CA key restored successfully.")
		} else {
			logger.Warnf("CA key was not found on the remote node '%s' and could not be restored.", sourceNode.GetName())
		}
	}

	return nil
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
