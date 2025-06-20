package kubeadm

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For IsExitCodeIgnored
)

// RunKubeadmStepSpec defines parameters for executing a generic kubeadm command.
type RunKubeadmStepSpec struct {
	spec.StepMeta `json:",inline"`

	SubCommand      string   `json:"subCommand,omitempty"`
	SubCommandArgs  []string `json:"subCommandArgs,omitempty"`
	GlobalArgs      []string `json:"globalArgs,omitempty"`
	Sudo            bool     `json:"sudo,omitempty"`
	IgnoreExitCodes []int    `json:"ignoreExitCodes,omitempty"`
}

// NewRunKubeadmStepSpec creates a new RunKubeadmStepSpec.
func NewRunKubeadmStepSpec(name, description, subCommand string, subCommandArgs []string, globalArgs []string) *RunKubeadmStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Run kubeadm %s", subCommand)
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &RunKubeadmStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		SubCommand:     subCommand,
		SubCommandArgs: subCommandArgs,
		GlobalArgs:     globalArgs,
	}
}

// Name returns the step's name.
func (s *RunKubeadmStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *RunKubeadmStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *RunKubeadmStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *RunKubeadmStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *RunKubeadmStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *RunKubeadmStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *RunKubeadmStepSpec) populateDefaults(logger runtime.Logger) {
	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Runs kubeadm %s command with arguments: global [%s], sub-command [%s]",
			s.SubCommand, strings.Join(s.GlobalArgs, " "), strings.Join(s.SubCommandArgs, " "))
	}
	// Sudo often defaults to true for kubeadm commands.
	// However, making it explicit by the caller is safer.
	// if !s.Sudo {
	//    s.Sudo = true
	//    logger.Debug("Sudo defaulted to true for kubeadm command.")
	// }
}

// Precheck ensures kubeadm is available.
func (s *RunKubeadmStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.SubCommand == "" {
		return false, fmt.Errorf("SubCommand must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, err := conn.LookPath(ctx.GoContext(), "kubeadm"); err != nil {
		return false, fmt.Errorf("kubeadm command not found on host %s: %w", host.GetName(), err)
	}
	logger.Debug("kubeadm command found on host.")

	// Generic kubeadm commands are usually not idempotent without specific checks related to their subcommands.
	// This precheck primarily ensures the tool exists.
	return false, nil
}

// Run executes the kubeadm command.
func (s *RunKubeadmStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.SubCommand == "" {
		return fmt.Errorf("SubCommand must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmdParts := []string{}
	if s.Sudo {
		cmdParts = append(cmdParts, "sudo")
	}
	cmdParts = append(cmdParts, "kubeadm")
	if len(s.GlobalArgs) > 0 {
		cmdParts = append(cmdParts, s.GlobalArgs...)
	}
	cmdParts = append(cmdParts, s.SubCommand)
	if len(s.SubCommandArgs) > 0 {
		cmdParts = append(cmdParts, s.SubCommandArgs...)
	}
	cmd := strings.Join(cmdParts, " ")

	logger.Info("Executing kubeadm command.", "command", cmd)
	execOpts := &connector.ExecOptions{Sudo: false} // Sudo is already part of the command if s.Sudo is true

	stdout, stderr, err := conn.Exec(ctx.GoContext(), cmd, execOpts)
	if err != nil {
		if utils.IsExitCodeIgnored(err, s.IgnoreExitCodes) {
			logger.Warn("kubeadm command exited with an ignored error code.",
				"command", cmd, "exitLog", err, "stdout", string(stdout), "stderr", string(stderr))
			return nil // Treat as success
		}
		return fmt.Errorf("failed to execute kubeadm command '%s' (stdout: %s, stderr: %s): %w",
			cmd, string(stdout), string(stderr), err)
	}

	// Stash stdout/stderr if needed, e.g., for token creation to capture the token
	// This can be done by specific task specs that use this generic RunKubeadmStepSpec.
	// Example: ctx.StepCache().Set(s.GetName()+"#stdout", string(stdout))

	logger.Info("kubeadm command executed successfully.", "command", cmd, "stdout", string(stdout))
	return nil
}

// Rollback for a generic kubeadm command is not supported.
func (s *RunKubeadmStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for a generic kubeadm command is not supported by this step. Specific rollback logic (e.g., 'kubeadm reset') should be implemented as a separate RunKubeadmStepSpec instance if needed.")
	return nil
}

var _ step.Step = (*RunKubeadmStepSpec)(nil)
