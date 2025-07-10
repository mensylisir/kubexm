package os

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ConfigureSELinuxStep sets the SELinux mode (e.g., "permissive", "enforcing", "disabled").
type ConfigureSELinuxStep struct {
	meta spec.StepMeta
	Mode string // "permissive", "enforcing", "disabled"
	Sudo bool
}

// NewConfigureSELinuxStep creates a new ConfigureSELinuxStep.
func NewConfigureSELinuxStep(instanceName, mode string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("ConfigureSELinux-%s", mode)
	}
	return &ConfigureSELinuxStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Configures SELinux to %s mode.", mode),
		},
		Mode: strings.ToLower(mode),
		Sudo: sudo,
	}
}

func (s *ConfigureSELinuxStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ConfigureSELinuxStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, err
	}

	// Check current SELinux status using getenforce
	// Sudo might be needed for setenforce, but usually not for getenforce.
	stdout, _, err := runnerSvc.Run(ctx.GoContext(), conn, "getenforce", false)
	if err != nil {
		// If getenforce is not found, SELinux might not be installed/supported.
		// Consider this as "cannot determine" or "not applicable".
		logger.Warn("getenforce command failed or SELinux not available. Assuming precheck fails.", "error", err)
		return false, nil // Let Run attempt to configure.
	}

	currentMode := strings.ToLower(strings.TrimSpace(string(stdout)))
	logger.Info("Current SELinux mode.", "mode", currentMode)

	if currentMode == s.Mode {
		// TODO: Also check persistent config in /etc/selinux/config
		logger.Info("SELinux is already in the desired mode.", "mode", s.Mode)
		return true, nil
	}

	logger.Info("SELinux not in desired mode.", "current", currentMode, "desired", s.Mode)
	return false, nil
}

func (s *ConfigureSELinuxStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return err
	}

	// Set runtime mode
	if s.Mode == "disabled" { // setenforce 0 means permissive
		logger.Info("Setting SELinux to Permissive mode for runtime (setenforce 0) as 'disabled' requires reboot.")
		if _, _, err := runnerSvc.Run(ctx.GoContext(), conn, "setenforce 0", s.Sudo); err != nil {
			logger.Warn("Failed to set SELinux to permissive at runtime. Configuration file change will still be attempted.", "error", err)
			// Don't return error, as persistent change is more important.
		}
	} else if s.Mode == "permissive" || s.Mode == "enforcing" {
		cmd := fmt.Sprintf("setenforce %s", s.Mode)
		if s.Mode == "enforcing" { cmd = "setenforce 1" }
		if s.Mode == "permissive" { cmd = "setenforce 0" }

		logger.Info("Setting runtime SELinux mode.", "command", cmd)
		if _, _, err := runnerSvc.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to execute '%s': %w", cmd, err)
		}
	} else {
		return fmt.Errorf("invalid SELinux mode specified: %s", s.Mode)
	}

	// Set persistent mode in /etc/selinux/config or /etc/sysconfig/selinux
	// This is OS dependent. Using a common sed command for /etc/selinux/config
	// File path might be /etc/sysconfig/selinux on RHEL/CentOS < 8 or /etc/selinux/config
	// For simplicity, targeting /etc/selinux/config
	selinuxConfigFile := "/etc/selinux/config"
	logger.Info("Attempting to set persistent SELinux mode.", "file", selinuxConfigFile, "mode", s.Mode)

	// Ensure the file exists before trying to sed it, or handle sed failure if file missing.
	// sed -i 's/^SELINUX=.*/SELINUX=permissive/' /etc/selinux/config
	sedCmd := fmt.Sprintf("sed -i 's/^SELINUX=.*/SELINUX=%s/' %s", s.Mode, selinuxConfigFile)
	_, stderr, errSed := runnerSvc.Run(ctx.GoContext(), conn, sedCmd, s.Sudo)
	if errSed != nil {
		// If sed fails, it could be because the file doesn't exist or pattern not found.
		// This might not be a fatal error if setenforce worked, but persistence is desired.
		logger.Warn("Failed to set SELinux mode in config file using sed. Manual check might be needed for persistence.", "file", selinuxConfigFile, "command", sedCmd, "error", errSed, "stderr", string(stderr))
		// Consider this a best-effort for now. A reboot is often needed for 'disabled'.
	} else {
		logger.Info("Persistent SELinux mode updated in config file.", "file", selinuxConfigFile, "mode", s.Mode)
	}
	if s.Mode == "disabled" {
		logger.Warn("SELinux set to 'disabled' in config. A reboot is required for this change to take full effect.")
	}

	return nil
}

func (s *ConfigureSELinuxStep) Rollback(ctx step.StepContext, host connector.Host) error {
	// Rollback is complex: would need to know previous runtime and persistent state.
	ctx.GetLogger().Warn("Rollback for ConfigureSELinuxStep is not implemented.", "step", s.meta.Name)
	return nil
}

var _ step.Step = (*ConfigureSELinuxStep)(nil)
```
