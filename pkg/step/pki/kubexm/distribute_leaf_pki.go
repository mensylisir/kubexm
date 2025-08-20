package kubexm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DistributeLeafCertsStep struct {
	step.Base
	localKubeCertsDir string
	remoteKubePkiDir  string
	filesToSync       []string
}

type DistributeLeafCertsStepBuilder struct {
	step.Builder[DistributeLeafCertsStepBuilder, *DistributeLeafCertsStep]
}

func NewDistributeLeafCertsStepBuilder(ctx runtime.Context, instanceName string) *DistributeLeafCertsStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	remotePkiDir := common.DefaultPKIPath

	s := &DistributeLeafCertsStep{
		localKubeCertsDir: localCertsDir,
		remoteKubePkiDir:  remotePkiDir,
		filesToSync: []string{
			"apiserver",
			"apiserver-kubelet-client",
			"front-proxy-client",
			"admin",
			"controller-manager",
			"scheduler",
			"kube-proxy-client",
		},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Distribute renewed Kubernetes leaf certificates and keys to the master node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(DistributeLeafCertsStepBuilder).Init(s)
	return b
}

func (s *DistributeLeafCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeLeafCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for leaf certificate distribution...")

	if _, ok := ctx.GetTaskCache().Get(CacheKeyRemotePKIBackupPath); !ok {
		return false, fmt.Errorf("precheck failed: remote PKI backup path not found in cache. The backup step must run first")
	}

	for _, baseName := range s.filesToSync {
		localCert := filepath.Join(s.localKubeCertsDir, fmt.Sprintf("%s.crt", baseName))
		localKey := filepath.Join(s.localKubeCertsDir, fmt.Sprintf("%s.key", baseName))
		if _, err := os.Stat(localCert); os.IsNotExist(err) {
			return false, fmt.Errorf("precheck failed: local source file '%s' not found", localCert)
		}
		if _, err := os.Stat(localKey); os.IsNotExist(err) {
			return false, fmt.Errorf("precheck failed: local source file '%s' not found", localKey)
		}
	}

	logger.Info("Precheck passed: backup found and all local source files exist.")
	return false, nil
}

func (s *DistributeLeafCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Distributing renewed Kubernetes leaf certificates and keys...")
	for _, baseName := range s.filesToSync {
		log := logger.With("certificate", baseName)

		localCertPath := filepath.Join(s.localKubeCertsDir, fmt.Sprintf("%s.crt", baseName))
		remoteCertPath := filepath.Join(s.remoteKubePkiDir, fmt.Sprintf("%s.crt", baseName))

		localKeyPath := filepath.Join(s.localKubeCertsDir, fmt.Sprintf("%s.key", baseName))
		remoteKeyPath := filepath.Join(s.remoteKubePkiDir, fmt.Sprintf("%s.key", baseName))

		log.Infof("Uploading new certificate and key...")
		if err := runner.Upload(ctx.GoContext(), conn, localCertPath, remoteCertPath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s.crt': %w", baseName, err)
		}
		if err := runner.Upload(ctx.GoContext(), conn, localKeyPath, remoteKeyPath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s.key': %w", baseName, err)
		}
	}

	logger.Info("Successfully distributed renewed Kubernetes leaf certificates and keys to the node.")
	return nil
}

func (s *DistributeLeafCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	backupPath, ok := ctx.GetTaskCache().Get(CacheKeyRemotePKIBackupPath)
	if !ok {
		logger.Error("CRITICAL: No backup path found in cache. CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("no backup path found in cache for host '%s', cannot restore PKI", ctx.GetHost().GetName())
	}
	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		logger.Error("CRITICAL: Invalid backup path in cache. CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("invalid backup path in cache for host '%s'", ctx.GetHost().GetName())
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Cannot connect to host for rollback, manual intervention required. Error: %v", err)
		return err
	}

	logger.Warnf("Rolling back: restoring entire PKI from backup '%s'...", backupDir)

	cleanupCmd := fmt.Sprintf("rm -rf %s", s.remoteKubePkiDir)
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove modified PKI directory during rollback. Manual cleanup may be needed. Error: %v", err)
	}

	restoreCmd := fmt.Sprintf("mv %s %s", backupDir, s.remoteKubePkiDir)
	if _, err := runner.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo); err != nil {
		logger.Errorf("CRITICAL: Failed to restore PKI backup on host '%s'. MANUAL INTERVENTION REQUIRED. Error: %v", ctx.GetHost().GetName(), err)
		return err
	}

	logger.Info("Rollback completed: original PKI directory has been restored.")
	return nil
}

var _ step.Step = (*DistributeLeafCertsStep)(nil)
