package kubeadm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

const (
	DefaultEtcdSnapshotName = "etcd-snapshot-kubeadm.db"
)

type KubeadmBackupEtcdStep struct {
	step.Base
	localSnapshotPath  string
	remoteSnapshotPath string
	podSnapshotPath    string
}

type KubeadmBackupEtcdStepBuilder struct {
	step.Builder[KubeadmBackupEtcdStepBuilder, *KubeadmBackupEtcdStep]
}

func NewKubeadmBackupEtcdStepBuilder(ctx runtime.Context, instanceName string) *KubeadmBackupEtcdStepBuilder {
	backupDir := filepath.Join(ctx.GetGlobalWorkDir(), "cluster-backups", time.Now().Format("2006-01-02-150405"))

	s := &KubeadmBackupEtcdStep{
		localSnapshotPath:  filepath.Join(backupDir, DefaultEtcdSnapshotName),
		remoteSnapshotPath: filepath.Join("/tmp", DefaultEtcdSnapshotName),
		podSnapshotPath:    "/etcd-snapshot.db",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Create a snapshot of the kubeadm-managed etcd database"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(KubeadmBackupEtcdStepBuilder).Init(s)
	return b
}

func (s *KubeadmBackupEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmBackupEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for kubeadm etcd data backup...")

	if helpers.IsFileExist(s.localSnapshotPath) {
		logger.Infof("Local etcd snapshot '%s' already exists. Step is done.", s.localSnapshotPath)
		return true, nil
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v kubectl", s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: 'kubectl' command not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed.")
	return false, nil
}

func (s *KubeadmBackupEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.localSnapshotPath), 0755); err != nil {
		return fmt.Errorf("failed to create local backup directory: %w", err)
	}

	nodeName := ctx.GetHost().GetName()
	etcdPodName := fmt.Sprintf("etcd-%s", nodeName)

	logger.Infof("Creating etcd snapshot inside pod '%s'...", etcdPodName)

	snapshotCmd := fmt.Sprintf(
		"kubectl --kubeconfig /etc/kubernetes/admin.conf exec -n kube-system %s -- "+
			"sh -c 'ETCDCTL_API=3 etcdctl snapshot save %s "+
			"--endpoints=https://127.0.0.1:2379 "+
			"--cacert=/etc/kubernetes/pki/etcd/ca.crt "+
			"--cert=/etc/kubernetes/pki/etcd/server.crt "+
			"--key=/etc/kubernetes/pki/etcd/server.key'",
		etcdPodName,
		s.podSnapshotPath,
	)

	if _, err := runner.Run(ctx.GoContext(), conn, snapshotCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to create etcd snapshot inside pod on host '%s': %w", nodeName, err)
	}
	logger.Info("Snapshot created successfully inside the pod.")

	logger.Infof("Copying snapshot from pod to host filesystem at '%s'...", s.remoteSnapshotPath)
	cpCmd := fmt.Sprintf(
		"kubectl --kubeconfig /etc/kubernetes/admin.conf cp -n kube-system %s:%s %s",
		etcdPodName,
		s.podSnapshotPath,
		s.remoteSnapshotPath,
	)
	if _, err := runner.Run(ctx.GoContext(), conn, cpCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to copy snapshot from pod to host: %w", err)
	}

	logger.Infof("Fetching snapshot from host to local workspace at '%s'...", s.localSnapshotPath)
	if err := runner.Fetch(ctx.GoContext(), conn, s.remoteSnapshotPath, s.localSnapshotPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to fetch etcd snapshot from host: %w", err)
	}
	logger.Info("Snapshot fetched successfully.")

	logger.Info("Cleaning up temporary snapshot files...")
	cleanupHostCmd := fmt.Sprintf("rm -f %s", s.remoteSnapshotPath)
	_, _ = runner.Run(ctx.GoContext(), conn, cleanupHostCmd, s.Sudo)

	cleanupPodCmd := fmt.Sprintf(
		"kubectl --kubeconfig /etc/kubernetes/admin.conf exec -n kube-system %s -- rm -f %s",
		etcdPodName,
		s.podSnapshotPath,
	)
	_, _ = runner.Run(ctx.GoContext(), conn, cleanupPodCmd, s.Sudo)

	logger.Info("Kubeadm etcd data backup step completed successfully.")
	return nil
}

func (s *KubeadmBackupEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by deleting the fetched local snapshot file...")

	if err := os.Remove(s.localSnapshotPath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove local snapshot file '%s' during rollback: %v", s.localSnapshotPath, err)
	}

	logger.Info("Rollback for kubeadm etcd data backup finished.")
	return nil
}

var _ step.Step = (*KubeadmBackupEtcdStep)(nil)
