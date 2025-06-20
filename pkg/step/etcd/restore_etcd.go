package etcd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

// RestoreEtcdStepSpec defines parameters for restoring an etcd instance from a snapshot.
type RestoreEtcdStepSpec struct {
	spec.StepMeta `json:",inline"`

	BackupFilePath           string `json:"backupFilePath,omitempty"`
	TargetDataDir            string `json:"targetDataDir,omitempty"`
	Name                     string `json:"name,omitempty"` // Node name for this member in the new cluster
	InitialCluster           string `json:"initialCluster,omitempty"`
	InitialAdvertisePeerURLs string `json:"initialAdvertisePeerURLs,omitempty"`
	InitialClusterToken      string `json:"initialClusterToken,omitempty"`
	EtcdCtlEndpoint          string `json:"etcdCtlEndpoint,omitempty"` // For etcdctl path, not used in restore command itself
	SkipDataDirCleanup       bool   `json:"skipDataDirCleanup,omitempty"`
	Sudo                     bool   `json:"sudo,omitempty"`
}

// NewRestoreEtcdStepSpec creates a new RestoreEtcdStepSpec.
func NewRestoreEtcdStepSpec(name, description, backupPath, dataDir, nodeName, initialCluster, initialAdvertisePeerURLs, token string) *RestoreEtcdStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Restore etcd from %s", backupPath)
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &RestoreEtcdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		BackupFilePath:           backupPath,
		TargetDataDir:            dataDir,
		Name:                     nodeName,
		InitialCluster:           initialCluster,
		InitialAdvertisePeerURLs: initialAdvertisePeerURLs,
		InitialClusterToken:      token,
	}
}

// Name returns the step's name.
func (s *RestoreEtcdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *RestoreEtcdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *RestoreEtcdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *RestoreEtcdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *RestoreEtcdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *RestoreEtcdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *RestoreEtcdStepSpec) populateDefaults(logger runtime.Logger) {
	if s.InitialClusterToken == "" {
		s.InitialClusterToken = "etcd-cluster-default"
		logger.Debug("InitialClusterToken defaulted.", "token", s.InitialClusterToken)
	}
	if s.EtcdCtlEndpoint == "" { // Though not used in restore, can be defaulted for consistency
		s.EtcdCtlEndpoint = "http://127.0.0.1:2379"
		logger.Debug("EtcdCtlEndpoint defaulted.", "endpoint", s.EtcdCtlEndpoint)
	}


	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Restores etcd member %s from snapshot %s to data directory %s. Initial Cluster: %s, PeerURLs: %s, Token: %s",
			s.Name, s.BackupFilePath, s.TargetDataDir, s.InitialCluster, s.InitialAdvertisePeerURLs, s.InitialClusterToken)
	}
}

// Precheck ensures etcdctl is available and the backup file exists.
func (s *RestoreEtcdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.BackupFilePath == "" || s.TargetDataDir == "" || s.Name == "" || s.InitialCluster == "" || s.InitialAdvertisePeerURLs == "" {
		return false, fmt.Errorf("BackupFilePath, TargetDataDir, Name, InitialCluster, and InitialAdvertisePeerURLs must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, err := conn.LookPath(ctx.GoContext(), "etcdctl"); err != nil {
		return false, fmt.Errorf("etcdctl command not found on host %s: %w", host.GetName(), err)
	}
	logger.Debug("etcdctl command found on host.")

	backupExists, err := conn.Exists(ctx.GoContext(), s.BackupFilePath)
	if err != nil {
		logger.Error("Failed to check backup file existence.", "path", s.BackupFilePath, "error", err)
		return false, fmt.Errorf("failed to check existence of backup file %s on host %s: %w", s.BackupFilePath, host.GetName(), err)
	}
	if !backupExists {
		return false, fmt.Errorf("backup file %s does not exist on host %s", s.BackupFilePath, host.GetName())
	}
	logger.Debug("Backup file exists.", "path", s.BackupFilePath)

	// If not skipping cleanup, check TargetDataDir. If it exists and is not empty, Run will clear it.
	// A true "done" state for restore is complex (is node part of healthy cluster with restored data?).
	// For precheck, we ensure inputs are valid and let Run attempt the operation.
	// If TargetDataDir is already populated correctly from a previous restore, this step might still run
	// to re-join, which etcd restore doesn't do by itself. Restore creates a data dir for a new single-member cluster.
	logger.Info("Precheck complete, restore operation will proceed if invoked.")
	return false, nil
}

// Run executes the etcdctl snapshot restore command.
func (s *RestoreEtcdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.BackupFilePath == "" || s.TargetDataDir == "" || s.Name == "" || s.InitialCluster == "" || s.InitialAdvertisePeerURLs == "" {
		return fmt.Errorf("BackupFilePath, TargetDataDir, Name, InitialCluster, and InitialAdvertisePeerURLs must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	if !s.SkipDataDirCleanup {
		logger.Info("Attempting to clean up target data directory.", "path", s.TargetDataDir)
		rmCmd := fmt.Sprintf("rm -rf %s", s.TargetDataDir)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts)
		if errRm != nil {
			return fmt.Errorf("failed to remove target data directory %s (stderr: %s): %w", s.TargetDataDir, string(stderrRm), errRm)
		}
		logger.Info("Target data directory removed.", "path", s.TargetDataDir)

		mkdirCmd := fmt.Sprintf("mkdir -p %s", s.TargetDataDir)
		_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
		if errMkdir != nil {
			return fmt.Errorf("failed to create target data directory %s (stderr: %s): %w", s.TargetDataDir, string(stderrMkdir), errMkdir)
		}
		logger.Info("Target data directory created.", "path", s.TargetDataDir)
		// Permissions/owner might be needed here if etcd runs as a non-root user.
		// chownCmd := fmt.Sprintf("chown etcd:etcd %s", s.TargetDataDir) // Example
		// if _, _, errChown := conn.Exec(ctx.GoContext(), chownCmd, execOpts); errChown != nil { ... }
	} else {
		logger.Info("Skipping target data directory cleanup as per spec.", "path", s.TargetDataDir)
		// Ensure directory exists even if cleanup is skipped.
		mkdirCmd := fmt.Sprintf("mkdir -p %s", s.TargetDataDir)
		_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
		if errMkdir != nil {
			return fmt.Errorf("failed to ensure target data directory %s exists (stderr: %s): %w", s.TargetDataDir, string(stderrMkdir), errMkdir)
		}
	}

	cmdArgs := []string{
		"etcdctl", "snapshot", "restore", s.BackupFilePath,
		fmt.Sprintf("--name=%s", s.Name),
		fmt.Sprintf("--data-dir=%s", s.TargetDataDir),
		fmt.Sprintf("--initial-cluster=%s", s.InitialCluster),
		fmt.Sprintf("--initial-cluster-token=%s", s.InitialClusterToken),
		fmt.Sprintf("--initial-advertise-peer-urls=%s", s.InitialAdvertisePeerURLs),
	}
	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Executing etcd snapshot restore.", "command", cmd)
	// etcdctl snapshot restore itself usually does not require sudo if the data-dir is writable by the user running it.
	// However, if data-dir was created by root, or if etcdctl needs other privileges, sudo might be needed.
	// The Sudo flag in the spec controls this.
	stdout, stderrRestore, errRestore := conn.Exec(ctx.GoContext(), cmd, execOpts)
	if errRestore != nil {
		return fmt.Errorf("failed to restore etcd snapshot to %s (stdout: %s, stderr: %s): %w", s.TargetDataDir, string(stdout), string(stderrRestore), errRestore)
	}

	logger.Info("Etcd snapshot restored successfully.", "dataDir", s.TargetDataDir, "stdout", string(stdout))
	return nil
}

// Rollback for etcd restore is very risky and not automatically performed.
func (s *RestoreEtcdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Error("Rollback for etcd restore is extremely risky and not automatically performed. Manual intervention is required to assess cluster state and data directory contents.")
	// Removing TargetDataDir on rollback could lead to data loss if the restore was partially successful
	// or if other processes have started using it.
	return nil
}

var _ step.Step = (*RestoreEtcdStepSpec)(nil)
