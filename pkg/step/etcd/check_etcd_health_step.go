package etcd

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	// Assuming step.StepContext will be used, which step.StepContext is an interface for.
)

// CheckEtcdHealthStep checks the health of an etcd endpoint or cluster.
type CheckEtcdHealthStep struct {
	meta        spec.StepMeta
	Endpoints   string // Comma-separated list of etcd endpoints. Defaults to local if empty.
	CACertPath  string
	CertPath    string
	KeyPath     string
	EtcdctlPath string
	Timeout     time.Duration // Timeout for the health check command
	Retries     int           // Number of retries for the health check
	RetryDelay  time.Duration // Delay between retries
	Sudo        bool
}

// NewCheckEtcdHealthStep creates a new CheckEtcdHealthStep.
func NewCheckEtcdHealthStep(instanceName, endpoints, caPath, certPath, keyPath, etcdctlPath string, timeout time.Duration, retries int, retryDelay time.Duration, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "CheckEtcdHealth"
	}
	ep := endpoints
	if ep == "" {
		ep = "https://127.0.0.1:2379"
	}
	ctlPath := etcdctlPath
	if ctlPath == "" {
		ctlPath = "etcdctl"
	}
	to := timeout
	if to <= 0 {
		to = 5 * time.Second // Default timeout for a single check attempt
	}
	r := retries
	if r < 0 {
		r = 0 // No retries if negative
	}
	rd := retryDelay
	if rd <= 0 {
		rd = 2 * time.Second // Default delay between retries
	}

	return &CheckEtcdHealthStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Checks etcd health for endpoints: %s", ep),
		},
		Endpoints:   ep,
		CACertPath:  caPath,
		CertPath:    certPath,
		KeyPath:     keyPath,
		EtcdctlPath: ctlPath,
		Timeout:     to,
		Retries:     r,
		RetryDelay:  rd,
		Sudo:        sudo,
	}
}

func (s *CheckEtcdHealthStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CheckEtcdHealthStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	// Precheck for health check could be to see if etcdctl exists.
	// The health check itself is the primary action.
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("Precheck: failed to get connector for host %s: %w", host.GetName(), err)
	}
	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, s.EtcdctlPath); err != nil {
		logger.Error("etcdctl command not found.", "path_tried", s.EtcdctlPath, "error", err)
		return false, fmt.Errorf("etcdctl command '%s' not found for health check: %w", s.EtcdctlPath, err)
	}
	logger.Info("etcdctl found. Health check will proceed in Run phase.")
	return false, nil // Always run the health check when this step is scheduled.
}

func (s *CheckEtcdHealthStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmdArgs := []string{
		"ETCDCTL_API=3",
		s.EtcdctlPath,
		"endpoint", "health", // Could also use "cluster-health" for overall, "endpoint health" for specific
		"--endpoints=" + s.Endpoints,
		// The default output for 'endpoint health' is simple, one line per endpoint:
		// <endpoint_url>, unhealthy, <error_msg>
		// <endpoint_url>, healthy, <other_info_json>
		// We want to ensure all listed endpoints are healthy.
		// Adding -w table or -w json might make parsing easier if needed, but default is fine for checking health.
		// For "cluster-health": "cluster is healthy" or "cluster is unhealthy"
	}
	if s.CACertPath != "" {
		cmdArgs = append(cmdArgs, "--cacert="+s.CACertPath)
	}
	if s.CertPath != "" {
		cmdArgs = append(cmdArgs, "--cert="+s.CertPath)
	}
	if s.KeyPath != "" {
		cmdArgs = append(cmdArgs, "--key="+s.KeyPath)
	}
	cmd := strings.Join(cmdArgs, " ")

	var lastErr error
	for i := 0; i <= s.Retries; i++ {
		logger.Info("Executing etcd health check command.", "command", cmd, "attempt", i+1)
		execCtx, cancel := context.WithTimeout(ctx.GoContext(), s.Timeout)
		defer cancel()

		stdout, stderr, runErr := runnerSvc.RunWithOptions(execCtx, conn, cmd, &connector.ExecOptions{Sudo: s.Sudo, Check: true})

		if runErr == nil { // Exit code 0
			output := string(stdout)
			// "endpoint health" output:
			// 127.0.0.1:2379 is healthy: successfully committed proposal: took = ...
			// For multiple endpoints, it lists each. All must be healthy.
			// "cluster-health" output:
			// cluster is healthy
			// member ... has no leader
			// member ... is healthy
			// ...
			// For simplicity, we check if "unhealthy" is NOT in stdout for endpoint health.
			// And for cluster health, if "cluster is healthy" is present.
			// A more robust check would parse each line for "endpoint health".
			// For now, a simple check: if the command exits 0 and doesn't say "unhealthy", it's good.
			// This might need refinement based on actual etcdctl output versions.
			// `etcdctl endpoint health --cluster` gives overall health too.
			// `etcdctl endpoint health --endpoints <csv_list>` checks each one.
			// If any endpoint in the list is unhealthy, `etcdctl endpoint health` (without --cluster) will exit non-zero.
			// So, exit code 0 from `etcdctl endpoint health --endpoints ...` means all specified endpoints are healthy.
			logger.Info("Etcd health check command successful (exit 0).", "stdout", string(stdout))
			return nil // Healthy
		}

		lastErr = fmt.Errorf("etcd health check command '%s' attempt %d failed: %w. Stdout: %s, Stderr: %s", cmd, i+1, runErr, string(stdout), string(stderr))
		logger.Warn("Etcd health check attempt failed.", "error", lastErr)

		if i < s.Retries {
			logger.Info("Retrying etcd health check after delay.", "delay", s.RetryDelay)
			select {
			case <-time.After(s.RetryDelay):
			case <-ctx.GoContext().Done(): // If overall context is cancelled
				logger.Info("Context cancelled during retry delay.")
				return ctx.GoContext().Err()
			}
		}
	}

	logger.Error("Etcd health check failed after all retries.", "final_error", lastErr)
	return lastErr
}

func (s *CheckEtcdHealthStep) Rollback(ctx step.StepContext, host connector.Host) error {
	// No rollback action for a health check.
	return nil
}

var _ step.Step = (*CheckEtcdHealthStep)(nil)
```
