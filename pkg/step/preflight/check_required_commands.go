package preflight

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CheckRequiredCommandsStep struct {
	step.Base
	Commands []string
}

type CheckRequiredCommandsStepBuilder struct {
	step.Builder[CheckRequiredCommandsStepBuilder, *CheckRequiredCommandsStep]
}

func NewCheckRequiredCommandsStepBuilder(ctx runtime.Context, instanceName string) *CheckRequiredCommandsStepBuilder {
	s := &CheckRequiredCommandsStep{
		Commands: []string{
			"systemctl",
			"curl",
			"ss",
			"nc",
			"dig",
			"nslookup",
			"ntpstat",
			"chronyc",
			"kubectl",
			"etcdctl",
			"rm",
			"mv",
			"cp",
			"mkdir",
		},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check if all required command-line tools are installed on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(CheckRequiredCommandsStepBuilder).Init(s)
	return b
}

func (s *CheckRequiredCommandsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckRequiredCommandsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for required commands check...")
	logger.Info("Precheck passed: Required commands check will always be attempted.")
	return false, nil
}

func (s *CheckRequiredCommandsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Checking for existence of all required commands...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	checkCmd := ""
	for i, cmd := range s.Commands {
		checkCmd += fmt.Sprintf("command -v %s", cmd)
		if i < len(s.Commands)-1 {
			checkCmd += " >/dev/null && "
		} else {
			checkCmd += " >/dev/null"
		}
	}

	logger.Debugf("Executing command: %s", checkCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo); err != nil {
		var missingCmds []string
		for _, cmd := range s.Commands {
			individualCheckCmd := fmt.Sprintf("command -v %s", cmd)
			if _, individualErr := runner.Run(ctx.GoContext(), conn, individualCheckCmd, s.Sudo); individualErr != nil {
				missingCmds = append(missingCmds, cmd)
			}
		}
		if len(missingCmds) > 0 {
			return fmt.Errorf("one or more required commands are missing on host %s: %s", ctx.GetHost().GetName(), strings.Join(missingCmds, ", "))
		}
		return fmt.Errorf("failed to verify required commands on host %s, although individual checks seem to pass: %w", ctx.GetHost().GetName(), err)
	}

	logger.Info("All required commands are available on the node.")
	return nil
}

func (s *CheckRequiredCommandsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a check-only step.")
	return nil
}

var _ step.Step = (*CheckRequiredCommandsStep)(nil)
