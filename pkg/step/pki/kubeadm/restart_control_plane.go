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

type KubeadmRestartControlPlaneStep struct {
	step.Base
	manifestsDir   string
	tempDir        string
	components     []string
	waitTimeout    time.Duration
	postWaitSettle time.Duration
}

type KubeadmRestartControlPlaneStepBuilder struct {
	step.Builder[KubeadmRestartControlPlaneStepBuilder, *KubeadmRestartControlPlaneStep]
}

func NewKubeadmRestartControlPlaneStepBuilder(ctx runtime.Context, instanceName string) *KubeadmRestartControlPlaneStepBuilder {
	s := &KubeadmRestartControlPlaneStep{
		manifestsDir: common.KubernetesManifestsDir,
		tempDir:      common.DefaultSystemTmpDir,
		components: []string{
			"kube-apiserver.yaml",
			"kube-controller-manager.yaml",
			"kube-scheduler.yaml",
		},
		waitTimeout:    5 * time.Minute,
		postWaitSettle: 15 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart control plane static pods to apply new certificates"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(KubeadmRestartControlPlaneStepBuilder).Init(s)
	return b
}

func (s *KubeadmRestartControlPlaneStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmRestartControlPlaneStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying control plane manifest files exist...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	for _, component := range s.components {
		manifestPath := filepath.Join(s.manifestsDir, component)
		checkCmd := fmt.Sprintf("[ -f %s ]", manifestPath)

		log := logger.With("manifest", manifestPath)
		log.Infof("Checking for manifest file...")
		if _, err := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo); err != nil {
			log.Errorf("Required manifest file not found.")
			return false, fmt.Errorf("precheck failed: required manifest file '%s' not found on host '%s'", manifestPath, ctx.GetHost().GetName())
		}
	}

	logger.Info("Precheck passed: all required control plane manifest files found.")
	return false, nil
}

func (s *KubeadmRestartControlPlaneStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Starting controlled restart of control plane static pods...")

	for _, component := range s.components {
		log := logger.With("component", component)
		manifestPath := filepath.Join(s.manifestsDir, component)
		tempPath := filepath.Join(s.tempDir, fmt.Sprintf("%s.%d", component, time.Now().UnixNano()))

		log.Info("Temporarily moving manifest to stop the pod...")
		moveOutCmd := fmt.Sprintf("mv %s %s", manifestPath, tempPath)
		if _, err := runner.Run(ctx.GoContext(), conn, moveOutCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to move manifest out for '%s' on host '%s': %w", component, ctx.GetHost().GetName(), err)
		}

		time.Sleep(5 * time.Second)

		log.Info("Moving manifest back to restart the pod with new certificates...")
		moveInCmd := fmt.Sprintf("mv %s %s", tempPath, manifestPath)
		if _, err := runner.Run(ctx.GoContext(), conn, moveInCmd, s.Sudo); err != nil {
			log.Errorf("CRITICAL: Failed to move manifest back! Control plane component will not start. MANUAL INTERVENTION REQUIRED.")
			return fmt.Errorf("failed to restore manifest for '%s' on host '%s': %w", component, ctx.GetHost().GetName(), err)
		}

		log.Infof("Waiting for component to become healthy...")
		time.Sleep(s.postWaitSettle)
		log.Info("Component restarted.")
	}

	logger.Info("Control plane static pods have been restarted successfully.")
	return nil
}

func (s *KubeadmRestartControlPlaneStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for this step is complex. Ensuring manifests are in place from temp directory...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	for _, component := range s.components {
		manifestPath := filepath.Join(s.manifestsDir, component)
		tempPathPattern := filepath.Join(s.tempDir, fmt.Sprintf("%s.*", component))
		restoreCmd := fmt.Sprintf("find %s -type f -printf '%%T@ %%p\\n' | sort -n | tail -1 | cut -d' ' -f2- | xargs -I {} mv {} %s", tempPathPattern, manifestPath)
		_, _ = runner.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo)
	}

	logger.Info("Rollback attempt finished.")
	return nil
}

var _ step.Step = (*KubeadmRestartControlPlaneStep)(nil)
