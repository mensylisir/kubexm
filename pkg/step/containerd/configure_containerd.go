package containerd

import (
	"bytes" // For template rendering
	"fmt"
	"path/filepath"
	"strings"
	"text/template" // For template rendering

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

const DefaultContainerdConfigPath = "/etc/containerd/config.toml"

// MirrorConfiguration holds endpoint and rewrite rules for a registry mirror.
type MirrorConfiguration struct {
	Endpoints []string
	Rewrite   map[string]string // Optional: for advanced image name rewriting
}

// containerdConfigTemplate is the Go template for generating config.toml.
const containerdConfigTemplate = `
version = 2
# Ensure CRI plugin is enabled for Kubernetes. Default is often enabled.
# If you need to ensure it's not in disabled_plugins:
# disabled_plugins = [] # Or ensure "io.containerd.grpc.v1.cri" is not listed if others are disabled.

[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = "{{ .SandboxImage }}"
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
    config_path = "{{ .RegistryConfigPath }}" # Set to a directory like "/etc/containerd/certs.d" if using individual host certs/configs.
    {{if .RegistryMirrors}}
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
      {{range $registry, $mirrorConfig := .RegistryMirrors}}
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{$registry}}"]
        endpoint = [{{range $i, $ep := $mirrorConfig.Endpoints}}{{if $i}}, {{end}}"{{$ep}}"{{end}}]
        {{- if $mirrorConfig.Rewrite}}
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{$registry}}".rewrite]
          {{- range $pattern, $replacement := $mirrorConfig.Rewrite}}
          "{{$pattern}}" = "{{$replacement}}"
          {{- end}}
        {{- end}}
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
	meta               spec.StepMeta
	SandboxImage       string
	RegistryMirrors    map[string]MirrorConfiguration // Changed to MirrorConfiguration struct
	InsecureRegistries []string
	ConfigFilePath     string
	RegistryConfigPath string // For plugins."io.containerd.grpc.v1.cri".registry.config_path
	ExtraTomlContent   string
	UseSystemdCgroup   bool
	Sudo               bool
}

// NewConfigureContainerdStep creates a new ConfigureContainerdStep.
func NewConfigureContainerdStep(
	instanceName string,
	sandboxImage string,
	registryMirrors map[string]MirrorConfiguration,
	insecureRegistries []string,
	configFilePath string,
	registryConfigPath string,
	extraTomlContent string,
	useSystemdCgroup bool,
	sudo bool,
) step.Step {
	effectivePath := configFilePath
	if effectivePath == "" {
		effectivePath = DefaultContainerdConfigPath
	}
	effectiveSandboxImage := sandboxImage
	if effectiveSandboxImage == "" {
		effectiveSandboxImage = "registry.k8s.io/pause:3.9" // Common default
	}
	// effectiveRegistryConfigPath defaults to empty string if not provided, which is fine for the template.

	name := instanceName
	if name == "" {
		name = "ConfigureContainerdToml"
	}
	return &ConfigureContainerdStep{
		meta: spec.StepMeta{
			Name: name,
			Description: fmt.Sprintf("Configures containerd by writing %s with specified mirrors, insecure registries, sandbox image and cgroup settings.", effectivePath),
		},
		SandboxImage:       effectiveSandboxImage,
		RegistryMirrors:    registryMirrors,
		InsecureRegistries: insecureRegistries,
		ConfigFilePath:     effectivePath,
		RegistryConfigPath: registryConfigPath,
		ExtraTomlContent:   extraTomlContent,
		UseSystemdCgroup:   useSystemdCgroup,
		Sudo:               sudo,
	}
}

// Meta returns the step's metadata.
func (s *ConfigureContainerdStep) Meta() *spec.StepMeta {
	return &s.meta
}

// renderExpectedConfig generates the expected config content for precheck comparison.
func (s *ConfigureContainerdStep) renderExpectedConfig() (string, error) {
	tmpl, err := template.New("containerdConfig").Parse(containerdConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("dev error: failed to parse internal containerd config template: %w", err)
	}
	var expectedBuf bytes.Buffer
	templateData := map[string]interface{}{
		"SandboxImage":       s.SandboxImage,
		"RegistryMirrors":    s.RegistryMirrors,
		"InsecureRegistries": s.InsecureRegistries,
		"UseSystemdCgroup":   s.UseSystemdCgroup,
		"RegistryConfigPath": s.RegistryConfigPath,
		"ExtraTomlContent":   s.ExtraTomlContent,
	}
	if err := tmpl.Execute(&expectedBuf, templateData); err != nil {
		return "", fmt.Errorf("failed to render expected containerd config template: %w", err)
	}
	// TrimSpace is important to remove leading/trailing newlines from template rendering
	return strings.TrimSpace(expectedBuf.String()), nil
}


func (s *ConfigureContainerdStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.ConfigFilePath)
	if err != nil {
		logger.Warn("Failed to check existence of config file. Assuming not configured.", "path", s.ConfigFilePath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Debug("Containerd config file does not exist.", "path", s.ConfigFilePath)
		return false, nil
	}

	// Generate expected content
	expectedContent, err := s.renderExpectedConfig()
	if err != nil {
		logger.Error("Failed to render expected config for precheck comparison.", "error", err)
		return false, err // Propagate error as it's a step logic issue
	}

	// Read current content
	currentContentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ConfigFilePath)
	if err != nil {
		logger.Warn("Failed to read existing config for comparison. Assuming not configured correctly.", "path", s.ConfigFilePath, "error", err)
		return false, nil
	}
	currentContent := strings.TrimSpace(string(currentContentBytes))

	// Normalize line endings for comparison
	currentContent = strings.ReplaceAll(currentContent, "\r\n", "\n")
	// expectedContent is already normalized by TrimSpace and template rendering typically uses \n

	if currentContent == expectedContent {
		logger.Info("Containerd config file content matches expected configuration.", "path", s.ConfigFilePath)
		return true, nil
	}

	logger.Info("Containerd config file content does not match expected configuration. Will regenerate.", "path", s.ConfigFilePath)
	// For debugging:
	// logger.Debug("Current content", "value", currentContent)
	// logger.Debug("Expected content", "value", expectedContent)
	return false, nil
}

func (s *ConfigureContainerdStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	parentDir := filepath.Dir(s.ConfigFilePath)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, parentDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create directory %s for containerd config on host %s for step %s: %w", parentDir, host.GetName(), s.meta.Name, err)
	}

	configContent, err := s.renderExpectedConfig() // Use the same rendering logic
	if err != nil {
		return err // Error from rendering
	}

	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(configContent), s.ConfigFilePath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write containerd config to %s on host %s for step %s: %w", s.ConfigFilePath, host.GetName(), s.meta.Name, err)
	}

	logger.Info("Containerd configuration written. Restart containerd service for changes to take effect.", "path", s.ConfigFilePath)
	return nil
}

func (s *ConfigureContainerdStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.meta.Name, err)
	}

	logger.Info("Attempting to remove containerd config file for rollback.", "path", s.ConfigFilePath)
	if errRemove := runnerSvc.Remove(ctx.GoContext(), conn, s.ConfigFilePath, s.Sudo); errRemove != nil {
		logger.Error("Failed to remove containerd config file during rollback.", "path", s.ConfigFilePath, "error", errRemove)
		// Rollback is best-effort
	} else {
		logger.Info("Successfully removed containerd config file if it existed.", "path", s.ConfigFilePath)
	}
	return nil
}

// Ensure ConfigureContainerdStep implements the step.Step interface.
var _ step.Step = (*ConfigureContainerdStep)(nil)
