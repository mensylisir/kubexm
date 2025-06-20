package containerd

import (
	"context" // Required by runtime.Context
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	// "github.com/kubexms/kubexms/pkg/runner" // Not directly needed here, runner methods are on ctx.Host.Runner
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
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
func (e *ConfigureContainerdStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for ConfigureContainerdStepExecutor Check method")
	}
	spec, ok := currentFullSpec.(*ConfigureContainerdStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for ConfigureContainerdStepExecutor Check method: %T", currentFullSpec)
	}

	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	// Logger is already contextualized by Task.Run and NewHostContext
	hostCtxLogger := ctx.Logger.SugaredLogger.With("phase", "Check").Sugar()

	configPath := spec.effectiveConfigPath()
	exists, err := ctx.Host.Runner.Exists(ctx.GoContext, configPath)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of %s on host %s: %w", configPath, ctx.Host.Name, err)
	}
	if !exists {
		hostCtxLogger.Debugf("Containerd config file %s does not exist.", configPath)
		return false, nil
	}

	if len(spec.RegistryMirrors) == 0 && spec.ExtraTomlContent == "" && !spec.UseSystemdCgroup && len(spec.InsecureRegistries) == 0 {
		hostCtxLogger.Infof("No specific containerd configurations in spec to check in %s beyond existence.", configPath)
		return true, nil
	}

	contentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, configPath)
	if err != nil {
		hostCtxLogger.Warnf("Failed to read %s for checking current configuration: %v. Assuming not configured.", configPath, err)
		return false, nil
	}
	content := string(contentBytes)

	allChecksPass := true
	for registry, mirror := range spec.RegistryMirrors {
		mirrorEndpoint := fmt.Sprintf(`endpoint = ["%s"]`, mirror)
		mirrorSection := fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors.%q]`, registry)
		sectionIndex := strings.Index(content, mirrorSection)
		if sectionIndex == -1 {
			hostCtxLogger.Debugf("Mirror section for registry %q not found in %s.", registry, configPath)
			allChecksPass = false; break
		}
		searchScopeAfterSection := content[sectionIndex:]
		if !strings.Contains(searchScopeAfterSection, mirrorEndpoint) {
			hostCtxLogger.Debugf("Mirror endpoint %s for registry %q not found (or not correctly associated) in %s.", mirrorEndpoint, registry, configPath)
			allChecksPass = false; break
		}
	}
	if !allChecksPass { return false, nil }

	if spec.UseSystemdCgroup && !strings.Contains(content, "SystemdCgroup = true") {
		hostCtxLogger.Debugf("SystemdCgroup = true not found in %s.", configPath)
		allChecksPass = false
	}
	if !allChecksPass { return false, nil }

	if spec.ExtraTomlContent != "" && !strings.Contains(content, strings.TrimSpace(spec.ExtraTomlContent)) {
	    hostCtxLogger.Debugf("Extra TOML content snippet not found in %s.", configPath)
	    allChecksPass = false
	}
    if !allChecksPass { return false, nil }

	for _, insecureReg := range spec.InsecureRegistries {
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


// Execute generates and writes containerd config.
func (e *ConfigureContainerdStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()

	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for ConfigureContainerdStepExecutor Execute method"))
	}
	spec, ok := currentFullSpec.(*ConfigureContainerdStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for ConfigureContainerdStepExecutor Execute method: %T", currentFullSpec))
	}

	// Logger is already contextualized
	hostCtxLogger := ctx.Logger.SugaredLogger.With("phase", "Execute").Sugar()
	res := step.NewResult(ctx, startTime, nil) // Use ctx for NewResult

	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	configPath := spec.effectiveConfigPath()
	parentDir := filepath.Dir(configPath)

	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, parentDir, "0755", true); err != nil {
		res.Error = fmt.Errorf("failed to create directory %s for containerd config on host %s: %w", parentDir, ctx.Host.Name, err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	tmpl, err := template.New("containerdConfig").Parse(containerdConfigTemplate)
	if err != nil {
		res.Error = fmt.Errorf("dev error: failed to parse internal containerd config template: %w", err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	var buf strings.Builder
	templateData := map[string]interface{}{
		"RegistryMirrors":    spec.RegistryMirrors,
		"InsecureRegistries": spec.InsecureRegistries,
		"UseSystemdCgroup":   spec.UseSystemdCgroup,
		"ExtraTomlContent":   spec.ExtraTomlContent,
	}
	if err := tmpl.Execute(&buf, templateData); err != nil {
		res.Error = fmt.Errorf("failed to render containerd config template for host %s: %w", ctx.Host.Name, err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	configContent := buf.String()
	hostCtxLogger.Debugf("Generated containerd config for %s:\n%s", configPath, configContent)

	// Sudo true for writing to /etc
	err = ctx.Host.Runner.WriteFile(ctx.GoContext, []byte(configContent), configPath, "0644", true)
	if err != nil {
		res.Error = fmt.Errorf("failed to write containerd config to %s on host %s: %w", configPath, ctx.Host.Name, err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	res.EndTime = time.Now(); res.Status = step.StatusSucceeded
	res.Message = fmt.Sprintf("Containerd configuration written to %s successfully on host %s.", configPath, ctx.Host.Name)
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)

	res.Message += " Containerd may need to be restarted for changes to take effect."
	hostCtxLogger.Infof("Containerd may need to be restarted on host %s for changes to %s to take effect.", ctx.Host.Name, configPath)

	return res
}
var _ step.StepExecutor = &ConfigureContainerdStepExecutor{}
