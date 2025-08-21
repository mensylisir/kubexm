package kubeadm

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	remoteBackupSuffix = "kubexm-backup"
)

type KubeadmBackupRemotePKIStep struct {
	step.Base
	remotePkiDir    string
	remoteBackupDir string
}

type KubeadmBackupRemotePKIStepBuilder struct {
	step.Builder[KubeadmBackupRemotePKIStepBuilder, *KubeadmBackupRemotePKIStep]
}

func NewKubeadmBackupRemotePKIStepBuilder(ctx runtime.Context, instanceName string) *KubeadmBackupRemotePKIStepBuilder {
	remotePkiDir := common.DefaultPKIPath
	backupDir := fmt.Sprintf("%s.%s-%d", remotePkiDir, remoteBackupSuffix, time.Now().Unix())

	s := &KubeadmBackupRemotePKIStep{
		remotePkiDir:    remotePkiDir,
		remoteBackupDir: backupDir,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("Back up remote PKI directory '%s' on the node", remotePkiDir)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(KubeadmBackupRemotePKIStepBuilder).Init(s)
	return b
}

func (s *KubeadmBackupRemotePKIStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmBackupRemotePKIStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if _, ok := ctx.GetTaskCache().Get(common.CacheKubeCertsBackupPath); ok {
		logger.Info("Remote PKI directory has already been backed up in this task. Step is done.")
		return true, nil
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	checkCmd := fmt.Sprintf("[ -d %s ]", s.remotePkiDir)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, checkCmd, s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: source PKI directory '%s' not found on host '%s'", s.remotePkiDir, ctx.GetHost().GetName())
	}

	return false, nil
}

func (s *KubeadmBackupRemotePKIStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Backing up remote PKI directory from '%s' to '%s'...", s.remotePkiDir, s.remoteBackupDir)
	backupCmd := fmt.Sprintf("cp -a %s %s", s.remotePkiDir, s.remoteBackupDir)
	if _, err := runner.Run(ctx.GoContext(), conn, backupCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to back up remote PKI directory on host '%s': %w", ctx.GetHost().GetName(), err)
	}

	ctx.GetTaskCache().Set(common.CacheKubeCertsBackupPath, s.remoteBackupDir)
	logger.Infof("Successfully backed up PKI directory. Backup path '%s' saved to cache.", s.remoteBackupDir)

	return nil
}

func (s *KubeadmBackupRemotePKIStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	backupPath, ok := ctx.GetTaskCache().Get(common.CacheKubeCertsBackupPath)
	if !ok {
		logger.Warn("No backup path found in cache, nothing to roll back.")
		return nil
	}

	backupDir, ok := backupPath.(string)
	if !ok {
		logger.Error("Backup path in cache is not a string, cannot proceed with rollback.")
		return nil
	}

	logger.Warnf("Rolling back by removing the created backup directory '%s'...", backupDir)
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cleanupCmd := fmt.Sprintf("rm -rf %s", backupDir)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove backup directory during rollback. Manual cleanup may be needed. Error: %v", err)
	}

	return nil
}

var _ step.Step = (*KubeadmBackupRemotePKIStep)(nil)
