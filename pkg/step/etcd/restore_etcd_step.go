package etcd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// RestoreEtcdStep performs a restore of the etcd datastore from a snapshot.
// This is a complex operation and typically involves multiple actions coordinated by a task.
// This step focuses on the `etcdctl snapshot restore` command itself.
// Pre-conditions (handled by other steps/task):
// - All etcd nodes' services are stopped.
// - Old data directories are cleaned/moved.
// - Snapshot file is available on the node where restore command will run.
type RestoreEtcdStep struct {
	meta                  spec.StepMeta
	SnapshotFilePath      string // Path to the snapshot.db file on the node
	NewDataDir            string // Path to the new data directory where etcd will be restored
	InitialCluster        string // Full initial-cluster string for the new cluster (e.g., "node1=https://ip1:2380,...")
	InitialClusterToken   string // Token for the new cluster
	InitialAdvertisePeerURL string // This node's peer URL for the new cluster (e.g., "https://<this_node_ip>:2380")
	Name                  string // This node's name in the new cluster
	EtcdctlPath           string // Path to etcdctl binary
	Sudo                  bool   // Sudo for etcdctl command and directory operations
	SkipHashCheck         bool   // Corresponds to --skip-hash-check for etcdctl snapshot restore
}

// NewRestoreEtcdStep creates a new RestoreEtcdStep.
func NewRestoreEtcdStep(instanceName, snapshotPath, newDataDir, name, peerURL, initialCluster, token, etcdctlPath string, skipHashCheck, sudo bool) step.Step {
	stepName := instanceName
	if stepName == "" {
		stepName = "RestoreEtcdFromSnapshot"
	}
	ctlPath := etcdctlPath
	if ctlPath == "" {
		ctlPath = "etcdctl" // Assume in PATH
	}
	ndd := newDataDir
	if ndd == "" {
		ndd = "/var/lib/etcd-restored" // Must be different from old data dir or old one must be cleared
	}

	return &RestoreEtcdStep{
		meta: spec.StepMeta{
			Name:        stepName,
			Description: fmt.Sprintf("Restores etcd datastore from %s to %s", snapshotPath, ndd),
		},
		SnapshotFilePath:      snapshotPath,
		NewDataDir:            ndd,
		Name:                  name,
		InitialAdvertisePeerURL: peerURL,
		InitialCluster:        initialCluster,
		InitialClusterToken:   token,
		EtcdctlPath:           ctlPath,
		SkipHashCheck:         skipHashCheck,
		Sudo:                  sudo,
	}
}

func (s *RestoreEtcdStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *RestoreEtcdStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Check if etcdctl is available
	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, s.EtcdctlPath); err != nil {
		logger.Error("etcdctl command not found.", "path_tried", s.EtcdctlPath, "error", err)
		return false, fmt.Errorf("etcdctl command '%s' not found: %w", s.EtcdctlPath, err)
	}

	// Check if snapshot file exists
	snapshotExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.SnapshotFilePath)
	if err != nil {
		logger.Warn("Failed to check existence of snapshot file.", "path", s.SnapshotFilePath, "error", err)
		return false, fmt.Errorf("failed to check snapshot file %s: %w", s.SnapshotFilePath, err)
	}
	if !snapshotExists {
		logger.Error("Snapshot file does not exist.", "path", s.SnapshotFilePath)
		return false, fmt.Errorf("snapshot file %s not found", s.SnapshotFilePath)
	}

	// Check if NewDataDir already exists and is not empty (could indicate a previous failed restore or unexpected state)
	newDataDirExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, s.NewDataDir)
	if newDataDirExists {
		// Simple check: if NewDataDir/member/snap exists, it's likely a populated data dir.
		memberSnapDir := filepath.Join(s.NewDataDir, "member", "snap")
		alreadyRestored, _ := runnerSvc.Exists(ctx.GoContext(), conn, memberSnapDir)
		if alreadyRestored {
			logger.Info("New data directory already exists and appears populated. Restore might be complete or data dir not cleaned.", "path", s.NewDataDir)
			return true, nil // Consider it done if target dir looks like a valid etcd data dir
		}
		logger.Warn("New data directory exists but does not appear to be a fully restored etcd datadir. Will attempt restore.", "path", s.NewDataDir)
	}


	logger.Info("Preconditions for etcd restore seem met (etcdctl found, snapshot exists, new data dir is clear or non-existent).")
	return false, nil // Proceed with restore
}

func (s *RestoreEtcdStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Ensure NewDataDir parent exists, and NewDataDir itself is empty or does not exist.
	// etcdctl snapshot restore requires the target data directory to not exist or be empty.
	newDataDirExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, s.NewDataDir)
	if newDataDirExists {
		// Attempt to remove it only if it's confirmed empty or known to be from a previous failed attempt.
		// For safety, this step expects the directory to be cleared by a prior step if it existed.
		// If it's not empty, etcdctl restore will fail.
		logger.Warn("Target new data directory already exists. Ensure it's empty or removed before restore.", "path", s.NewDataDir)
		// One might add a `Remove(s.NewDataDir)` here if it's safe.
	} else {
		parentDir := filepath.Dir(s.NewDataDir)
		logger.Info("Ensuring parent of new data directory exists.", "path", parentDir)
		if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, parentDir, "0700", s.Sudo); err != nil {
			return fmt.Errorf("failed to create parent directory for new data dir %s: %w", parentDir, err)
		}
	}


	// Construct etcdctl snapshot restore command
	// ETCDCTL_API=3 etcdctl snapshot restore snapshot.db \
	//   --name m1 \
	//   --initial-cluster "m1=http://10.0.1.10:2380,m2=http://10.0.1.11:2380,m3=http://10.0.1.12:2380" \
	//   --initial-cluster-token etcd-cluster-1 \
	//   --initial-advertise-peer-urls http://10.0.1.10:2380 \
	//   --data-dir /var/lib/etcd-tmp
	cmdArgs := []string{
		"ETCDCTL_API=3",
		s.EtcdctlPath,
		"snapshot", "restore", s.SnapshotFilePath,
		"--name=" + s.Name,
		"--initial-cluster=" + s.InitialCluster,
		"--initial-cluster-token=" + s.InitialClusterToken,
		"--initial-advertise-peer-urls=" + s.InitialAdvertisePeerURL,
		"--data-dir=" + s.NewDataDir,
	}
	if s.SkipHashCheck {
		cmdArgs = append(cmdArgs, "--skip-hash-check")
	}
	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Executing etcd restore command.", "command", cmd)
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Error("Etcd restore command failed.", "command", cmd, "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("etcd restore command '%s' failed: %w. Stderr: %s", cmd, err, string(stderr))
	}

	logger.Info("Etcd restore command executed successfully.", "output", string(stdout))
	// After restore, the service needs to be started with this NewDataDir and updated cluster configuration.
	// That is typically handled by subsequent GenerateEtcdConfigStep (pointing to NewDataDir) and ManageEtcdServiceSteps.
	return nil
}

func (s *RestoreEtcdStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rollback for a restore operation could mean deleting the NewDataDir if the restore failed
	// or if subsequent steps in a larger restore task failed.
	logger.Info("Attempting to remove newly created data directory for rollback.", "path", s.NewDataDir)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil // Best effort
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.NewDataDir, s.Sudo); err != nil {
		logger.Warn("Failed to remove new data directory during rollback (best effort).", "path", s.NewDataDir, "error", err)
	} else {
		logger.Info("Successfully removed new data directory (if it existed).", "path", s.NewDataDir)
	}
	return nil
}

var _ step.Step = (*RestoreEtcdStep)(nil)
