package kubeadm

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmRestartEtcdStep struct {
	step.Base
	manifestsDir   string
	tempDir        string
	etcdManifest   string
	postWaitSettle time.Duration
}

type KubeadmRestartEtcdStepBuilder struct {
	step.Builder[KubeadmRestartEtcdStepBuilder, *KubeadmRestartEtcdStep]
}

func NewKubeadmRestartEtcdStepBuilder(ctx runtime.Context, instanceName string) *KubeadmRestartEtcdStepBuilder {
	s := &KubeadmRestartEtcdStep{
		manifestsDir:   common.DefaultEtcdPKIDir,
		tempDir:        "/tmp",
		etcdManifest:   "etcd.yaml",
		postWaitSettle: 20 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart the etcd static pod to apply new certificates"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmRestartEtcdStepBuilder).Init(s)
	return b
}

func (s *KubeadmRestartEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmRestartEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying etcd manifest file exists...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	manifestPath := filepath.Join(s.manifestsDir, s.etcdManifest)
	checkCmd := fmt.Sprintf("[ -f %s ]", manifestPath)

	if _, err := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo); err != nil {
		logger.Warnf("Etcd manifest file '%s' not found. This node might not be a stacked etcd master. Skipping.", manifestPath)
		return true, nil
	}

	logger.Info("Precheck passed: etcd manifest file found.")
	return false, nil
}

func (s *KubeadmRestartEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Starting controlled restart of the etcd static pod...")

	manifestPath := filepath.Join(s.manifestsDir, s.etcdManifest)
	tempPath := filepath.Join(s.tempDir, fmt.Sprintf("%s.%d", s.etcdManifest, time.Now().UnixNano()))

	logger.Info("Temporarily moving etcd manifest to stop the pod...")
	moveOutCmd := fmt.Sprintf("mv %s %s", manifestPath, tempPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveOutCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to move etcd manifest out on host '%s': %w", ctx.GetHost().GetName(), err)
	}

	time.Sleep(5 * time.Second)

	logger.Info("Moving etcd manifest back to restart the pod with new certificates...")
	moveInCmd := fmt.Sprintf("mv %s %s", tempPath, manifestPath)
	if _, err := runner.Run(ctx.GoContext(), conn, moveInCmd, s.Sudo); err != nil {
		logger.Errorf("CRITICAL: Failed to move etcd manifest back! Etcd will not start. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("failed to restore manifest for etcd on host '%s': %w", ctx.GetHost().GetName(), err)
	}

	logger.Infof("Waiting for etcd pod to become healthy...")
	time.Sleep(s.postWaitSettle)

	logger.Info("Etcd static pod has been restarted.")
	return nil
}

func (s *KubeadmRestartEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for this step is complex. Ensuring etcd manifest is in place...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(s.manifestsDir, s.etcdManifest)
	tempPathPattern := filepath.Join(s.tempDir, fmt.Sprintf("%s.*", s.etcdManifest))

	restoreCmd := fmt.Sprintf("find %s -type f -printf '%%T@ %%p\\n' | sort -n | tail -1 | cut -d' ' -f2- | xargs -I {} mv {} %s", tempPathPattern, manifestPath)
	_, _ = runner.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo)

	logger.Info("Rollback attempt for etcd manifest finished.")
	return nil
}

var _ step.Step = (*KubeadmRestartEtcdStep)(nil)
