package etcd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type BackupEtcdStep struct {
	step.Base
	RemoteBackupDir   string
	BackupFileName    string
	EtcdctlBinaryPath string
	VerifySnapshot    bool
}

type BackupEtcdStepBuilder struct {
	step.Builder[BackupEtcdStepBuilder, *BackupEtcdStep]
}

func NewBackupEtcdStepBuilder(ctx runtime.Context, instanceName string) *BackupEtcdStepBuilder {
	s := &BackupEtcdStep{
		RemoteBackupDir:   common.DefaultEtcdBackupDir,
		BackupFileName:    "",
		EtcdctlBinaryPath: filepath.Join(common.DefaultBinDir, "etcdctl"),
		VerifySnapshot:    true,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Backup etcd cluster data on current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(BackupEtcdStepBuilder).Init(s)
	return b
}

func (b *BackupEtcdStepBuilder) WithRemoteBackupDir(path string) *BackupEtcdStepBuilder {
	b.Step.RemoteBackupDir = path
	return b
}

func (b *BackupEtcdStepBuilder) WithBackupFileName(name string) *BackupEtcdStepBuilder {
	b.Step.BackupFileName = name
	return b
}

func (b *BackupEtcdStepBuilder) WithVerifySnapshot(verify bool) *BackupEtcdStepBuilder {
	b.Step.VerifySnapshot = verify
	return b
}

func (s *BackupEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *BackupEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *BackupEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteBackupDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote backup directory %s: %w", s.RemoteBackupDir, err)
	}

	fileName := s.BackupFileName
	if fileName == "" {
		timestamp := time.Now().UTC().Format("2006-01-02-150405")
		fileName = fmt.Sprintf("etcd-snapshot-%s.db", timestamp)
		s.BackupFileName = fileName
	}
	remoteBackupPath := filepath.Join(s.RemoteBackupDir, s.BackupFileName)
	logger.Info("Preparing to save etcd snapshot...", "path", remoteBackupPath)

	caPath, certPath, keyPath := getEtcdctlCertPaths(ctx.GetHost().GetName())

	endpoint := "https://127.0.0.1:2379"

	saveCmd := fmt.Sprintf("ETCDCTL_API=3 %s snapshot save %s --endpoints=%s --cacert=%s --cert=%s --key=%s",
		s.EtcdctlBinaryPath,
		remoteBackupPath,
		endpoint,
		caPath,
		certPath,
		keyPath,
	)

	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, saveCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to save etcd snapshot: %w, stderr: %s", err, stderr)
	}

	logger.Info("Etcd snapshot saved successfully.", "path", remoteBackupPath)

	if s.VerifySnapshot {
		logger.Info("Verifying the integrity of the snapshot...")
		statusCmd := fmt.Sprintf("ETCDCTL_API=3 %s snapshot status %s", s.EtcdctlBinaryPath, remoteBackupPath)

		if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, statusCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to verify etcd snapshot: %w, stderr: %s", err, stderr)
		}
		logger.Info("Snapshot verification successful.", "path", remoteBackupPath)
	}
	return nil
}

func (s *BackupEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	remoteBackupPath := filepath.Join(s.RemoteBackupDir, s.BackupFileName)
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback")
		return nil
	}

	logger.Warn("Rolling back by removing the created snapshot file", "path", remoteBackupPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteBackupPath, s.Sudo, false); err != nil {
		logger.Error(err, "Failed to remove snapshot file during rollback")
	}

	return nil
}

var _ step.Step = (*BackupEtcdStep)(nil)
