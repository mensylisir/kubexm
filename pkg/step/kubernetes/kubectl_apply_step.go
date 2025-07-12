package kubernetes

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"time"
)

// KubectlApplyStep executes 'kubectl apply -f' with a given manifest file.
type KubectlApplyStep struct {
	meta             spec.StepMeta
	ManifestPath     string // Path to the manifest file on the target host
	KubeconfigPath   string // Optional: Path to kubeconfig file on the target host
	Sudo             bool   // If kubectl command needs sudo (rare, but possible if kubeconfig is root-only)
	Retries          int    // Number of retries on failure
	RetryDelay       int    // Delay in seconds between retries
}

// NewKubectlApplyStep creates a new KubectlApplyStep.
func NewKubectlApplyStep(instanceName, manifestPath, kubeconfigPath string, sudo bool, retries, retryDelay int) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("KubectlApply-%s", manifestPath)
	}
	return &KubectlApplyStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Applies Kubernetes manifest %s", manifestPath),
		},
		ManifestPath:     manifestPath,
		KubeconfigPath:   kubeconfigPath,
		Sudo:             sudo,
		Retries:          retries,
		RetryDelay:       retryDelay,
	}
}

func (s *KubectlApplyStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *KubectlApplyStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	// Precheck could try to see if resources defined in the manifest already exist and match.
	// This is complex and often not done for a simple apply step.
	// For now, assume the apply is idempotent or the desired state should always be enforced.
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	logger.Info("KubectlApplyStep Precheck: Assuming run is required to ensure desired state.")
	return false, nil
}

func (s *KubectlApplyStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmd := "kubectl apply -f " + s.ManifestPath
	if s.KubeconfigPath != "" {
		cmd += " --kubeconfig " + s.KubeconfigPath
	}

	execOptions := &connector.ExecOptions{
		Sudo: s.Sudo,
		// Retries and RetryDelay can be handled by ExecOptions if it supports them,
		// or implemented in a loop here. For now, assuming ExecOptions handles it.
		// If ExecOptions doesn't support retries, this step would need its own loop.
		// The current connector.ExecOptions has Retries and RetryDelaySeconds.
		Retries:           s.Retries,
		RetryDelaySeconds: s.RetryDelay, // Pass int directly
	}


	logger.Info("Running kubectl apply command", "command", cmd)
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, execOptions)
	if err != nil {
		logger.Error("kubectl apply failed", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("kubectl apply -f %s failed: %w. Stdout: %s, Stderr: %s", s.ManifestPath, err, string(stdout), string(stderr))
	}

	logger.Info("kubectl apply completed successfully.", "manifest", s.ManifestPath, "stdout", string(stdout))
	return nil
}

func (s *KubectlApplyStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	// Rollback for 'kubectl apply' could be 'kubectl delete -f <manifest>', but this can be risky
	// if the manifest created shared resources or if the "previous" state is unknown.
	// For simplicity, often it's a no-op or requires manual intervention.
	logger.Info("Rollback for KubectlApplyStep is typically complex and not automatically performed. Consider 'kubectl delete -f <manifest>' manually if needed.", "manifest", s.ManifestPath)
	return nil
}

var _ step.Step = (*KubectlApplyStep)(nil)

[end of pkg/step/kubernetes/kubectl_apply_step.go]
