package containerd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// VerifyContainerdCrictlStep uses crictl to check containerd's CRI interface.
type VerifyContainerdCrictlStep struct {
	meta             spec.StepMeta
	CrictlPath       string // Path to crictl binary, defaults to "crictl" (expect in PATH)
	RuntimeEndpoint  string // Containerd socket path, e.g., "unix:///run/containerd/containerd.sock"
	Sudo             bool   // If crictl command itself needs sudo (unlikely if socket permissions are correct)
}

// NewVerifyContainerdCrictlStep creates a new VerifyContainerdCrictlStep.
func NewVerifyContainerdCrictlStep(instanceName, crictlPath, runtimeEndpoint string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "VerifyContainerdWithCrictl"
	}
	cp := crictlPath
	if cp == "" {
		cp = "crictl" // Assume in PATH
	}
	ep := runtimeEndpoint
	if ep == "" {
		ep = "unix:///run/containerd/containerd.sock" // Default containerd socket
	}

	return &VerifyContainerdCrictlStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Verifies containerd CRI interface using %s with endpoint %s.", cp, ep),
		},
		CrictlPath:      cp,
		RuntimeEndpoint: ep,
		Sudo:            sudo,
	}
}

func (s *VerifyContainerdCrictlStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *VerifyContainerdCrictlStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	// This step is a verification step, its Precheck could be true if a previous run succeeded
	// and cached a success status, or it could always return false to ensure verification runs.
	// For simplicity, let's always run the verification if this step is scheduled.
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	logger.Info("Verification step will always run if scheduled, precheck returns false.")
	return false, nil
}

func (s *VerifyContainerdCrictlStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Ensure crictl is available
	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, s.CrictlPath); err != nil {
		logger.Error("crictl command not found or not in PATH.", "path_tried", s.CrictlPath, "error", err)
		return fmt.Errorf("crictl command '%s' not found: %w", s.CrictlPath, err)
	}

	// crictl --runtime-endpoint <endpoint> info
	// crictl --runtime-endpoint <endpoint> ps -a (optional, more thorough)
	cmdArgs := []string{s.CrictlPath}
	if s.RuntimeEndpoint != "" {
		cmdArgs = append(cmdArgs, "--runtime-endpoint", s.RuntimeEndpoint)
	}
	cmdArgs = append(cmdArgs, "info")
	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Executing crictl info command.", "command", cmd)
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Error("crictl info command failed.", "command", cmd, "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("crictl info command '%s' failed: %w. Stderr: %s", cmd, err, string(stderr))
	}

	outputStr := string(stdout)
	logger.Info("crictl info command executed successfully.", "output_snippet", firstNLines(outputStr, 5))

	// Basic check on output: look for "runtimeName" or "runtimeVersion" for containerd.
	// A more robust check would parse the JSON output if `crictl info -o json` is used.
	if !strings.Contains(outputStr, "runtimeName") || !strings.Contains(outputStr, "runtimeVersion") {
		logger.Warn("Output of 'crictl info' does not contain expected fields (runtimeName, runtimeVersion).", "output", outputStr)
		// Depending on strictness, this could be an error.
		// For now, a successful command execution is considered a pass for this basic step.
	} else {
		logger.Info("crictl info output contains expected fields, CRI interface appears healthy.")
	}

	return nil
}

func (s *VerifyContainerdCrictlStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for VerifyContainerdCrictlStep is not applicable as it's a verification step.")
	return nil
}

var _ step.Step = (*VerifyContainerdCrictlStep)(nil)

// firstNLines returns the first N lines of a string, or the whole string if fewer than N lines.
func firstNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n..."
}
