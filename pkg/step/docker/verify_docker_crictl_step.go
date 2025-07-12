package docker

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// VerifyDockerCrictlStep uses crictl to check cri-dockerd's CRI interface.
type VerifyDockerCrictlStep struct {
	meta             spec.StepMeta
	CrictlPath       string
	RuntimeEndpoint  string // cri-dockerd socket path, e.g., "unix:///run/cri-dockerd.sock"
	Sudo             bool
}

// NewVerifyDockerCrictlStep creates a new VerifyDockerCrictlStep.
func NewVerifyDockerCrictlStep(instanceName, crictlPath, runtimeEndpoint string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "VerifyDockerWithCrictl"
	}
	cp := crictlPath
	if cp == "" {
		cp = "crictl"
	}
	ep := runtimeEndpoint
	if ep == "" {
		ep = "unix:///run/cri-dockerd.sock" // Default cri-dockerd socket
	}

	return &VerifyDockerCrictlStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Verifies Docker (via cri-dockerd) CRI interface using %s with endpoint %s.", cp, ep),
		},
		CrictlPath:      cp,
		RuntimeEndpoint: ep,
		Sudo:            sudo,
	}
}

func (s *VerifyDockerCrictlStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *VerifyDockerCrictlStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	logger.Info("Verification step will always run if scheduled, precheck returns false.")
	return false, nil
}

func (s *VerifyDockerCrictlStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, s.CrictlPath); err != nil {
		logger.Error("crictl command not found or not in PATH.", "path_tried", s.CrictlPath, "error", err)
		return fmt.Errorf("crictl command '%s' not found: %w", s.CrictlPath, err)
	}

	cmdArgs := []string{s.CrictlPath}
	if s.RuntimeEndpoint != "" {
		cmdArgs = append(cmdArgs, "--runtime-endpoint", s.RuntimeEndpoint)
	}
	cmdArgs = append(cmdArgs, "info")
	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Executing crictl info command for cri-dockerd.", "command", cmd)
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Error("crictl info command (for cri-dockerd) failed.", "command", cmd, "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("crictl info command '%s' failed: %w. Stderr: %s", cmd, err, string(stderr))
	}

	outputStr := string(stdout)
	logger.Info("crictl info (for cri-dockerd) command executed successfully.", "output_snippet", firstNLines(outputStr, 10)) // Show more lines for Docker

	// Check for Docker version information in the output.
	// `crictl info` when pointed to cri-dockerd should show Docker server version.
	if !strings.Contains(strings.ToLower(outputStr), "docker") || !strings.Contains(outputStr, "ServerVersion") {
		logger.Warn("Output of 'crictl info' (for cri-dockerd) does not clearly indicate Docker information.", "output", outputStr)
	} else {
		logger.Info("crictl info output (for cri-dockerd) contains Docker information, CRI interface appears healthy.")
	}

	return nil
}

func (s *VerifyDockerCrictlStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for VerifyDockerCrictlStep is not applicable.")
	return nil
}

var _ step.Step = (*VerifyDockerCrictlStep)(nil)

// firstNLines helper (can be moved to a common util if used elsewhere)
func firstNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n..."
}
