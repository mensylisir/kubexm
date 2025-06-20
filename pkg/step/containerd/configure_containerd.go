package containerd

import (
	"bytes" // For template rendering
	"fmt"
	"path/filepath"
	"strings"
	"text/template" // For template rendering

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	// No longer need spec or time directly
)

const DefaultContainerdConfigPath = "/etc/containerd/config.toml"

// containerdConfigTemplate is the Go template for generating config.toml.
// Note: The template provided in the prompt has some subtle differences from typical containerd configs,
// especially around how mirrors and insecure registries are deeply nested.
// This template is used as-is from the prompt.
const containerdConfigTemplate = `
version = 2
# Ensure CRI plugin is enabled for Kubernetes. Default is often enabled.
# If you need to ensure it's not in disabled_plugins:
# disabled_plugins = [] # Or ensure "io.containerd.grpc.v1.cri" is not listed if others are disabled.

[plugins."io.containerd.grpc.v1.cri"]
  [plugins."io.containerd.grpc.v1.cri".containerd]
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
        BinaryName = "runc"
        {{if .UseSystemdCgroup}}
        SystemdCgroup = true
        {{else}}
        SystemdCgroup = false
        {{end}}
  # Registry mirrors configuration
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "" # Set to a directory like "/etc/containerd/certs.d" if using individual host certs/configs.
    {{if .RegistryMirrors}}
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
      {{range $registry, $mirror := .RegistryMirrors}}
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{$registry}}"]
        endpoint = ["{{$mirror}}"]
      {{end}}
    {{end}}
    # Insecure registries configuration
    {{if .InsecureRegistries}}
    [plugins."io.containerd.grpc.v1.cri".registry.configs]
      {{range .InsecureRegistries}}
      [plugins."io.containerd.grpc.v1.cri".registry.configs."{{.}}".tls] # {{.}} is the insecure registry hostname:port
        insecure_skip_verify = true # For HTTP or self-signed HTTPS
      {{end}}
    {{end}}

{{if .ExtraTomlContent}}
# Extra TOML content provided by user
{{.ExtraTomlContent}}
{{end}}
`

// ConfigureContainerdStep configures containerd by writing its config.toml.
type ConfigureContainerdStep struct {
	RegistryMirrors    map[string]string
	InsecureRegistries []string
	ConfigFilePath     string
	ExtraTomlContent   string
	UseSystemdCgroup   bool
	StepName           string
}

// NewConfigureContainerdStep creates a new ConfigureContainerdStep.
func NewConfigureContainerdStep(
	registryMirrors map[string]string,
	insecureRegistries []string,
	configFilePath string,
	extraTomlContent string,
	useSystemdCgroup bool,
	stepName string,
) step.Step {
	effectivePath := configFilePath
	if effectivePath == "" {
		effectivePath = DefaultContainerdConfigPath
	}
	return &ConfigureContainerdStep{
		RegistryMirrors:    registryMirrors,
		InsecureRegistries: insecureRegistries,
		ConfigFilePath:     effectivePath,
		ExtraTomlContent:   extraTomlContent,
		UseSystemdCgroup:   useSystemdCgroup,
		StepName:           stepName,
	}
}

func (s *ConfigureContainerdStep) Name() string {
	if s.StepName != "" {
		return s.StepName
	}
	return "Configure containerd (config.toml)"
}

func (s *ConfigureContainerdStep) Description() string {
	return fmt.Sprintf("Configures containerd by writing %s with specified mirrors, insecure registries, and cgroup settings.", s.ConfigFilePath)
}

func (s *ConfigureContainerdStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.ConfigFilePath)
	if err != nil {
		// If error checking existence, assume not configured and let Run attempt.
		logger.Warn("Failed to check existence of config file. Assuming not configured.", "path", s.ConfigFilePath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Debug("Containerd config file does not exist.", "path", s.ConfigFilePath)
		return false, nil
	}

	// If no specific content to check, and file exists, assume done.
	// This means an empty spec (besides path) will consider an existing file as "done".
	if len(s.RegistryMirrors) == 0 && s.ExtraTomlContent == "" && !s.UseSystemdCgroup && len(s.InsecureRegistries) == 0 {
		logger.Info("No specific containerd configurations in spec to check beyond existence.", "path", s.ConfigFilePath)
		return true, nil
	}

	contentBytes, err := conn.ReadFile(ctx.GoContext(), s.ConfigFilePath)
	if err != nil {
		logger.Warn("Failed to read config for checking current configuration. Assuming not configured.", "path", s.ConfigFilePath, "error", err)
		return false, nil
	}
	content := string(contentBytes)

	allChecksPass := true
	for registry, mirror := range s.RegistryMirrors {
		// Basic check, assumes mirror endpoint is unique enough. TOML parsing would be more robust.
		mirrorEndpoint := fmt.Sprintf(`endpoint = ["%s"]`, mirror)
		mirrorBlock := fmt.Sprintf("[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"%s\"]", registry)

		idx := strings.Index(content, mirrorBlock)
		if idx == -1 { allChecksPass = false; break }

		scope := content[idx+len(mirrorBlock):] // Start search after the block header
		nextBlockStart := strings.Index(scope, "\n[")
		if nextBlockStart != -1 {
			scope = scope[:nextBlockStart]
		}
		if !strings.Contains(scope, mirrorEndpoint) { allChecksPass = false; break }
	}
	if !allChecksPass { logger.Debug("Registry mirror check failed."); return false, nil }

	if s.UseSystemdCgroup && !strings.Contains(content, "SystemdCgroup = true") {
		logger.Debug("SystemdCgroup = true not found."); return false, nil
	}

	// TrimSpace for ExtraTomlContent comparison to handle potential leading/trailing newlines from user input or template.
	if s.ExtraTomlContent != "" && !strings.Contains(content, strings.TrimSpace(s.ExtraTomlContent)) {
	    logger.Debug("Extra TOML content snippet not found."); return false, nil
	}

	for _, insecureReg := range s.InsecureRegistries {
		insecureBlock := fmt.Sprintf("[plugins.\"io.containerd.grpc.v1.cri\".registry.configs.\"%s\".tls]", insecureReg)
		skipLine := "insecure_skip_verify = true"

		idx := strings.Index(content, insecureBlock)
		if idx == -1 { allChecksPass = false; break }

		scope := content[idx+len(insecureBlock):]
		nextBlockStart := strings.Index(scope, "\n[")
		if nextBlockStart != -1 {
			scope = scope[:nextBlockStart]
		}
		if !strings.Contains(scope, skipLine) { allChecksPass = false; break }
	}
    if !allChecksPass { logger.Debug("Insecure registry check failed."); return false, nil }

	logger.Info("Containerd config file content checks passed.", "path", s.ConfigFilePath)
	return true, nil // All checks passed
}

func (s *ConfigureContainerdStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	parentDir := filepath.Dir(s.ConfigFilePath)
	// Using Exec to ensure directory creation with sudo if needed, as conn.Mkdir might not directly support sudo.
	mkdirCmd := fmt.Sprintf("mkdir -p %s", parentDir)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, &connector.ExecOptions{Sudo: true})
	if errMkdir != nil {
		return fmt.Errorf("failed to create directory %s for containerd config (stderr: %s) on host %s for step %s: %w", parentDir, string(stderrMkdir), host.GetName(), s.Name(), errMkdir)
	}

	tmpl, err := template.New("containerdConfig").Parse(containerdConfigTemplate)
	if err != nil {
		return fmt.Errorf("dev error: failed to parse internal containerd config template for step %s: %w", s.Name(), err)
	}

	var buf bytes.Buffer
	templateData := map[string]interface{}{
		"RegistryMirrors":    s.RegistryMirrors,
		"InsecureRegistries": s.InsecureRegistries,
		"UseSystemdCgroup":   s.UseSystemdCgroup,
		"ExtraTomlContent":   s.ExtraTomlContent,
	}
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return fmt.Errorf("failed to render containerd config template for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	configContentBytes := buf.Bytes()
	// logger.Debug("Generated containerd config", "content", string(configContentBytes)) // Avoid logging full config unless necessary

	// Use WriteFile with sudo, assuming it's handled by the connector or a sudo-wrapper if needed.
	// If connector.WriteFile doesn't handle sudo, would need to write to temp then sudo mv.
	// For now, assuming WriteFile handles permissions appropriately via connector's setup.
	err = conn.WriteFile(ctx.GoContext(), configContentBytes, s.ConfigFilePath, "0644")
	if err != nil {
		return fmt.Errorf("failed to write containerd config to %s on host %s for step %s: %w", s.ConfigFilePath, host.GetName(), s.Name(), err)
	}

	logger.Info("Containerd configuration written. Restart containerd service for changes to take effect.", "path", s.ConfigFilePath)
	return nil
}

func (s *ConfigureContainerdStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.Name(), err)
	}

	logger.Info("Attempting to remove containerd config file for rollback.", "path", s.ConfigFilePath)
	removeOpts := connector.RemoveOptions{Recursive: false, IgnoreNotExist: true}
	// Assuming conn.Remove handles sudo if connector is configured for it.
	if errRemove := conn.Remove(ctx.GoContext(), s.ConfigFilePath, removeOpts); errRemove != nil {
		logger.Error("Failed to remove containerd config file during rollback.", "path", s.ConfigFilePath, "error", errRemove)
		// Rollback is best-effort, so not returning error
	} else {
		logger.Info("Successfully removed containerd config file if it existed.", "path", s.ConfigFilePath)
	}
	return nil
}

// Ensure ConfigureContainerdStep implements the step.Step interface.
var _ step.Step = (*ConfigureContainerdStep)(nil)
