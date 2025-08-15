package etcd

import (
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const DefaultRemoteEtcdCertsDir = common.DefaultEtcdPKIDir

var filesToBackup = []string{
	"ca.key",
	"ca.pem",
	"member-*.pem",
	"admin-*.pem",
	"node-*.pem",
}

type BackupRemoteEtcdCertsStep struct {
	step.Base
	remoteCertsDir string
	BackupDir      string
	remoteFilesMap map[string]string
}

func NewBackupRemoteEtcdCertsStep(ctx runtime.Context, instanceName string) *BackupRemoteEtcdCertsStep {
	s := &BackupRemoteEtcdCertsStep{
		remoteCertsDir: DefaultRemoteEtcdCertsDir,
		BackupDir:      fmt.Sprintf("%s:%s", DefaultRemoteEtcdCertsDir, time.Now().Unix()),
		remoteFilesMap: make(map[string]string),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "[BackupRemoteEtcdCerts]>> Backup etcd certificates from a live node to the control plane"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return s
}

func (s *BackupRemoteEtcdCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *BackupRemoteEtcdCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	requiredRenewCerts, ok := ctx.GetTaskCache().Get(CacheKeyLeafRequiresRenewal)
	if !ok {
		return false, fmt.Errorf("requiresRenewal not found in task cache")
	}

	requiredRenewCA, ok := ctx.GetTaskCache().Get(CacheKeyCARequiresRenewal)
	if !ok {
		return false, fmt.Errorf("requiresRenewal not found in task cache")
	}
	if requiredRenewCA.(bool) == false && requiredRenewCerts.(bool) == false {
		logger.Info("No need to generate new ca certificates. Step is done.")
		return true, nil
	}

	etcdNodes := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdNodes) == 0 {
		return false, fmt.Errorf("no etcd nodes found in context to back up from")
	}
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	logger.Infof("Using node '%s' as the source for the certificate backup.", ctx.GetHost().GetName())

	logger.Infof("Checking for certificate files on %s...", ctx.GetHost().GetName())
	for _, pattern := range filesToBackup {
		remotePathPattern := filepath.Join(s.remoteCertsDir, pattern)
		findCmd := fmt.Sprintf("find %s -type f 2>/dev/null", remotePathPattern)
		stdout, err := runner.Run(ctx.GoContext(), conn, findCmd, s.Sudo)
		if err != nil {
			logger.Warnf("Could not find files for pattern '%s' on %s. This may be expected.", remotePathPattern, ctx.GetHost().GetName())
			continue
		}

		remoteFiles := strings.Fields(stdout)
		for _, remoteFile := range remoteFiles {
			sumCmd := fmt.Sprintf("sha256sum %s", remoteFile)
			sumOut, err := runner.Run(ctx.GoContext(), conn, sumCmd, s.Sudo)
			if err != nil {
				return false, fmt.Errorf("failed to get checksum for remote file '%s': %w", remoteFile, err)
			}
			s.remoteFilesMap[remoteFile] = strings.Fields(string(sumOut))[0]
		}
	}

	if len(s.remoteFilesMap) == 0 {
		logger.Warn("No certificate files found on the remote node. Nothing to back up.")
		return true, nil
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.BackupDir)
	if err != nil {
		logger.Debugf("Backup directory '%s' does not exist. Backup is required.", s.BackupDir)
		return false, nil
	}
	if !exists {
		logger.Debugf("Backup directory '%s' does not exist. Backup is required.", s.BackupDir)
		return false, nil
	}

	for remoteFile, remoteSum := range s.remoteFilesMap {
		backupFile := filepath.Join(s.BackupDir, filepath.Base(remoteFile))
		backupSum, err := ctx.GetRunner().GetSHA256(ctx.GoContext(), conn, backupFile)
		if err != nil || backupSum != remoteSum {
			logger.Infof("Backup file '%s' is missing or outdated. Backup is required.", backupFile)
			return false, nil
		}
	}

	logger.Info("A valid and up-to-date backup already exists. Step is done.")
	return true, nil
}

func (s *BackupRemoteEtcdCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	exists, err := ctx.GetRunner().Exists(ctx.GoContext(), conn, s.BackupDir)
	if !exists || errors.IsNotFound(err) {
		logger.Infof("Creating backup directory '%s'...", s.BackupDir)
		if err := ctx.GetRunner().Mkdirp(ctx.GoContext(), conn, s.BackupDir, "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to create backup directory '%s': %w", s.BackupDir, err)
		}
	}

	logger.Infof("Backup %d certificate files from %s to %s...", len(s.remoteFilesMap), s.remoteCertsDir, s.BackupDir)
	for remoteFile := range s.remoteFilesMap {
		backupFile := filepath.Join(s.BackupDir, filepath.Base(remoteFile))
		logger.Debugf("Copying remote:%s to local:%s", remoteFile, backupFile)

		if err := ctx.GetRunner().CopyFile(ctx.GoContext(), conn, remoteFile, backupFile, false, s.Sudo); err != nil {
			return fmt.Errorf("failed to scp file '%s' from remote node: %w", remoteFile, err)
		}
	}

	logger.Info("Verifying integrity of backup files...")
	for remoteFile, remoteSum := range s.remoteFilesMap {
		backupFile := filepath.Join(s.BackupDir, filepath.Base(remoteFile))
		backupFileSum, err := ctx.GetRunner().GetSHA256(ctx.GoContext(), conn, backupFile)
		if err != nil || backupFileSum != remoteSum {
			return fmt.Errorf("checksum mismatch for file '%s'. Remote: %s, Local: %s", filepath.Base(remoteFile), remoteSum, backupFileSum)
		}
	}
	logger.Infof("Backup completed successfully. %d files are stored in '%s'.", len(s.remoteFilesMap), s.BackupDir)
	return nil
}

func (s *BackupRemoteEtcdCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	if s.BackupDir == "" {
		logger.Info("Backup directory path not set, nothing to roll back.")
		return nil
	}

	logger.Warnf("Rolling back by deleting the local backup directory: %s", s.BackupDir)
	exists, err := ctx.GetRunner().Exists(ctx.GoContext(), conn, s.BackupDir)
	if exists {
		if err := ctx.GetRunner().Remove(ctx.GoContext(), conn, s.BackupDir, s.Sudo, true); err != nil {
			return fmt.Errorf("CRITICAL: failed to delete backup directory '%s' during rollback. Manual cleanup may be required. Error: %w", s.BackupDir, err)
		}
		logger.Info("Local backup directory successfully deleted.")
	} else {
		logger.Info("Local backup directory does not exist, nothing to delete.")
	}

	return nil
}

var _ step.Step = (*BackupRemoteEtcdCertsStep)(nil)
