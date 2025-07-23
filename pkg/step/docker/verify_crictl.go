package docker

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type VerifyCrictlStep struct {
	step.Base
	CrictlPath      string
	RuntimeEndpoint string
}

type VerifyCrictlStepBuilder struct {
	step.Builder[VerifyCrictlStepBuilder, *VerifyCrictlStep]
}

func NewVerifyCrictlStepBuilder(ctx runtime.Context, instanceName string) *VerifyCrictlStepBuilder {
	s := &VerifyCrictlStep{
		CrictlPath:      "crictl",
		RuntimeEndpoint: "unix:///run/cri-dockerd.sock",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Verify Docker (via cri-dockerd) CRI interface using crictl", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(VerifyCrictlStepBuilder).Init(s)
	return b
}

func (b *VerifyCrictlStepBuilder) WithCrictlPath(path string) *VerifyCrictlStepBuilder {
	b.Step.CrictlPath = path
	return b
}

func (b *VerifyCrictlStepBuilder) WithRuntimeEndpoint(endpoint string) *VerifyCrictlStepBuilder {
	b.Step.RuntimeEndpoint = endpoint
	return b
}

func (s *VerifyCrictlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *VerifyCrictlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *VerifyCrictlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cmdArgs := []string{s.CrictlPath}
	if s.RuntimeEndpoint != "" {
		cmdArgs = append(cmdArgs, "--runtime-endpoint", s.RuntimeEndpoint)
	}
	cmdArgs = append(cmdArgs, "info")
	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Executing crictl info command for cri-dockerd.", "command", cmd)
	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		logger.Error(err, "crictl info command (for cri-dockerd) failed.", "command", cmd, "stdout", stdout, "stderr", stderr)
		return fmt.Errorf("crictl info command '%s' failed: %w. Stderr: %s", cmd, err, stderr)
	}

	outputStr := stdout
	logger.Info("crictl info (for cri-dockerd) command executed successfully.", "output_snippet", firstNLines(outputStr, 10))

	if !strings.Contains(strings.ToLower(outputStr), "docker") || !strings.Contains(outputStr, "ServerVersion") {
		logger.Warn("Output of 'crictl info' (for cri-dockerd) does not clearly indicate Docker information.", "output", outputStr)
	} else {
		logger.Info("crictl info output (for cri-dockerd) contains Docker information, CRI interface appears healthy.")
	}

	return nil
}

func (s *VerifyCrictlStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

func firstNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n..."
}
