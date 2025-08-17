package backup

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
	DefaultK8sStateFileName = "cluster-state.yaml"
)

type KubernetesBackupObjectsStep struct {
	step.Base
	localBackupFilePath string
}

type KubernetesBackupObjectsStepBuilder struct {
	step.Builder[KubernetesBackupObjectsStepBuilder, *KubernetesBackupObjectsStep]
}

func NewKubernetesBackupObjectsStepBuilder(ctx runtime.Context, instanceName string) *KubernetesBackupObjectsStepBuilder {
	backupDir := filepath.Join(ctx.GetGlobalWorkDir(), "cluster-backups", time.Now().Format("2006-01-02-150405"))

	s := &KubernetesBackupObjectsStep{
		localBackupFilePath: filepath.Join(backupDir, DefaultK8sStateFileName),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Back up all Kubernetes API objects to a YAML file"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(KubernetesBackupObjectsStepBuilder).Init(s)
	return b
}

func (s *KubernetesBackupObjectsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubernetesBackupObjectsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for Kubernetes objects backup...")

	if helpers.IsFileExist(s.localBackupFilePath) {
		logger.Infof("Local Kubernetes objects backup file '%s' already exists. Step is done.", s.localBackupFilePath)
		return true, nil
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v kubectl", s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: 'kubectl' command not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: Local backup file does not exist and kubectl is available.")
	return false, nil
}

func (s *KubernetesBackupObjectsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.localBackupFilePath), 0755); err != nil {
		return fmt.Errorf("failed to create local backup directory: %w", err)
	}

	logger.Info("Backing up all Kubernetes objects from the cluster...")

	backupCmd := "kubectl --kubeconfig /etc/kubernetes/admin.conf get all --all-namespaces -o yaml"

	stdout, err := runner.Run(ctx.GoContext(), conn, backupCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to execute 'kubectl get all': %w", err)
	}

	logger.Infof("Writing cluster state to local file: '%s'", s.localBackupFilePath)
	if err := os.WriteFile(s.localBackupFilePath, []byte(stdout), 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	logger.Info("Kubernetes objects backup completed successfully.")
	return nil
}

func (s *KubernetesBackupObjectsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by deleting the created local backup file...")

	if err := os.Remove(s.localBackupFilePath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove local backup file '%s' during rollback: %v", s.localBackupFilePath, err)
	}

	logger.Info("Rollback for Kubernetes objects backup finished.")
	return nil
}

var _ step.Step = (*KubernetesBackupObjectsStep)(nil)
