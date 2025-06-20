package containerd

import (
	"context" // Required by runtime.Context
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // Added
	// "github.com/kubexms/kubexms/pkg/runner" // Not directly needed here, runner methods are on ctx.Host.Runner
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const DefaultContainerdConfigPath = "/etc/containerd/config.toml"

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

// ConfigureContainerdStepSpec defines parameters for configuring containerd.
type ConfigureContainerdStepSpec struct {
	RegistryMirrors    map[string]string
	InsecureRegistries []string
	ConfigFilePath     string // Defaults to DefaultContainerdConfigPath
	ExtraTomlContent   string
	UseSystemdCgroup   bool
	StepName           string // Optional: for a custom name for this specific step instance
}

// GetName returns the step name.
func (s *ConfigureContainerdStepSpec) GetName() string {
	if s.StepName != "" { return s.StepName }
	return "Configure containerd (config.toml)"
}

// effectiveConfigPath returns the actual config file path to be used.
func (s *ConfigureContainerdStepSpec) effectiveConfigPath() string {
	if s.ConfigFilePath == "" { return DefaultContainerdConfigPath }
	return s.ConfigFilePath
}
var _ spec.StepSpec = &ConfigureContainerdStepSpec{}

// ConfigureContainerdStepExecutor implements logic for ConfigureContainerdStepSpec.
type ConfigureContainerdStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&ConfigureContainerdStepSpec{}), &ConfigureContainerdStepExecutor{})
}

// Check determines if containerd config reflects desired state. (Basic check)
func (e *ConfigureContainerdStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for ConfigureContainerdStepExecutor Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for ConfigureContainerdStepExecutor Check method")
	}
	spec, ok := rawSpec.(*ConfigureContainerdStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for ConfigureContainerdStepExecutor Check method: %T", rawSpec)
	}
	logger = logger.With("step", spec.GetName(), "phase", "Check")


	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	configPath := spec.effectiveConfigPath()
	exists, err := conn.Exists(goCtx, configPath) // Use connector
	if err != nil {
		logger.Error("Failed to check existence of config file", "path", configPath, "error", err)
		return false, fmt.Errorf("failed to check existence of %s on host %s: %w", configPath, currentHost.GetName(), err)
	}
	if !exists {
		logger.Debug("Containerd config file does not exist.", "path", configPath)
		return false, nil
	}

	if len(spec.RegistryMirrors) == 0 && spec.ExtraTomlContent == "" && !spec.UseSystemdCgroup && len(spec.InsecureRegistries) == 0 {
		logger.Info("No specific containerd configurations in spec to check beyond existence.", "path", configPath)
		return true, nil // If file exists and no specific content to check, assume it's "done" or correctly configured by other means.
	}

	contentBytes, err := conn.ReadFile(goCtx, configPath) // Use connector
	if err != nil {
		logger.Warn("Failed to read config for checking current configuration. Assuming not configured.", "path", configPath, "error", err)
		return false, nil // If cannot read, assume not configured as desired.
	}
	content := string(contentBytes)

	allChecksPass := true
	for registry, mirror := range spec.RegistryMirrors {
		mirrorEndpoint := fmt.Sprintf(`endpoint = ["%s"]`, mirror)
		// TOML parsing is tricky with simple string contains. This is a basic check.
		// A robust check would parse the TOML.
		mirrorBlock := fmt.Sprintf("[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"%s\"]", registry)
		if idx := strings.Index(content, mirrorBlock); idx != -1 {
			scope := content[idx:]
			if endIdx := strings.Index(scope, "\n["); endIdx != -1 { // Limit scope to this mirror block
				scope = scope[:endIdx]
			}
			if !strings.Contains(scope, mirrorEndpoint) {
				logger.Debug("Mirror endpoint not found or incorrect.", "registry", registry, "endpoint", mirrorEndpoint, "path", configPath)
				allChecksPass = false; break
			}
		} else {
			logger.Debug("Mirror section not found.", "registry", registry, "path", configPath)
			allChecksPass = false; break
		}
	}
	if !allChecksPass { return false, nil }

	if spec.UseSystemdCgroup && !strings.Contains(content, "SystemdCgroup = true") {
		logger.Debug("SystemdCgroup = true not found.", "path", configPath)
		allChecksPass = false
	}
	if !allChecksPass { return false, nil }

	if spec.ExtraTomlContent != "" && !strings.Contains(content, strings.TrimSpace(spec.ExtraTomlContent)) {
	    logger.Debug("Extra TOML content snippet not found.", "path", configPath)
	    allChecksPass = false
	}
    if !allChecksPass { return false, nil }

	for _, insecureReg := range spec.InsecureRegistries {
		// This check is also basic. A robust check would parse TOML.
		// [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.example.com".tls]
		//   insecure_skip_verify = true
		insecureBlock := fmt.Sprintf("[plugins.\"io.containerd.grpc.v1.cri\".registry.configs.\"%s\".tls]", insecureReg)
		skipLine := "insecure_skip_verify = true"
		if idx := strings.Index(content, insecureBlock); idx != -1 {
			scope := content[idx:]
			if endIdx := strings.Index(scope, "\n["); endIdx != -1 {
				scope = scope[:endIdx]
			}
			if !strings.Contains(scope, skipLine) {
				logger.Debug("insecure_skip_verify = true not found for registry.", "registry", insecureReg, "path", configPath)
				allChecksPass = false; break
			}
		} else {
			logger.Debug("Insecure registry TLS section not found.", "registry", insecureReg, "path", configPath)
			allChecksPass = false; break
		}
	}

	logger.Debug("Containerd config file content checks result.", "path", configPath, "passed", allChecksPass)
	return allChecksPass, nil
}


// Execute generates and writes containerd config.
func (e *ConfigureContainerdStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for ConfigureContainerdStepExecutor Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for ConfigureContainerdStepExecutor Execute method")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*ConfigureContainerdStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for ConfigureContainerdStepExecutor Execute method: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("step", spec.GetName(), "phase", "Execute")


	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	configPath := spec.effectiveConfigPath()
	parentDir := filepath.Dir(configPath)

	// Assuming connector.Mkdir(goCtx, path, perms) and it handles sudo via connector setup or options.
	// The original used Mkdirp(goCtx, parentDir, "0755", true) -> true for sudo.
	// Let's assume connector.Mkdir handles recursive and sudo is part of its context/setup.
	if err := conn.Mkdir(goCtx, parentDir, "0755"); err != nil { // Sudo true is implied if connector is sudo-enabled
		logger.Error("Failed to create directory for containerd config", "path", parentDir, "error", err)
		res.Error = fmt.Errorf("failed to create directory %s for containerd config on host %s: %w", parentDir, currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	tmpl, err := template.New("containerdConfig").Parse(containerdConfigTemplate)
	if err != nil {
		// This is a developer error, not a runtime one usually.
		logger.Error("Failed to parse internal containerd config template", "error", err)
		res.Error = fmt.Errorf("dev error: failed to parse internal containerd config template: %w", err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	var buf strings.Builder
	templateData := map[string]interface{}{
		"RegistryMirrors":    spec.RegistryMirrors,
		"InsecureRegistries": spec.InsecureRegistries,
		"UseSystemdCgroup":   spec.UseSystemdCgroup,
		"ExtraTomlContent":   spec.ExtraTomlContent,
	}
	if err := tmpl.Execute(&buf, templateData); err != nil {
		logger.Error("Failed to render containerd config template", "error", err)
		res.Error = fmt.Errorf("failed to render containerd config template for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	configContent := buf.String()
	logger.Debug("Generated containerd config", "path", configPath, "content", configContent) // Be careful logging full content if it's sensitive

	// Assuming connector.WriteFile(goCtx, content []byte, path, permissions string)
	// Original used WriteFile(goCtx, []byte(configContent), configPath, "0644", true) -> true for sudo.
	err = conn.WriteFile(goCtx, []byte(configContent), configPath, "0644") // Sudo true is implied if connector is sudo-enabled
	if err != nil {
		logger.Error("Failed to write containerd config", "path", configPath, "error", err)
		res.Error = fmt.Errorf("failed to write containerd config to %s on host %s: %w", configPath, currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	res.EndTime = time.Now()
	res.Status = step.StatusSucceeded
	res.Message = fmt.Sprintf("Containerd configuration written to %s successfully on host %s.", configPath, currentHost.GetName())
	logger.Info("Step succeeded", "message", res.Message)

	res.Message += " Containerd may need to be restarted for changes to take effect."
	logger.Info("Containerd may need to be restarted for changes to take effect.", "host", currentHost.GetName(), "configPath", configPath)

	return res
}
var _ step.StepExecutor = &ConfigureContainerdStepExecutor{}
