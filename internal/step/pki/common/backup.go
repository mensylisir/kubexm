package common

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// BackupPKIStep backs up PKI directory to a backup location.
type BackupPKIStep struct {
	step.Base
	SourceDir      string
	BackupDir      string
	BackupFileName string
}

type BackupPKIStepBuilder struct {
	step.Builder[BackupPKIStepBuilder, *BackupPKIStep]
}

func NewBackupPKIStepBuilder(ctx runtime.ExecutionContext, instanceName, sourceDir, backupDir string) *BackupPKIStepBuilder {
	timestamp := time.Now().Format("20060102-150405")
	s := &BackupPKIStep{
		SourceDir:      sourceDir,
		BackupDir:      backupDir,
		BackupFileName: fmt.Sprintf("pki-backup-%s.tar.gz", timestamp),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Backup PKI from %s", instanceName, sourceDir)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(BackupPKIStepBuilder).Init(s)
}

func (s *BackupPKIStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *BackupPKIStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *BackupPKIStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	backupPath := filepath.Join(s.BackupDir, s.BackupFileName)

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.BackupDir, "0755", s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to create backup directory")
		return result, err
	}

	logger.Infof("Creating compressed backup of %s to %s", s.SourceDir, backupPath)

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to get host facts for compression")
		return result, err
	}

	sources := []string{s.SourceDir}
	if err := runner.Compress(ctx.GoContext(), conn, facts, backupPath, sources, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to backup PKI")
		return result, err
	}

	logger.Infof("PKI backed up successfully to %s", backupPath)
	result.MarkCompleted(fmt.Sprintf("PKI backed up to %s", backupPath))
	return result, nil
}

func (s *BackupPKIStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	backupPath := filepath.Join(s.BackupDir, s.BackupFileName)
	logger.Warnf("Rolling back by removing backup %s", backupPath)
	runner.Remove(ctx.GoContext(), conn, backupPath, true, false)
	return nil
}

var _ step.Step = (*BackupPKIStep)(nil)
