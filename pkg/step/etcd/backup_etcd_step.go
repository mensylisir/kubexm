package etcd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// BackupEtcdStep performs a snapshot backup of the etcd datastore.
type BackupEtcdStep struct {
	meta             spec.StepMeta
	Endpoints        string // Comma-separated list of etcd endpoints, e.g., "https://127.0.0.1:2379"
	CACertPath       string // Path to etcd CA certificate
	CertPath         string // Path to etcd client certificate
	KeyPath          string // Path to etcd client key
	BackupFilePath   string // Full path on the etcd node where the snapshot.db file will be saved
	EtcdctlPath      string // Path to the etcdctl binary, defaults to "etcdctl" (expect in PATH)
	Sudo             bool   // Whether to use sudo for etcdctl command (if needed for cert access or etcdctl path)
}

// NewBackupEtcdStep creates a new BackupEtcdStep.
func NewBackupEtcdStep(instanceName, endpoints, caPath, certPath, keyPath, backupFilePath, etcdctlPath string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "BackupEtcd"
	}
	ep := endpoints
	if ep == "" {
		ep = "https://127.0.0.1:2379" // Default to local endpoint
	}
	bfp := backupFilePath
	if bfp == "" {
		// Default backup path, e.g., /var/backups/etcd/snapshot-<timestamp>.db
		timestamp := time.Now().Format("2006-01-02T150405Z0700")
		bfp = filepath.Join("/var/backups/etcd", fmt.Sprintf("snapshot-%s.db", timestamp))
	}
	ctlPath := etcdctlPath
	if ctlPath == "" {
		ctlPath = "etcdctl" // Assume in PATH
	}

	return &BackupEtcdStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Backs up etcd datastore from %s to %s", ep, bfp),
		},
		Endpoints:      ep,
		CACertPath:     caPath,     // e.g., /etc/etcd/pki/ca.pem
		CertPath:       certPath,   // e.g., /etc/etcd/pki/apiserver-etcd-client.pem or a specific backup client cert
		KeyPath:        keyPath,    // e.g., /etc/etcd/pki/apiserver-etcd-client-key.pem
		BackupFilePath: bfp,
		EtcdctlPath:    ctlPath,
		Sudo:           sudo,
	}
}

func (s *BackupEtcdStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *BackupEtcdStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	// Precheck for backup might involve checking if etcdctl exists and if the cluster is healthy.
	// However, a direct precheck for "is backup already done?" is not straightforward unless
	// we check for the existence of s.BackupFilePath, but that might be from a previous, different backup.
	// For simplicity, this precheck will be minimal or rely on the task not to run it if not needed.

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Check if etcdctl is available
	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, s.EtcdctlPath); err != nil {
		logger.Error("etcdctl command not found or not in PATH.", "path_tried", s.EtcdctlPath, "error", err)
		return false, fmt.Errorf("etcdctl command '%s' not found: %w", s.EtcdctlPath, err)
	}

	// Check if backup directory exists and is writable (complex to check writability without trying)
	backupDir := filepath.Dir(s.BackupFilePath)
	dirExists, err := runnerSvc.Exists(ctx.GoContext(), conn, backupDir)
	if err != nil {
		logger.Warn("Failed to check existence of backup directory, will attempt backup.", "dir", backupDir, "error", err)
		return false, nil
	}
	if !dirExists {
		logger.Info("Backup directory does not exist, will be created by Run.", "dir", backupDir)
		return false, nil // Let Run create it
	}

	logger.Info("Precheck for etcd backup passed (etcdctl found, backup dir existence checked). Actual backup will proceed.")
	return false, nil // Always run the backup when this step is scheduled.
}

func (s *BackupEtcdStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	backupDir := filepath.Dir(s.BackupFilePath)
	logger.Info("Ensuring backup directory exists.", "path", backupDir)
	// Permissions for backup dir should be restrictive, e.g., 0700, owned by user running etcdctl.
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, backupDir, "0700", s.Sudo); err != nil {
		return fmt.Errorf("failed to create backup directory %s: %w", backupDir, err)
	}

	// Construct etcdctl command
	// ETCDCTL_API=3 etcdctl snapshot save /path/to/snapshot.db \
	//   --endpoints=https://127.0.0.1:2379 \
	//   --cacert=/etc/etcd/pki/ca.pem \
	//   --cert=/etc/etcd/pki/server.pem \
	//   --key=/etc/etcd/pki/server-key.pem
	cmdArgs := []string{
		"ETCDCTL_API=3",
		s.EtcdctlPath,
		"snapshot", "save", s.BackupFilePath,
		"--endpoints=" + s.Endpoints,
	}
	if s.CACertPath != "" {
		cmdArgs = append(cmdArgs, "--cacert="+s.CACertPath)
	}
	if s.CertPath != "" {
		cmdArgs = append(cmdArgs, "--cert="+s.CertPath)
	}
	if s.KeyPath != "" {
		cmdArgs = append(cmdArgs, "--key="+s.KeyPath)
	}
	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Executing etcd backup command.", "command", cmd)
	// Backup can take time, consider a timeout if necessary, passed via ExecOptions
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Error("Etcd backup command failed.", "command", cmd, "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("etcd backup command '%s' failed: %w. Stderr: %s", cmd, err, string(stderr))
	}

	logger.Info("Etcd backup command executed successfully.", "output", string(stdout))
	// Verify backup file existence and size (optional)
	backupStat, errStat := runnerSvc.Stat(ctx.GoContext(), conn, s.BackupFilePath)
	if errStat != nil {
		logger.Warn("Could not stat backup file after creation.", "path", s.BackupFilePath, "error", errStat)
	} else if backupStat.Size == 0 {
		logger.Warn("Backup file was created but is empty.", "path", s.BackupFilePath)
		// Consider this an error depending on strictness
		// return fmt.Errorf("etcd backup file %s is empty after snapshot save", s.BackupFilePath)
	} else {
		logger.Info("Backup file created successfully.", "path", s.BackupFilePath, "size", backupStat.Size)
	}

	return nil
}

func (s *BackupEtcdStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rollback for a backup operation could mean deleting the created backup file.
	// This is often desired to clean up if the overall process fails after this step.
	logger.Info("Attempting to remove created backup file for rollback.", "path", s.BackupFilePath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil // Best effort
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.BackupFilePath, s.Sudo); err != nil {
		logger.Warn("Failed to remove backup file during rollback (best effort).", "path", s.BackupFilePath, "error", err)
	} else {
		logger.Info("Successfully removed backup file (if it existed).", "path", s.BackupFilePath)
	}
	return nil
}

var _ step.Step = (*BackupEtcdStep)(nil)
