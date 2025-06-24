package kubernetes

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	KubeadmInitOutputCacheKey      = "KubeadmInitOutput"
	KubeadmJoinCommandCacheKey     = "KubeadmJoinCommand"
	KubeadmTokenCacheKey           = "KubeadmToken"
	KubeadmCertificateKeyCacheKey  = "KubeadmCertificateKey" // For control-plane join
	KubeadmDiscoveryHashCacheKey = "KubeadmDiscoveryHash"  // For worker join
)

// KubeadmInitStep executes 'kubeadm init' and captures its output.
type KubeadmInitStep struct {
	meta                spec.StepMeta
	ConfigPathOnHost    string // Path to the kubeadm config file on the target host
	Sudo                bool
	IgnorePreflightErrors string // Comma-separated list of preflight errors to ignore, or "all"
}

// NewKubeadmInitStep creates a new KubeadmInitStep.
func NewKubeadmInitStep(instanceName, configPathOnHost, ignorePreflightErrors string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "KubeadmInit"
	}
	return &KubeadmInitStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Initializes Kubernetes control plane using kubeadm with config %s", configPathOnHost),
		},
		ConfigPathOnHost:    configPathOnHost,
		Sudo:                sudo,
		IgnorePreflightErrors: ignorePreflightErrors,
	}
}

func (s *KubeadmInitStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *KubeadmInitStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	// Kubeadm init is generally not idempotent in a simple way by checking file existence.
	// It might be considered "done" if the control plane is already up and running.
	// This would involve more complex checks (e.g., API server health, etcd health).
	// For now, assume it always needs to run if scheduled, or a more specific pre-task handles this.
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	logger.Info("KubeadmInitStep Precheck: Assuming run is required if this step is reached.")
	return false, nil
}

func (s *KubeadmInitStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmd := fmt.Sprintf("kubeadm init --config %s --upload-certs", s.ConfigPathOnHost)
	if s.IgnorePreflightErrors != "" {
		cmd += fmt.Sprintf(" --ignore-preflight-errors=%s", s.IgnorePreflightErrors)
	}

	logger.Info("Running kubeadm init command", "command", cmd)
	// Kubeadm init can take a while. A timeout might be needed from ExecOptions.
	// For now, using default timeout from runner.
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})

	// Store full output in cache for debugging or other purposes
	ctx.TaskCache().Set(KubeadmInitOutputCacheKey, string(stdout)+"\n"+string(stderr))

	if err != nil {
		logger.Error("kubeadm init failed", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("kubeadm init failed: %w. Stdout: %s, Stderr: %s", err, string(stdout), string(stderr))
	}

	logger.Info("kubeadm init completed successfully.", "stdout", string(stdout))

	// TODO: Parse stdout for join command, token, cert hash and store them in TaskCache
	// Example (simplified parsing):
	// outputStr := string(stdout)
	// if joinCmdLine := findJoinCommand(outputStr); joinCmdLine != "" {
	//    ctx.TaskCache().Set(KubeadmJoinCommandCacheKey, joinCmdLine)
	//    logger.Info("Found and cached kubeadm join command.")
	// }
	// Similar for token, cert key, discovery hash.

	return nil
}

func (s *KubeadmInitStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback", "error", err)
		return nil // Best effort
	}

	cmd := "kubeadm reset -f" // Force reset
	logger.Info("Attempting kubeadm reset for rollback", "command", cmd)
	_, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Warn("kubeadm reset command failed during rollback (best effort).", "error", err, "stderr", string(stderr))
	} else {
		logger.Info("kubeadm reset executed successfully for rollback.")
	}
	return nil
}

var _ step.Step = (*KubeadmInitStep)(nil)

[end of pkg/step/kubernetes/kubeadm_init_step.go]
