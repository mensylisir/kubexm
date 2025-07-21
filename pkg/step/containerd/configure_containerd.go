package containerd

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

const DefaultContainerdConfigPath = "/etc/containerd/config.toml"
const ContainerdConfigTemplateName = "containerd/config.toml.tmpl"

type MirrorConfiguration struct {
	Endpoints []string
	Rewrite   map[string]string // Optional: for advanced image name rewriting
}

// const containerdConfigTemplate = `...` // REMOVED

// ConfigureContainerdStep configures containerd by writing its config.toml.
type ConfigureContainerdStep struct {
	meta               spec.StepMeta
	SandboxImage       string
	APIRegistryMirrors map[string][]string // Store the API spec version
	InsecureRegistries []string
	ConfigFilePath     string
	RegistryConfigPath string // For plugins."io.containerd.grpc.v1.cri".registry.config_path
	ExtraTomlContent   string
	UseSystemdCgroup   bool
	Sudo               bool
}

// NewConfigureContainerdStep creates a new ConfigureContainerdStep.
// Accepts registryMirrors as map[string][]string, matching v1alpha1.ContainerdConfig.
func NewConfigureContainerdStep(
	instanceName string,
	sandboxImage string,
	apiRegistryMirrors map[string][]string, // Changed parameter type
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
			Name:        name,
			Description: fmt.Sprintf("Configures containerd by writing %s with specified mirrors, insecure registries, sandbox image and cgroup settings.", effectivePath),
		},
		SandboxImage:       effectiveSandboxImage,
		APIRegistryMirrors: apiRegistryMirrors, // Use the new field
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
	templateString, err := templates.Get(ContainerdConfigTemplateName)
	if err != nil {
		return "", fmt.Errorf("failed to get containerd config template '%s': %w", ContainerdConfigTemplateName, err)
	}
	tmpl, err := template.New("containerdConfig").Parse(templateString)
	if err != nil {
		return "", fmt.Errorf("dev error: failed to parse containerd config template: %w", err)
	}
	var expectedBuf bytes.Buffer

	// Adapt APIRegistryMirrors (map[string][]string) to the structure expected by the template
	// The template expects map[string]MirrorConfiguration where MirrorConfiguration has Endpoints.
	templateRegistryMirrors := make(map[string]MirrorConfiguration)
	if s.APIRegistryMirrors != nil {
		for host, endpoints := range s.APIRegistryMirrors {
			templateRegistryMirrors[host] = MirrorConfiguration{Endpoints: endpoints}
			// If MirrorConfiguration had a Rewrite field that needed to be populated from API,
			// that logic would go here. For now, assuming only Endpoints.
		}
	}

	templateData := map[string]interface{}{
		"SandboxImage":       s.SandboxImage,
		"RegistryMirrors":    templateRegistryMirrors, // Use the adapted structure
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

func (s *ConfigureContainerdStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
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

func (s *ConfigureContainerdStep) Run(ctx step.StepContext, host connector.Host) error {
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

func (s *ConfigureContainerdStep) Rollback(ctx step.StepContext, host connector.Host) error {
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
