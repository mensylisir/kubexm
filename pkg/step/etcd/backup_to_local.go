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

const (
	DefaultEtcdSnapshotName = "etcd-snapshot.db"
)

type EtcdBackupDataStep struct {
	step.Base
	remoteCertsDir     string
	localSnapshotPath  string
	remoteSnapshotPath string
}

type EtcdBackupDataStepBuilder struct {
	step.Builder[EtcdBackupDataStepBuilder, *EtcdBackupDataStep]
}

func NewEtcdBackupDataStepBuilder(ctx runtime.Context, instanceName string) *EtcdBackupDataStepBuilder {
	backupDir := filepath.Join(ctx.GetGlobalWorkDir(), "etcd-backups")

	s := &EtcdBackupDataStep{
		remoteCertsDir:     common.DefaultEtcdPKIDir,
		localSnapshotPath:  filepath.Join(backupDir, DefaultEtcdSnapshotName),
		remoteSnapshotPath: filepath.Join("/tmp", DefaultEtcdSnapshotName),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Create a snapshot of the etcd database from a healthy member"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(EtcdBackupDataStepBuilder).Init(s)
	return b
}

func (s *EtcdBackupDataStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EtcdBackupDataStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for etcd data backup...")

	if helpers.IsFileExist(s.localSnapshotPath) {
		logger.Infof("Local etcd snapshot '%s' already exists. Step is done.", s.localSnapshotPath)
		return true, nil
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v etcdctl", s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: 'etcdctl' command not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: Local snapshot does not exist and etcdctl is available.")
	return false, nil
}

func (s *EtcdBackupDataStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.localSnapshotPath), 0755); err != nil {
		return fmt.Errorf("failed to create local backup directory: %w", err)
	}

	logger.Infof("Creating etcd snapshot on remote node at '%s'...", s.remoteSnapshotPath)
	nodeName := ctx.GetHost().GetName()
	snapshotCmd := fmt.Sprintf("etcdctl snapshot save %s "+
		"--endpoints=https://127.0.0.1:2379 "+
		"--cacert=%s "+
		"--cert=%s "+
		"--key=%s",
		s.remoteSnapshotPath,
		filepath.Join(s.remoteCertsDir, common.EtcdCaPemFileName),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName)),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName)),
	)

	if _, err := runner.Run(ctx.GoContext(), conn, snapshotCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to create etcd snapshot on host '%s': %w", nodeName, err)
	}
	logger.Info("Snapshot created successfully on remote node.")

	logger.Infof("Fetching snapshot from '%s' to '%s'...", s.remoteSnapshotPath, s.localSnapshotPath)
	if err := runner.Fetch(ctx.GoContext(), conn, s.remoteSnapshotPath, s.localSnapshotPath, s.Sudo); err != nil {
		_ = runner.Remove(ctx.GoContext(), conn, s.remoteSnapshotPath, s.Sudo, false)
		return fmt.Errorf("failed to fetch etcd snapshot: %w", err)
	}
	logger.Info("Snapshot fetched successfully.")

	logger.Infof("Cleaning up remote snapshot file '%s'...", s.remoteSnapshotPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.remoteSnapshotPath, s.Sudo, false); err != nil {
		logger.Warnf("Failed to clean up remote snapshot file. Manual cleanup may be required on host '%s'. Error: %v", nodeName, err)
	}

	logger.Info("Etcd data backup step completed successfully.")
	return nil
}

func (s *EtcdBackupDataStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by deleting the fetched local snapshot file...")

	if err := os.Remove(s.localSnapshotPath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove local snapshot file '%s' during rollback: %v", s.localSnapshotPath, err)
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err == nil {
		runner := ctx.GetRunner()
		logger.Warnf("Attempting to clean up remote snapshot file '%s' during rollback...", s.remoteSnapshotPath)
		_ = runner.Remove(ctx.GoContext(), conn, s.remoteSnapshotPath, s.Sudo, false)
	}

	logger.Info("Rollback for etcd data backup finished.")
	return nil
}

var _ step.Step = (*EtcdBackupDataStep)(nil)
