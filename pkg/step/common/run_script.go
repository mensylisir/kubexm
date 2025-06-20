package common

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

// randomString generates a random hex string of length n.
func randomString(n int) string {
	bytes := make([]byte, n/2+(n%2)) // n/2 bytes for hex string of length n
	if _, err := rand.Read(bytes); err != nil {
		// Fallback if crypto/rand fails, though highly unlikely.
		return "randfallback"
	}
	return hex.EncodeToString(bytes)[:n]
}

// RunScriptStepSpec defines parameters for executing a script.
type RunScriptStepSpec struct {
	spec.StepMeta `json:",inline"`

	ScriptContent    string   `json:"scriptContent,omitempty"`
	ScriptPath       string   `json:"scriptPath,omitempty"`
	WorkingDirectory string   `json:"workingDirectory,omitempty"`
	Interpreter      string   `json:"interpreter,omitempty"`
	Sudo             bool     `json:"sudo,omitempty"`
	Args             []string `json:"args,omitempty"`
	TempScriptPrefix string   `json:"tempScriptPrefix,omitempty"`
}

// NewRunScriptStepSpecWithContent creates a new RunScriptStepSpec for inline script content.
func NewRunScriptStepSpecWithContent(name, description, scriptContent string) *RunScriptStepSpec {
	finalName, finalDesc := name, description
	if finalName == "" {
		finalName = "Run Inline Script"
	}
	// Description refined in populateDefaults
	return &RunScriptStepSpec{
		StepMeta:      spec.StepMeta{Name: finalName, Description: finalDesc},
		ScriptContent: scriptContent,
	}
}

// NewRunScriptStepSpecWithPath creates a new RunScriptStepSpec for a script path.
func NewRunScriptStepSpecWithPath(name, description, scriptPath string) *RunScriptStepSpec {
	finalName, finalDesc := name, description
	if finalName == "" {
		finalName = fmt.Sprintf("Run Script %s", scriptPath)
	}
	// Description refined in populateDefaults
	return &RunScriptStepSpec{
		StepMeta:   spec.StepMeta{Name: finalName, Description: finalDesc},
		ScriptPath: scriptPath,
	}
}

// Name returns the step's name.
func (s *RunScriptStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *RunScriptStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *RunScriptStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *RunScriptStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *RunScriptStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *RunScriptStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *RunScriptStepSpec) populateDefaults(logger runtime.Logger) {
	if s.Interpreter == "" {
		s.Interpreter = "/bin/bash"
		logger.Debug("Interpreter defaulted to /bin/bash.")
	}
	if s.TempScriptPrefix == "" {
		s.TempScriptPrefix = "kubexm_script_"
		logger.Debug("TempScriptPrefix defaulted.", "prefix", s.TempScriptPrefix)
	}
	// Sudo defaults to false (zero value).

	if s.StepMeta.Description == "" {
		if s.ScriptPath != "" {
			s.StepMeta.Description = fmt.Sprintf("Runs script from path %s using %s", s.ScriptPath, s.Interpreter)
		} else {
			contentSnippet := s.ScriptContent
			if len(contentSnippet) > 30 {
				contentSnippet = contentSnippet[:27] + "..."
			}
			s.StepMeta.Description = fmt.Sprintf("Runs inline script starting with '%s' using %s", contentSnippet, s.Interpreter)
		}
		if s.Sudo {
			s.StepMeta.Description += " with sudo"
		}
		if len(s.Args) > 0 {
			s.StepMeta.Description += fmt.Sprintf(" (Args: %s)", strings.Join(s.Args, " "))
		}
	}
}

// Precheck validates inputs and checks for remote script existence if applicable.
func (s *RunScriptStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.ScriptContent == "" && s.ScriptPath == "" {
		return false, fmt.Errorf("either ScriptContent or ScriptPath must be provided for %s", s.GetName())
	}
	if s.ScriptContent != "" && s.ScriptPath != "" {
		return false, fmt.Errorf("ScriptContent and ScriptPath cannot both be provided for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if s.ScriptPath != "" {
		exists, err := conn.Exists(ctx.GoContext(), s.ScriptPath)
		if err != nil {
			return false, fmt.Errorf("failed to check existence of script %s on host %s: %w", s.ScriptPath, host.GetName(), err)
		}
		if !exists {
			return false, fmt.Errorf("script path %s does not exist on host %s", s.ScriptPath, host.GetName())
		}
		// TODO: Check for execute permissions if possible via connector, though not all provide stat-like features easily.
		// For now, existence is the main precheck for ScriptPath.
	}

	// Check if interpreter exists
	if _, err := conn.LookPath(ctx.GoContext(), s.Interpreter); err != nil {
		return false, fmt.Errorf("interpreter %s not found on host %s: %w", s.Interpreter, host.GetName(), err)
	}


	logger.Debug("Precheck passed, script will be run.")
	return false, nil // Always run if inputs are valid, idempotency is script's concern.
}

// Run executes the script.
func (s *RunScriptStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if (s.ScriptContent == "" && s.ScriptPath == "") || (s.ScriptContent != "" && s.ScriptPath != "") {
		return fmt.Errorf("validation failed for %s: either ScriptContent or ScriptPath must be provided, but not both", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	executableScriptPath := s.ScriptPath
	var tempScriptPathOnHost string // Used if ScriptContent is provided

	cleanupTempScript := func() {
		if tempScriptPathOnHost != "" {
			logger.Debug("Cleaning up temporary script.", "path", tempScriptPathOnHost)
			// Sudo for removal should match sudo for creation/execution context.
			// If script content was written to /tmp, typically doesn't need sudo to remove.
			// However, if working dir was privileged, it might.
			// Using s.Sudo here for consistency if the script itself ran with sudo.
			err := conn.Remove(ctx.GoContext(), tempScriptPathOnHost, connector.RemoveOptions{Sudo: s.Sudo, IgnoreNotExist: true})
			if err != nil {
				logger.Warn("Failed to remove temporary script file.", "path", tempScriptPathOnHost, "error", err)
			}
		}
	}

	if s.ScriptContent != "" {
		// Create a unique name for the temporary script
		// Using a simpler random string for now.
		randSuffix := randomString(8)
		tempScriptName := s.TempScriptPrefix + randSuffix
		if !strings.Contains(s.Interpreter, "python") { // Add .sh for shell scripts for clarity
		    tempScriptName += ".sh"
		}

		// Determine temp directory. User's home or /tmp.
		// For now, hardcoding to /tmp as it's generally writable.
		// A more robust solution might use mktemp command on host or specific workdir from context.
		tempDir := "/tmp"
		tempScriptPathOnHost = filepath.Join(tempDir, tempScriptName)
		executableScriptPath = tempScriptPathOnHost

		logger.Info("Writing inline script content to temporary file on host.", "path", tempScriptPathOnHost)
		// Permissions for script. Sudo for CopyContent if tempDir requires it (unlikely for /tmp).
		// Let's assume /tmp is writable by default by connection user.
		errWrite := conn.CopyContent(ctx.GoContext(), s.ScriptContent, tempScriptPathOnHost, connector.FileStat{
			Permissions: "0755", // Make it executable
			Sudo:        utils.PathRequiresSudo(tempDir), // Sudo if /tmp needs it (rare)
		})
		if errWrite != nil {
			return fmt.Errorf("failed to write temporary script to %s on host %s: %w", tempScriptPathOnHost, host.GetName(), errWrite)
		}
		// Defer cleanup only if script was successfully written.
		defer cleanupTempScript()
	} else { // s.ScriptPath is used
	    // Ensure script has execute permissions if it's a remote path being executed.
	    // This might be done here or assumed to be pre-set. For now, assume pre-set.
	    // chmodCmd := fmt.Sprintf("chmod +x %s", executableScriptPath)
	    // _, _, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, &connector.ExecOptions{Sudo: s.Sudo})
	    // if errChmod != nil { return fmt.Errorf("failed to make script %s executable: %w", executableScriptPath, errChmod)}
	}


	cmdParts := []string{}
	if s.Sudo {
		cmdParts = append(cmdParts, "sudo")
	}
	if s.Interpreter != "" {
		cmdParts = append(cmdParts, s.Interpreter)
	}
	cmdParts = append(cmdParts, executableScriptPath)
	if len(s.Args) > 0 {
		cmdParts = append(cmdParts, s.Args...)
	}

	finalCmd := strings.Join(cmdParts, " ")
	execOpts := &connector.ExecOptions{Sudo: false} // Sudo is already prepended if s.Sudo is true

	if s.WorkingDirectory != "" {
		// Ensure WorkingDirectory exists.
		mkdirCmd := fmt.Sprintf("mkdir -p %s", s.WorkingDirectory)
		// Sudo for mkdir based on path.
		_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, &connector.ExecOptions{Sudo: s.Sudo || utils.PathRequiresSudo(s.WorkingDirectory)})
		if errMkdir != nil {
			return fmt.Errorf("failed to create working directory %s (stderr: %s): %w", s.WorkingDirectory, string(stderrMkdir), errMkdir)
		}
		finalCmd = fmt.Sprintf("cd %s && %s", s.WorkingDirectory, finalCmd)
		logger.Debug("Executing command with working directory.", "command", finalCmd, "wd", s.WorkingDirectory)
	}


	logger.Info("Executing script.", "command", finalCmd)
	stdout, stderr, err := conn.Exec(ctx.GoContext(), finalCmd, execOpts)
	if err != nil {
		// Note: stdout and stderr might be large. Consider truncating or logging them carefully.
		logger.Error("Script execution failed.", "stdout_len", len(stdout), "stderr_len", len(stderr), "error", err)
		// Provide some stderr output for easier debugging.
		stderrSnippet := string(stderr)
		if len(stderrSnippet) > 200 { // Limit snippet size
			stderrSnippet = stderrSnippet[:200] + "..."
		}
		return fmt.Errorf("script execution failed for '%s' (stderr snippet: '%s'): %w", s.GetName(), stderrSnippet, err)
	}

	logger.Info("Script executed successfully.", "stdout_len", len(stdout), "stderr_len", len(stderr))
	// Optionally log snippets of stdout/stderr if needed, e.g. on debug level.
	// logger.Debug("Script stdout", "output", string(stdout))
	// logger.Debug("Script stderr", "output", string(stderr))
	return nil
}

// Rollback for generic script execution is not supported.
func (s *RunScriptStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for generic script execution is not supported by this step. Any side effects of the script need manual reversal if required.")
	return nil
}

var _ step.Step = (*RunScriptStepSpec)(nil)
