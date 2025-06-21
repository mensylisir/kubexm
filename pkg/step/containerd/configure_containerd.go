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
	meta               spec.StepMeta
	RegistryMirrors    map[string]string
	InsecureRegistries []string
	ConfigFilePath     string
	ExtraTomlContent   string
	UseSystemdCgroup   bool
	Sudo               bool
}

// NewConfigureContainerdStep creates a new ConfigureContainerdStep.
func NewConfigureContainerdStep(
	instanceName string,
	registryMirrors map[string]string,
	insecureRegistries []string,
	configFilePath string,
	extraTomlContent string,
	useSystemdCgroup bool,
	sudo bool,
) step.Step {
	effectivePath := configFilePath
	if effectivePath == "" {
		effectivePath = DefaultContainerdConfigPath
	}
	name := instanceName
	if name == "" {
		name = "ConfigureContainerdToml"
	}
	return &ConfigureContainerdStep{
		meta: spec.StepMeta{
			Name: name,
			Description: fmt.Sprintf("Configures containerd by writing %s with specified mirrors, insecure registries, and cgroup settings.", effectivePath),
		},
		RegistryMirrors:    registryMirrors,
		InsecureRegistries: insecureRegistries,
		ConfigFilePath:     effectivePath,
		ExtraTomlContent:   extraTomlContent,
		UseSystemdCgroup:   useSystemdCgroup,
		Sudo:               sudo,
	}
}

// Meta returns the step's metadata.
func (s *ConfigureContainerdStep) Meta() *spec.StepMeta {
	return &s.meta
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

	if len(s.RegistryMirrors) == 0 && s.ExtraTomlContent == "" && !s.UseSystemdCgroup && len(s.InsecureRegistries) == 0 {
		logger.Info("No specific containerd configurations in spec to check beyond existence.", "path", s.ConfigFilePath)
		return true, nil
	}

	contentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ConfigFilePath)
	if err != nil {
		logger.Warn("Failed to read config for checking current configuration. Assuming not configured.", "path", s.ConfigFilePath, "error", err)
		return false, nil
	}
	content := string(contentBytes)

	allChecksPass := true // Assume true, set to false on any mismatch
	for registry, mirror := range s.RegistryMirrors {
		mirrorEndpoint := fmt.Sprintf(`endpoint = ["%s"]`, mirror)
		mirrorBlock := fmt.Sprintf("[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"%s\"]", registry)
		idx := strings.Index(content, mirrorBlock)
		if idx == -1 {
			allChecksPass = false
			break
		}
		scope := content[idx+len(mirrorBlock):]
		nextBlockStart := strings.Index(scope, "\n[")
		if nextBlockStart != -1 {
			scope = scope[:nextBlockStart]
		}
		if !strings.Contains(scope, mirrorEndpoint) {
			allChecksPass = false
			break
		}
	}
	if !allChecksPass {
		logger.Debug("Registry mirror check failed.")
		return false, nil
	}

	if s.UseSystemdCgroup && !strings.Contains(content, "SystemdCgroup = true") {
		logger.Debug("SystemdCgroup = true not found.")
		return false, nil
	}
	if !s.UseSystemdCgroup && strings.Contains(content, "SystemdCgroup = true") { // Check if it's set when it shouldn't be
		logger.Debug("SystemdCgroup = true found, but expected false.")
		return false, nil
	}


	if s.ExtraTomlContent != "" && !strings.Contains(content, strings.TrimSpace(s.ExtraTomlContent)) {
		logger.Debug("Extra TOML content snippet not found.")
		return false, nil
	}

	for _, insecureReg := range s.InsecureRegistries {
		insecureBlock := fmt.Sprintf("[plugins.\"io.containerd.grpc.v1.cri\".registry.configs.\"%s\".tls]", insecureReg)
		skipLine := "insecure_skip_verify = true"
		idx := strings.Index(content, insecureBlock)
		if idx == -1 {
			allChecksPass = false
			break
		}
		scope := content[idx+len(insecureBlock):]
		nextBlockStart := strings.Index(scope, "\n[")
		if nextBlockStart != -1 {
			scope = scope[:nextBlockStart]
		}
		if !strings.Contains(scope, skipLine) {
			allChecksPass = false
			break
		}
	}
	if !allChecksPass {
		logger.Debug("Insecure registry check failed.")
		return false, nil
	}

	logger.Info("Containerd config file content checks passed.", "path", s.ConfigFilePath)
	return true, nil
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

	tmpl, err := template.New("containerdConfig").Parse(containerdConfigTemplate)
	if err != nil {
		return fmt.Errorf("dev error: failed to parse internal containerd config template for step %s: %w", s.meta.Name, err)
	}

	var buf bytes.Buffer
	templateData := map[string]interface{}{
		"RegistryMirrors":    s.RegistryMirrors,
		"InsecureRegistries": s.InsecureRegistries,
		"UseSystemdCgroup":   s.UseSystemdCgroup,
		"ExtraTomlContent":   s.ExtraTomlContent,
	}
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return fmt.Errorf("failed to render containerd config template for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	configContentBytes := buf.Bytes()

	err = runnerSvc.WriteFile(ctx.GoContext(), conn, configContentBytes, s.ConfigFilePath, "0644", s.Sudo)
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
