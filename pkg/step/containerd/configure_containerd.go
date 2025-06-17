package containerd

import (
	"context"
	"fmt"
	"path/filepath" // Used for filepath.Dir
	"strings"
	"text/template"
	"time"

	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

const DefaultContainerdConfigPath = "/etc/containerd/config.toml"

// ConfigureContainerdMirrorStep configures containerd registry mirrors and other settings
// by rendering a config.toml file.
type ConfigureContainerdMirrorStep struct {
	// RegistryMirrors maps registry hostnames to their mirror URLs.
	// E.g., {"docker.io": "https://mirror.example.com"}
	RegistryMirrors map[string]string
	// InsecureRegistries is a list of registries to connect to over HTTP or with self-signed certs.
	// E.g., ["my.insecure.registry:5000"]
	InsecureRegistries []string
	// ConfigFilePath is the path to containerd's config.toml.
	// Defaults to /etc/containerd/config.toml.
	ConfigFilePath string
	// ExtraTomlContent allows appending arbitrary valid TOML string to the configuration.
	// Useful for settings not directly exposed by this step's fields.
	ExtraTomlContent string

	// UseSystemdCgroup specifies whether to configure containerd to use systemd cgroup driver.
	// This is a common requirement for Kubernetes.
	UseSystemdCgroup bool
}

// Name returns a human-readable name for the step.
func (s *ConfigureContainerdMirrorStep) Name() string {
	return "Configure containerd (config.toml)"
}

func (s *ConfigureContainerdMirrorStep) effectiveConfigPath() string {
	if s.ConfigFilePath == "" {
		return DefaultContainerdConfigPath
	}
	return s.ConfigFilePath
}

// Check determines if the containerd configuration reflects the desired state.
// This is a basic check and verifies existence and key string presence.
// A fully robust check would require TOML parsing and comparison.
func (s *ConfigureContainerdMirrorStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()

	configPath := s.effectiveConfigPath()
	exists, err := ctx.Host.Runner.Exists(ctx.GoContext, configPath)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of %s on host %s: %w", configPath, ctx.Host.Name, err)
	}
	if !exists {
		hostCtxLogger.Debugf("Containerd config file %s does not exist.", configPath)
		return false, nil
	}

	// If no specific configurations are provided to the step, and the file exists, consider it done.
	if len(s.RegistryMirrors) == 0 && s.ExtraTomlContent == "" && !s.UseSystemdCgroup && len(s.InsecureRegistries) == 0 {
		hostCtxLogger.Infof("No specific containerd configurations to check in %s beyond existence.", configPath)
		return true, nil
	}

	contentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, configPath)
	if err != nil {
		// If we can't read the file, we can't confirm its state, so assume not done.
		hostCtxLogger.Warnf("Failed to read %s for checking current configuration: %v. Assuming not configured.", configPath, err)
		return false, nil
	}
	content := string(contentBytes)

	allChecksPass := true
	for registry, mirror := range s.RegistryMirrors {
		// This is a simplified check. Robust TOML parsing would be better.
		// Example: [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
		//            endpoint = ["https://mirror.example.com"]
		mirrorRegistryPath := fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors.%q]`, registry)
		mirrorEndpointLine := fmt.Sprintf(`endpoint = ["%s"]`, mirror)

		// Look for the section and then the endpoint within it (basic check)
		sectionIndex := strings.Index(content, mirrorRegistryPath)
		if sectionIndex == -1 {
			hostCtxLogger.Debugf("Mirror section for registry %q not found in %s.", registry, configPath)
			allChecksPass = false; break
		}
		// Check for endpoint within a reasonable distance after the section header
		searchScopeAfterSection := content[sectionIndex:]
		if !strings.Contains(searchScopeAfterSection, mirrorEndpointLine) { // This check is very basic
			hostCtxLogger.Debugf("Mirror endpoint %s for registry %q not found (or not correctly associated) in %s.", mirrorEndpointLine, registry, configPath)
			allChecksPass = false; break
		}
	}
	if !allChecksPass { return false, nil }

	if s.UseSystemdCgroup {
		// Example: SystemdCgroup = true
		// This should be under [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
		if !strings.Contains(content, "SystemdCgroup = true") { // Basic string check
			hostCtxLogger.Debugf("SystemdCgroup = true not found in %s.", configPath)
			allChecksPass = false
		}
	}
	if !allChecksPass { return false, nil }

	if s.ExtraTomlContent != "" {
	    if !strings.Contains(content, strings.TrimSpace(s.ExtraTomlContent)) {
	        hostCtxLogger.Debugf("Extra TOML content snippet not found in %s.", configPath)
	        allChecksPass = false
	    }
	}
    if !allChecksPass { return false, nil }

	for _, insecureReg := range s.InsecureRegistries {
		// Example: [plugins."io.containerd.grpc.v1.cri".registry.configs."my.insecure.registry:5000".tls]
		//            insecure_skip_verify = true
		insecureConfigPath := fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.configs.%q.tls]`, insecureReg)
		insecureSkipLine := "insecure_skip_verify = true"

		sectionIndex := strings.Index(content, insecureConfigPath)
		if sectionIndex == -1 {
			hostCtxLogger.Debugf("Insecure registry TLS section for %q not found in %s.", insecureReg, configPath)
			allChecksPass = false; break
		}
		searchScopeAfterSection := content[sectionIndex:]
		if !strings.Contains(searchScopeAfterSection, insecureSkipLine) {
			hostCtxLogger.Debugf("insecure_skip_verify = true for registry %q not found in %s.", insecureReg, configPath)
			allChecksPass = false; break
		}
	}

	hostCtxLogger.Debugf("Containerd config file %s exists. Basic content checks result: %v", configPath, allChecksPass)
	return allChecksPass, nil
}


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
      {{range .}}
      [plugins."io.containerd.grpc.v1.cri".registry.configs."{{.}}".tls] # {{.}} is the insecure registry hostname:port
        insecure_skip_verify = true # For HTTP or self-signed HTTPS
      {{end}}
    {{end}}

{{if .ExtraTomlContent}}
# Extra TOML content provided by user
{{.ExtraTomlContent}}
{{end}}
`

// Run generates the config.toml file and writes it to the host.
func (s *ConfigureContainerdMirrorStep) Run(ctx *runtime.Context) *step.Result {
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()

	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	configPath := s.effectiveConfigPath()

	parentDir := filepath.Dir(configPath)
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, parentDir, "0755", true); err != nil {
		res.Error = fmt.Errorf("failed to create directory %s for containerd config on host %s: %w", parentDir, ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	tmpl, err := template.New("containerdConfig").Parse(containerdConfigTemplate)
	if err != nil {
		res.Error = fmt.Errorf("failed to parse internal containerd config template: %w", err)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v (this is a developer error)", res.Error) // Should not happen with valid template
		return res
	}

	var buf strings.Builder
	templateData := map[string]interface{}{
		"RegistryMirrors":    s.RegistryMirrors,
		"InsecureRegistries": s.InsecureRegistries, // Pass this to the template
		"UseSystemdCgroup":   s.UseSystemdCgroup,
		"ExtraTomlContent":   s.ExtraTomlContent,
	}
	if err := tmpl.Execute(&buf, templateData); err != nil {
		res.Error = fmt.Errorf("failed to render containerd config template for host %s: %w", ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	configContent := buf.String()
	hostCtxLogger.Debugf("Generated containerd config for %s:
%s", configPath, configContent)

	err = ctx.Host.Runner.WriteFile(ctx.GoContext, []byte(configContent), configPath, "0644", true)
	if err != nil {
		res.Error = fmt.Errorf("failed to write containerd config to %s on host %s: %w", configPath, ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	res.EndTime = time.Now()
	res.Status = "Succeeded"
	res.Message = fmt.Sprintf("Containerd configuration written to %s successfully on host %s.", configPath, ctx.Host.Name)
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)

	res.Message += " Containerd may need to be restarted for changes to take effect."
	hostCtxLogger.Infof("Containerd may need to be restarted on host %s for changes to %s to take effect.", ctx.Host.Name, configPath)

	return res
}

var _ step.Step = &ConfigureContainerdMirrorStep{}
