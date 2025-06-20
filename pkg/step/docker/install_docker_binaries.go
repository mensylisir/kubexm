package docker

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

const (
	defaultDockerSystemdUnitPath = "/etc/systemd/system/docker.service"
	defaultDockerExtractedPathKey = "DockerExtractedPath" // Default key if not provided by user
)

// DefaultDockerSystemdUnitTemplate provides a basic systemd unit file for Docker.
const DefaultDockerSystemdUnitTemplate = `[Unit]
Description=Docker Application Container Engine
Documentation=https://docs.docker.com
After=network-online.target firewalld.service containerd.service
Wants=network-online.target
Requires=containerd.service

[Service]
Type=notify
ExecStart={{.DockerdPath}} -H fd:// --containerd=/run/containerd/containerd.sock
ExecReload=/bin/kill -s HUP $MAINPID
TimeoutStartSec=0
RestartSec=2
Restart=always
StartLimitBurst=3
StartLimitInterval=60s
LimitNOFILE=infinity
LimitNPROC=infinity
LimitCORE=infinity
TasksMax=infinity
Delegate=yes
KillMode=process
OOMScoreAdjust=-500

[Install]
WantedBy=multi-user.target
`

// InstallDockerBinariesStepSpec defines parameters for installing Docker binaries and systemd unit.
type InstallDockerBinariesStepSpec struct {
	spec.StepMeta `json:",inline"`

	SourceExtractedPathCacheKey string            `json:"sourceExtractedPathCacheKey,omitempty"` // Required
	BinariesToCopy            map[string]string `json:"binariesToCopy,omitempty"`
	SystemdUnitTemplate       string            `json:"systemdUnitTemplate,omitempty"`
	SystemdUnitPath           string            `json:"systemdUnitPath,omitempty"`
	Sudo                      bool              `json:"sudo,omitempty"`
}

// NewInstallDockerBinariesStepSpec creates a new InstallDockerBinariesStepSpec.
func NewInstallDockerBinariesStepSpec(name, description, sourceExtractedPathCacheKey string) *InstallDockerBinariesStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Install Docker Binaries and Systemd Unit"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if sourceExtractedPathCacheKey == "" {
		// This is required.
	}

	return &InstallDockerBinariesStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		SourceExtractedPathCacheKey: sourceExtractedPathCacheKey,
		// Defaults in populateDefaults
	}
}

// Name returns the step's name.
func (s *InstallDockerBinariesStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *InstallDockerBinariesStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *InstallDockerBinariesStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *InstallDockerBinariesStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *InstallDockerBinariesStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *InstallDockerBinariesStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *InstallDockerBinariesStepSpec) populateDefaults(logger runtime.Logger) {
	if s.SourceExtractedPathCacheKey == "" {
		s.SourceExtractedPathCacheKey = defaultDockerExtractedPathKey
		logger.Debug("SourceExtractedPathCacheKey defaulted.", "key", s.SourceExtractedPathCacheKey)
	}

	if len(s.BinariesToCopy) == 0 {
		s.BinariesToCopy = map[string]string{
			"docker":                  "/usr/bin/docker",
			"dockerd":                 "/usr/bin/dockerd",
			"docker-proxy":            "/usr/bin/docker-proxy",
			"docker-init":             "/usr/bin/docker-init",
			// These are often part of containerd package, but some Docker tgz might include them
			// "containerd":              "/usr/bin/containerd",
			// "containerd-shim-runc-v2": "/usr/bin/containerd-shim-runc-v2",
			// "ctr":                     "/usr/bin/ctr",
			// "runc":                    "/usr/bin/runc",
		}
		logger.Debug("BinariesToCopy defaulted.", "map", s.BinariesToCopy)
	}
	if s.SystemdUnitTemplate == "" {
		s.SystemdUnitTemplate = DefaultDockerSystemdUnitTemplate
		logger.Debug("SystemdUnitTemplate defaulted.")
	}
	if s.SystemdUnitPath == "" {
		s.SystemdUnitPath = defaultDockerSystemdUnitPath
		logger.Debug("SystemdUnitPath defaulted.", "path", s.SystemdUnitPath)
	}
	if !s.Sudo { // Default to true if not explicitly set to false
		s.Sudo = true
		logger.Debug("Sudo defaulted to true.")
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Installs Docker binaries from cached path (key '%s') and sets up systemd unit at %s.",
			s.SourceExtractedPathCacheKey, s.SystemdUnitPath)
	}
}

// Precheck determines if Docker seems already installed.
func (s *InstallDockerBinariesStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.SourceExtractedPathCacheKey == "" {
		return false, fmt.Errorf("SourceExtractedPathCacheKey must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	allBinariesExist := true
	if len(s.BinariesToCopy) == 0 { // Should be populated by defaults
	    allBinariesExist = false // Or treat as error if defaults didn't run
	    logger.Warn("BinariesToCopy map is empty, cannot check for binary existence.")
	}
	for _, targetPath := range s.BinariesToCopy {
		exists, err := conn.Exists(ctx.GoContext(), targetPath)
		if err != nil {
			logger.Warn("Failed to check existence of binary, assuming not installed.", "path", targetPath, "error", err)
			return false, nil // Let Run attempt.
		}
		if !exists {
			logger.Info("Target binary does not exist.", "path", targetPath)
			allBinariesExist = false
			break
		}
	}

	unitFileExists := false
	if s.SystemdUnitPath != "" {
		exists, err := conn.Exists(ctx.GoContext(), s.SystemdUnitPath)
		if err != nil {
			logger.Warn("Failed to check existence of systemd unit file, assuming not installed.", "path", s.SystemdUnitPath, "error", err)
			return false, nil // Let Run attempt.
		}
		unitFileExists = exists
		if !unitFileExists {
			logger.Info("Systemd unit file does not exist.", "path", s.SystemdUnitPath)
		}
	} else { // No systemd unit path specified, so don't check for it.
	    unitFileExists = true // Effectively "done" in terms of systemd file.
	}


	if allBinariesExist && unitFileExists {
		logger.Info("All Docker binaries and systemd unit file appear to be installed.")
		// TODO: Optionally check content of systemd unit file against template.
		return true, nil
	}

	logger.Info("Docker installation incomplete (binaries or systemd unit file missing).")
	return false, nil
}

// Run installs Docker binaries and systemd unit file.
func (s *InstallDockerBinariesStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.SourceExtractedPathCacheKey == "" {
		return fmt.Errorf("SourceExtractedPathCacheKey must be specified for %s", s.GetName())
	}

	sourceExtractedPathVal, found := ctx.StepCache().Get(s.SourceExtractedPathCacheKey)
	if !found {
		return fmt.Errorf("extracted Docker path not found in StepCache using key '%s'", s.SourceExtractedPathCacheKey)
	}
	sourceExtractedPath, ok := sourceExtractedPathVal.(string)
	if !ok || sourceExtractedPath == "" {
		return fmt.Errorf("invalid extracted Docker path in StepCache (key '%s')", s.SourceExtractedPathCacheKey)
	}
	logger.Info("Using extracted Docker files from.", "path", sourceExtractedPath)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Copy binaries
	for srcFilename, targetSystemPath := range s.BinariesToCopy {
		fullSourcePath := filepath.Join(sourceExtractedPath, srcFilename) // Assumes binaries are at root of extractedPath
		targetDir := filepath.Dir(targetSystemPath)

		logger.Debug("Ensuring target directory for binary exists.", "path", targetDir)
		mkdirCmd := fmt.Sprintf("mkdir -p %s", targetDir)
		_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
		if errMkdir != nil {
			return fmt.Errorf("failed to create target directory %s for %s (stderr: %s): %w", targetDir, srcFilename, string(stderrMkdir), errMkdir)
		}

		logger.Info("Copying binary.", "source", fullSourcePath, "destination", targetSystemPath)
		cpCmd := fmt.Sprintf("cp -f %s %s", fullSourcePath, targetSystemPath) // -f to overwrite if exists
		_, stderrCp, errCp := conn.Exec(ctx.GoContext(), cpCmd, execOpts)
		if errCp != nil {
			return fmt.Errorf("failed to copy binary %s to %s (stderr: %s): %w", fullSourcePath, targetSystemPath, string(stderrCp), errCp)
		}

		chmodCmd := fmt.Sprintf("chmod +x %s", targetSystemPath)
		_, stderrChmod, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, execOpts)
		if errChmod != nil {
			return fmt.Errorf("failed to set executable permission for %s (stderr: %s): %w", targetSystemPath, string(stderrChmod), errChmod)
		}
		logger.Info("Binary installed and made executable.", "path", targetSystemPath)
	}

	// Install systemd unit file
	if s.SystemdUnitPath != "" && s.SystemdUnitTemplate != "" {
		logger.Info("Preparing and writing Docker systemd unit file.", "path", s.SystemdUnitPath)

		// Determine actual dockerd path for the template
		dockerdInstallPath := s.BinariesToCopy["dockerd"]
		if dockerdInstallPath == "" { // Fallback if not in map or map was empty
		    dockerdInstallPath = "/usr/bin/dockerd" // A common default
		    logger.Warn("Dockerd install path not found in BinariesToCopy map, using default for template.", "path", dockerdInstallPath)
		}

		tmplData := struct{ DockerdPath string }{ DockerdPath: dockerdInstallPath }
		tmpl, errTmpl := template.New("dockerSystemdUnit").Parse(s.SystemdUnitTemplate)
		if errTmpl != nil {
			return fmt.Errorf("failed to parse Docker systemd unit template: %w", errTmpl)
		}
		var unitContent bytes.Buffer
		if errExecute := tmpl.Execute(&unitContent, tmplData); errExecute != nil {
			return fmt.Errorf("failed to render Docker systemd unit template: %w", errExecute)
		}

		systemdUnitDir := filepath.Dir(s.SystemdUnitPath)
		mkdirUnitDirCmd := fmt.Sprintf("mkdir -p %s", systemdUnitDir)
		if _, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirUnitDirCmd, execOpts); errMkdir != nil {
			return fmt.Errorf("failed to create directory for systemd unit %s (stderr: %s): %w", systemdUnitDir, string(stderrMkdir), errMkdir)
		}

		errWrite := conn.CopyContent(ctx.GoContext(), unitContent.String(), s.SystemdUnitPath, connector.FileStat{
			Permissions: "0644", Sudo: s.Sudo,
		})
		if errWrite != nil {
			return fmt.Errorf("failed to write Docker systemd unit file to %s: %w", s.SystemdUnitPath, errWrite)
		}
		logger.Info("Docker systemd unit file written.", "path", s.SystemdUnitPath)

		logger.Info("Reloading systemd daemon.")
		daemonReloadCmd := "systemctl daemon-reload"
		_, stderrReload, errReload := conn.Exec(ctx.GoContext(), daemonReloadCmd, execOpts)
		if errReload != nil {
			// Non-fatal, as service might be started manually or by another step.
			logger.Warn("Failed to reload systemd daemon after writing Docker service unit.", "stderr", string(stderrReload), "error", errReload)
		} else {
			logger.Info("Systemd daemon reloaded successfully.")
		}
	} else {
		logger.Info("Skipping systemd unit file installation as path or template is empty.")
	}

	logger.Info("Docker binaries and systemd unit (if specified) installed successfully.")
	return nil
}

// Rollback removes installed Docker binaries and systemd unit file.
func (s *InstallDockerBinariesStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger) // Ensure paths are populated

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Remove binaries
	for _, targetPath := range s.BinariesToCopy {
		if targetPath == "" { continue }
		logger.Info("Attempting to remove binary.", "path", targetPath)
		rmCmd := fmt.Sprintf("rm -f %s", targetPath)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts)
		if errRm != nil {
			logger.Warn("Failed to remove binary during rollback (best effort).", "path", targetPath, "stderr", string(stderrRm), "error", errRm)
		}
	}

	// Remove systemd unit file
	if s.SystemdUnitPath != "" {
		logger.Info("Attempting to remove systemd unit file.", "path", s.SystemdUnitPath)
		rmCmd := fmt.Sprintf("rm -f %s", s.SystemdUnitPath)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts)
		if errRm != nil {
			logger.Warn("Failed to remove systemd unit file during rollback (best effort).", "path", s.SystemdUnitPath, "stderr", string(stderrRm), "error", errRm)
		} else {
			logger.Info("Systemd unit file removed, attempting daemon-reload.")
			daemonReloadCmd := "systemctl daemon-reload"
			if _, stderrReload, errReload := conn.Exec(ctx.GoContext(), daemonReloadCmd, execOpts); errReload != nil {
				logger.Warn("Failed to reload systemd daemon during rollback.", "stderr", string(stderrReload), "error", errReload)
			}
		}
	}
	logger.Info("Docker binaries and systemd unit file rollback attempt finished.")
	return nil
}

var _ step.Step = (*InstallDockerBinariesStepSpec)(nil)
