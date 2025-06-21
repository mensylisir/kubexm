package docker

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const DefaultCriDockerdServicePath = "/etc/systemd/system/cri-dockerd.service"

// ConfigureCriDockerdServiceStep modifies the cri-dockerd.service file, typically to set ExecStart arguments.
type ConfigureCriDockerdServiceStep struct {
	meta            spec.StepMeta
	ServiceFilePath string
	ExecStartArgs   map[string]string // Key-value pairs of arguments to ensure/modify, e.g., {"--network-plugin": "cni"}
	Sudo            bool
}

// NewConfigureCriDockerdServiceStep creates a new ConfigureCriDockerdServiceStep.
func NewConfigureCriDockerdServiceStep(instanceName, serviceFilePath string, execStartArgs map[string]string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "ConfigureCriDockerdService"
	}
	path := serviceFilePath
	if path == "" {
		path = DefaultCriDockerdServicePath
	}

	return &ConfigureCriDockerdServiceStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Configures cri-dockerd.service file at %s with arguments: %v", path, execStartArgs),
		},
		ServiceFilePath: path,
		ExecStartArgs:   execStartArgs, // e.g. {"--container-runtime-endpoint": "fd://", "--network-plugin": "cni"}
		Sudo:            true,
	}
}

func (s *ConfigureCriDockerdServiceStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ConfigureCriDockerdServiceStep) modifyExecStart(currentExecStart string, argsToSet map[string]string) string {
	parts := strings.Fields(currentExecStart)
	if len(parts) == 0 {
		return "" // Should not happen for a valid service file
	}

	// Naive approach: remove existing instances of the keys we want to set, then append.
	// A more robust solution would parse arguments properly, considering quotes, etc.

	// Store the command (parts[0])
	command := parts[0]
	existingArgs := make(map[string]string)
	currentKey := ""

	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "--") {
			currentKey = part
			// If it's a boolean flag (no value follows or next part is another flag)
			if _, ok := argsToSet[currentKey]; ok && (len(parts) == (indexOf(parts, part)+1) || strings.HasPrefix(parts[indexOf(parts, part)+1], "--")) {
				existingArgs[currentKey] = "true" // Placeholder for boolean flags
			} else {
				existingArgs[currentKey] = "" // Expecting a value next
			}
		} else if currentKey != "" {
			if existingArgs[currentKey] == "" { // First part of value
				existingArgs[currentKey] = part
			} else if existingArgs[currentKey] != "true" { // Subsequent parts of value (if space in value, not typical for flags)
				existingArgs[currentKey] += " " + part
			}
			// If it was a boolean flag, this part is a new key, so reset currentKey
			if strings.HasPrefix(part, "--") {
                 currentKey = part
                 existingArgs[currentKey] = ""
            } else if existingArgs[currentKey] != "true" { // Only reset if we actually consumed a value.
                 currentKey = "" // Reset after consuming value
            }
		}
	}

	// Override with new values or add new ones
	for key, value := range argsToSet {
		existingArgs[key] = value
	}

	// Reconstruct
	finalArgs := []string{command}
	for key, value := range existingArgs {
		finalArgs = append(finalArgs, key)
		if value != "true" && value != "" { // Don't append value for boolean flags or if value is empty
			finalArgs = append(finalArgs, value)
		}
	}
	return strings.Join(finalArgs, " ")
}
// Helper to find index, needed for naive arg parsing.
func indexOf(slice []string, item string) int {
    for i, v := range slice {
        if v == item {
            return i
        }
    }
    return -1
}


func (s *ConfigureCriDockerdServiceStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	if len(s.ExecStartArgs) == 0 {
		logger.Info("No ExecStart arguments to configure, precheck considered done.")
		return true, nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.ServiceFilePath)
	if err != nil {
		logger.Warn("Failed to check for existing service file, will attempt configuration.", "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("Service file does not exist.", "path", s.ServiceFilePath)
		return false, nil // Needs to be created first by InstallCriDockerdBinaryStep
	}

	currentContentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ServiceFilePath)
	if err != nil {
		logger.Warn("Failed to read existing service file for comparison, will reconfigure.", "error", err)
		return false, nil
	}
	currentContent := string(currentContentBytes)

	currentExecStart := ""
	for _, line := range strings.Split(currentContent, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "ExecStart=") {
			currentExecStart = strings.TrimPrefix(trimmedLine, "ExecStart=")
			break
		}
	}
	if currentExecStart == "" {
		logger.Warn("Could not find ExecStart in existing service file. Will attempt to configure.", "path", s.ServiceFilePath)
		return false, nil
	}

	modifiedExecStart := s.modifyExecStart(currentExecStart, s.ExecStartArgs)
	expectedLine := "ExecStart=" + modifiedExecStart

	// Check if all configured args are present as expected
	// This simple check might not be robust enough if order matters or if other args exist.
	if strings.Contains(currentContent, expectedLine) { // This is a very basic check.
		logger.Info("cri-dockerd.service file ExecStart seems to contain the target configuration.")
		return true, nil
	}

	logger.Info("cri-dockerd.service file ExecStart needs update.")
	return false, nil
}

func (s *ConfigureCriDockerdServiceStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	if len(s.ExecStartArgs) == 0 {
		logger.Info("No ExecStart arguments to configure.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	currentContentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ServiceFilePath)
	if err != nil {
		return fmt.Errorf("failed to read service file %s: %w", s.ServiceFilePath, err)
	}
	currentContent := string(currentContentBytes)
	lines := strings.Split(currentContent, "\n")
	var newLines []string
	execStartFound := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "ExecStart=") {
			currentExecStart := strings.TrimPrefix(trimmedLine, "ExecStart=")
			modifiedExecStart := s.modifyExecStart(currentExecStart, s.ExecStartArgs)
			// Preserve original indentation if any
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			newLines = append(newLines, indent+"ExecStart="+modifiedExecStart)
			execStartFound = true
		} else {
			newLines = append(newLines, line)
		}
	}

	if !execStartFound {
		// This case should ideally not happen if the service file was installed correctly.
		// Append a new ExecStart line if it's missing (e.g. in [Service] section).
		// This is a simplified addition; proper INI parsing would be better.
		logger.Warn("ExecStart line not found in service file. Appending a new one.", "path", s.ServiceFilePath)
		// Attempt to build a default ExecStart with configured args
		baseCmd := "/usr/local/bin/cri-dockerd" // Default, should be from a constant or config
		modifiedExecStart := s.modifyExecStart(baseCmd, s.ExecStartArgs)

		// Find [Service] section to append to, or just append at the end
		serviceSectionIndex := -1
		for i, line := range newLines {
			if strings.TrimSpace(line) == "[Service]" {
				serviceSectionIndex = i
				break
			}
		}
		execStartLine := "ExecStart=" + modifiedExecStart
		if serviceSectionIndex != -1 && serviceSectionIndex+1 < len(newLines) {
			newLines = append(newLines[:serviceSectionIndex+1], append([]string{execStartLine}, newLines[serviceSectionIndex+1:]...)...)
		} else {
			newLines = append(newLines, "[Service]", execStartLine) // Add section if not found
		}
	}

	newContent := strings.Join(newLines, "\n")
	logger.Info("Writing updated cri-dockerd.service file.", "path", s.ServiceFilePath)
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(newContent), s.ServiceFilePath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write updated cri-dockerd.service to %s: %w", s.ServiceFilePath, err)
	}

	logger.Info("cri-dockerd.service file configured. Run 'systemctl daemon-reload'.")
	return nil
}

func (s *ConfigureCriDockerdServiceStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for ConfigureCriDockerdServiceStep is complex (would require storing original file content) and is not implemented. Manual restore or re-running install steps might be needed.")
	return nil
}

var _ step.Step = (*ConfigureCriDockerdServiceStep)(nil)
