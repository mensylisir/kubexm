package kubernetes

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// APIServerReadyStep waits for Kubernetes APIServer to become ready.
type APIServerReadyStep struct {
	meta     spec.StepMeta
	Endpoint string        // APIServer endpoint (e.g., "https://127.0.0.1:6443")
	Timeout  time.Duration // How long to wait
}

// NewAPIServerReadyStep creates a new APIServerReadyStep.
func NewAPIServerReadyStep(instanceName, endpoint string, timeout time.Duration) step.Step {
	name := instanceName
	if name == "" {
		name = "APIServerReady"
	}
	return &APIServerReadyStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Waits for Kubernetes APIServer at %s to become ready", endpoint),
		},
		Endpoint: endpoint,
		Timeout:  timeout,
	}
}

func (s *APIServerReadyStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Precheck checks if the APIServer is already ready.
func (s *APIServerReadyStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName(), "phase", "Precheck")

	ready, err := s.checkAPIServerHealth(ctx, host, 10*time.Second)
	if err != nil {
		logger.Debug("APIServer health check failed", "error", err)
		return false, nil
	}

	if ready {
		logger.Debug("APIServer is already ready")
		return true, nil
	}

	logger.Debug("APIServer is not yet ready")
	return false, nil
}

// Run waits for the APIServer to become ready.
func (s *APIServerReadyStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName(), "phase", "Run")

	logger.Info("Waiting for APIServer to become ready", "endpoint", s.Endpoint, "timeout", s.Timeout)

	ready, err := s.checkAPIServerHealth(ctx, host, s.Timeout)
	if err != nil {
		logger.Error("APIServer health check failed", "error", err)
		return fmt.Errorf("APIServer at %s failed health check: %w", s.Endpoint, err)
	}

	if !ready {
		return fmt.Errorf("APIServer at %s did not become ready within timeout %v", s.Endpoint, s.Timeout)
	}

	logger.Info("APIServer is now ready", "endpoint", s.Endpoint)
	return nil
}

// checkAPIServerHealth checks if the APIServer is healthy by calling /healthz or /readyz
func (s *APIServerReadyStep) checkAPIServerHealth(ctx step.StepContext, host connector.Host, timeout time.Duration) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Use kubectl command to check API server health
	// This is more reliable than direct HTTP calls as it handles certs properly
	cmd := "kubectl get --raw='/healthz' --kubeconfig=/etc/kubernetes/admin.conf"
	
	start := time.Now()
	for time.Since(start) < timeout {
		_, _, err := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{
			Sudo:    false,
			Timeout: 10 * time.Second,
		})
		
		if err == nil {
			return true, nil
		}
		
		// Wait before retrying
		time.Sleep(5 * time.Second)
	}

	return false, nil
}

// Rollback is a no-op for APIServerReadyStep.
func (s *APIServerReadyStep) Rollback(ctx step.StepContext, host connector.Host) error {
	return nil
}

var _ step.Step = (*APIServerReadyStep)(nil)