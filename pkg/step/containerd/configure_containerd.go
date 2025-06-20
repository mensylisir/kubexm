package containerd

import (
	"bytes" // For template rendering
	"fmt"
	"path/filepath"
	"strings"
	"text/template" // For template rendering

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
	// No longer need time directly
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

// ConfigureContainerdStepSpec configures containerd by writing its config.toml.
type ConfigureContainerdStepSpec struct {
	spec.StepMeta      `json:",inline"`
	RegistryMirrors    map[string]string `json:"registryMirrors,omitempty"`
	InsecureRegistries []string          `json:"insecureRegistries,omitempty"`
	ConfigFilePath     string            `json:"configFilePath,omitempty"`
	ExtraTomlContent   string            `json:"extraTomlContent,omitempty"`
	UseSystemdCgroup   bool              `json:"useSystemdCgroup,omitempty"`
}

// NewConfigureContainerdStepSpec creates a new ConfigureContainerdStepSpec.
func NewConfigureContainerdStep(
	registryMirrors map[string]string,
	insecureRegistries []string,
	configFilePath string,
	extraTomlContent string,
	useSystemdCgroup bool,
	name, description string, // Added name and description for StepMeta
) *ConfigureContainerdStepSpec {
	effectivePath := configFilePath
	if effectivePath == "" {
		effectivePath = DefaultContainerdConfigPath
	}

	finalName := name
	if finalName == "" {
		finalName = "Configure containerd (config.toml)"
	}
	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Configures containerd by writing %s with specified mirrors, insecure registries, and cgroup settings.", effectivePath)
	}

	return &ConfigureContainerdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		RegistryMirrors:    registryMirrors,
		InsecureRegistries: insecureRegistries,
		ConfigFilePath:     effectivePath,
		ExtraTomlContent:   extraTomlContent,
		UseSystemdCgroup:   useSystemdCgroup,
	}
}

// Name returns the step's name (implementing step.Step).
func (s *ConfigureContainerdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *ConfigureContainerdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ConfigureContainerdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ConfigureContainerdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ConfigureContainerdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ConfigureContainerdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ConfigureContainerdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.GetName(), err)
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

func (s *ConfigureContainerdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.GetName(), err)
	}

	parentDir := filepath.Dir(s.ConfigFilePath)
	// Using Exec to ensure directory creation with sudo if needed, as conn.Mkdir might not directly support sudo.
	mkdirCmd := fmt.Sprintf("mkdir -p %s", parentDir)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, &connector.ExecOptions{Sudo: true}) // Assume sudo for /etc
	if errMkdir != nil {
		return fmt.Errorf("failed to create directory %s for containerd config (stderr: %s) on host %s for step %s: %w", parentDir, string(stderrMkdir), host.GetName(), s.GetName(), errMkdir)
	}

	tmpl, err := template.New("containerdConfig").Parse(containerdConfigTemplate)
	if err != nil {
		return fmt.Errorf("dev error: failed to parse internal containerd config template for step %s: %w", s.GetName(), err)
	}

	var buf bytes.Buffer
	templateData := map[string]interface{}{
		"RegistryMirrors":    s.RegistryMirrors,
		"InsecureRegistries": s.InsecureRegistries,
		"UseSystemdCgroup":   s.UseSystemdCgroup,
		"ExtraTomlContent":   s.ExtraTomlContent,
	}
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return fmt.Errorf("failed to render containerd config template for host %s for step %s: %w", host.GetName(), s.GetName(), err)
	}

	configContentBytes := buf.Bytes()

	// Assuming conn.CopyContent handles sudo internally if needed, based on FileStat.Sudo hint.
	// Or, if paths are always privileged, ensure connector is sudo-enabled or use Exec with sudo for writes.
	err = conn.CopyContent(ctx.GoContext(), string(configContentBytes), s.ConfigFilePath, connector.FileStat{
		Permissions: "0644",
		Sudo:        true, // Writing to /etc/containerd usually requires sudo
	})
	if err != nil {
		return fmt.Errorf("failed to write containerd config to %s on host %s for step %s: %w", s.ConfigFilePath, host.GetName(), s.GetName(), err)
	}

	logger.Info("Containerd configuration written. Restart containerd service for changes to take effect.", "path", s.ConfigFilePath)
	return nil
}

func (s *ConfigureContainerdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.GetName(), err)
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

// Ensure ConfigureContainerdStepSpec implements the step.Step interface.
var _ step.Step = (*ConfigureContainerdStepSpec)(nil)
