package cri_dockerd

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
	DefaultCriDockerdServiceUnitPath = "/etc/systemd/system/cri-docker.service"
	DefaultCriDockerdSocketUnitPath  = "/etc/systemd/system/cri-docker.socket"
	DefaultCriDockerdBinaryCacheKey  = "CriDockerdExtractedBinaryPath" // Expected key from previous step

	DefaultCriDockerdServiceTemplate = `[Unit]
Description=CRI Interface for Docker Application Container Engine
Documentation=https://docs.mirantis.com
After=network-online.target firewalld.service docker.socket
Wants=network-online.target
Requires=docker.socket

[Service]
Type=notify
ExecStart={{.CriDockerdBinaryPath}} --container-runtime-endpoint fd:// --network-plugin={{.NetworkPlugin}} {{if .CNIBinDir}}--cni-bin-dir={{.CNIBinDir}}{{end}} {{if .CNIConfDir}}--cni-conf-dir={{.CNIConfDir}}{{end}} --pod-infra-container-image={{.PodInfraContainerImage}} {{.ExtraArgsJoined}}
ExecReload=/bin/kill -s HUP $MAINPID
TimeoutSec=0
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

	DefaultCriDockerdSocketTemplate = `[Unit]
Description=CRI Docker Socket for the API
PartOf=cri-docker.service

[Socket]
ListenStream=/var/run/cri-dockerd.sock
SocketMode=0660
SocketUser=root
SocketGroup=docker

[Install]
WantedBy=sockets.target
`
)

// SetupCriDockerdServiceStepSpec defines parameters for setting up cri-dockerd systemd service and socket.
type SetupCriDockerdServiceStepSpec struct {
	spec.StepMeta `json:",inline"`

	CriDockerdBinaryPathCacheKey string   `json:"criDockerdBinaryPathCacheKey,omitempty"` // Required
	SystemdServiceUnitPath     string   `json:"systemdServiceUnitPath,omitempty"`
	SystemdSocketUnitPath      string   `json:"systemdSocketUnitPath,omitempty"`
	ServiceUnitTemplate        string   `json:"serviceUnitTemplate,omitempty"`
	SocketUnitTemplate         string   `json:"socketUnitTemplate,omitempty"`
	PodInfraContainerImage     string   `json:"podInfraContainerImage,omitempty"`
	NetworkPlugin              string   `json:"networkPlugin,omitempty"`
	CNIBinDir                  string   `json:"cniBinDir,omitempty"`
	CNIConfDir                 string   `json:"cniConfDir,omitempty"`
	ExtraArgs                  []string `json:"extraArgs,omitempty"`
	Sudo                       bool     `json:"sudo,omitempty"`
}

// NewSetupCriDockerdServiceStepSpec creates a new SetupCriDockerdServiceStepSpec.
func NewSetupCriDockerdServiceStepSpec(name, description, criDockerdBinaryPathCacheKey string) *SetupCriDockerdServiceStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Setup cri-dockerd Systemd Service"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if criDockerdBinaryPathCacheKey == "" {
		// This is required.
	}

	return &SetupCriDockerdServiceStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		CriDockerdBinaryPathCacheKey: criDockerdBinaryPathCacheKey,
		// Defaults in populateDefaults
	}
}

// Name returns the step's name.
func (s *SetupCriDockerdServiceStepSpec) Name() string { return s.StepMeta.Name }
// Description returns the step's description.
func (s *SetupCriDockerdServiceStepSpec) Description() string { return s.StepMeta.Description }
// GetName returns the step's name for spec interface.
func (s *SetupCriDockerdServiceStepSpec) GetName() string { return s.StepMeta.Name }
// GetDescription returns the step's description for spec interface.
func (s *SetupCriDockerdServiceStepSpec) GetDescription() string { return s.StepMeta.Description }
// GetSpec returns the spec itself.
func (s *SetupCriDockerdServiceStepSpec) GetSpec() interface{} { return s }
// Meta returns the step's metadata.
func (s *SetupCriDockerdServiceStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

type criDockerdServiceTemplateData struct {
	CriDockerdBinaryPath   string
	PodInfraContainerImage string
	NetworkPlugin          string
	CNIBinDir              string
	CNIConfDir             string
	ExtraArgsJoined        string
}

func (s *SetupCriDockerdServiceStepSpec) populateDefaults(logger runtime.Logger) {
	if s.CriDockerdBinaryPathCacheKey == "" {
		s.CriDockerdBinaryPathCacheKey = DefaultCriDockerdBinaryCacheKey
		logger.Debug("CriDockerdBinaryPathCacheKey defaulted.", "key", s.CriDockerdBinaryPathCacheKey)
	}
	if s.SystemdServiceUnitPath == "" {
		s.SystemdServiceUnitPath = DefaultCriDockerdServiceUnitPath
		logger.Debug("SystemdServiceUnitPath defaulted.", "path", s.SystemdServiceUnitPath)
	}
	if s.SystemdSocketUnitPath == "" {
		s.SystemdSocketUnitPath = DefaultCriDockerdSocketUnitPath
		logger.Debug("SystemdSocketUnitPath defaulted.", "path", s.SystemdSocketUnitPath)
	}
	if s.ServiceUnitTemplate == "" {
		s.ServiceUnitTemplate = DefaultCriDockerdServiceTemplate
		logger.Debug("ServiceUnitTemplate defaulted.")
	}
	if s.SocketUnitTemplate == "" {
		s.SocketUnitTemplate = DefaultCriDockerdSocketTemplate
		logger.Debug("SocketUnitTemplate defaulted.")
	}
	if s.PodInfraContainerImage == "" {
		s.PodInfraContainerImage = "registry.k8s.io/pause:3.9" // Common default
		logger.Debug("PodInfraContainerImage defaulted.", "image", s.PodInfraContainerImage)
	}
	if s.NetworkPlugin == "" {
		s.NetworkPlugin = "cni"
		logger.Debug("NetworkPlugin defaulted to 'cni'.")
	}
	if s.CNIBinDir == "" {
		s.CNIBinDir = "/opt/cni/bin"
		logger.Debug("CNIBinDir defaulted.", "dir", s.CNIBinDir)
	}
	if s.CNIConfDir == "" {
		s.CNIConfDir = "/etc/cni/net.d"
		logger.Debug("CNIConfDir defaulted.", "dir", s.CNIConfDir)
	}
	if !s.Sudo { // Default Sudo to true
		s.Sudo = true
		logger.Debug("Sudo defaulted to true.")
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Sets up cri-dockerd systemd service at %s and socket at %s.",
			s.SystemdServiceUnitPath, s.SystemdSocketUnitPath)
	}
}

// Precheck determines if the cri-dockerd service and socket files seem correctly configured.
func (s *SetupCriDockerdServiceStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.CriDockerdBinaryPathCacheKey == "" {
		return false, fmt.Errorf("CriDockerdBinaryPathCacheKey must be specified for %s", s.GetName())
	}
	criDockerdBinaryPathVal, found := ctx.StepCache().Get(s.CriDockerdBinaryPathCacheKey)
	if !found {
		logger.Info("cri-dockerd binary path not found in cache. Setup cannot proceed.", "key", s.CriDockerdBinaryPathCacheKey)
		return false, fmt.Errorf("cri-dockerd binary path not found in StepCache with key %s", s.CriDockerdBinaryPathCacheKey)
	}
	criDockerdBinaryPath, ok := criDockerdBinaryPathVal.(string)
	if !ok || criDockerdBinaryPath == "" {
		return false, fmt.Errorf("cached cri-dockerd binary path (key %s) is invalid", s.CriDockerdBinaryPathCacheKey)
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	serviceFileExists, _ := conn.Exists(ctx.GoContext(), s.SystemdServiceUnitPath)
	socketFileExists, _ := conn.Exists(ctx.GoContext(), s.SystemdSocketUnitPath)

	if serviceFileExists && socketFileExists {
		// Optional: Content check for more robust idempotency
		logger.Info("cri-dockerd systemd service and socket files already exist. Assuming configured.",
			"servicePath", s.SystemdServiceUnitPath, "socketPath", s.SystemdSocketUnitPath)
		return true, nil
	}

	logger.Info("cri-dockerd systemd service or socket file does not exist. Setup needed.",
	    "serviceExists", serviceFileExists, "socketExists", socketFileExists)
	return false, nil
}

// Run sets up the cri-dockerd systemd service and socket.
func (s *SetupCriDockerdServiceStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.CriDockerdBinaryPathCacheKey == "" {
		return fmt.Errorf("CriDockerdBinaryPathCacheKey must be specified for %s", s.GetName())
	}
	criDockerdBinaryPathVal, found := ctx.StepCache().Get(s.CriDockerdBinaryPathCacheKey)
	if !found {
		return fmt.Errorf("cri-dockerd binary path not found in StepCache using key '%s'", s.CriDockerdBinaryPathCacheKey)
	}
	criDockerdBinaryPath, ok := criDockerdBinaryPathVal.(string)
	if !ok || criDockerdBinaryPath == "" {
		return fmt.Errorf("invalid cri-dockerd binary path in StepCache (key '%s')", s.CriDockerdBinaryPathCacheKey)
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Prepare template data
	templateData := criDockerdServiceTemplateData{
		CriDockerdBinaryPath:   criDockerdBinaryPath,
		PodInfraContainerImage: s.PodInfraContainerImage,
		NetworkPlugin:          s.NetworkPlugin,
		CNIBinDir:              s.CNIBinDir,
		CNIConfDir:             s.CNIConfDir,
		ExtraArgsJoined:        strings.Join(s.ExtraArgs, " "),
	}

	// Render and write Service Unit File
	if s.SystemdServiceUnitPath != "" && s.ServiceUnitTemplate != "" {
		serviceTmpl, errTmpl := template.New("criDockerdServiceUnit").Parse(s.ServiceUnitTemplate)
		if errTmpl != nil {
			return fmt.Errorf("failed to parse cri-dockerd service unit template: %w", errTmpl)
		}
		var serviceBuf bytes.Buffer
		if errExecute := serviceTmpl.Execute(&serviceBuf, templateData); errExecute != nil {
			return fmt.Errorf("failed to render cri-dockerd service unit template: %w", errExecute)
		}

		unitDir := filepath.Dir(s.SystemdServiceUnitPath)
		if _, _, errMk := conn.Exec(ctx.GoContext(), fmt.Sprintf("mkdir -p %s", unitDir), execOpts); errMk != nil {
		    return fmt.Errorf("failed to create directory for systemd service unit %s: %w", unitDir, errMk)
		}

		errWrite := conn.CopyContent(ctx.GoContext(), serviceBuf.String(), s.SystemdServiceUnitPath, connector.FileStat{Permissions: "0644", Sudo: s.Sudo})
		if errWrite != nil {
			return fmt.Errorf("failed to write cri-dockerd service unit file to %s: %w", s.SystemdServiceUnitPath, errWrite)
		}
		logger.Info("cri-dockerd systemd service unit file written.", "path", s.SystemdServiceUnitPath)
	}

	// Render and write Socket Unit File
	if s.SystemdSocketUnitPath != "" && s.SocketUnitTemplate != "" {
		// Socket template usually doesn't need extensive data, but pass it anyway if it evolves.
		socketTmpl, errTmpl := template.New("criDockerdSocketUnit").Parse(s.SocketUnitTemplate)
		if errTmpl != nil {
			return fmt.Errorf("failed to parse cri-dockerd socket unit template: %w", errTmpl)
		}
		var socketBuf bytes.Buffer
		if errExecute := socketTmpl.Execute(&socketBuf, templateData); errExecute != nil { // Pass same data, though socket might not use it
			return fmt.Errorf("failed to render cri-dockerd socket unit template: %w", errExecute)
		}

		socketUnitDir := filepath.Dir(s.SystemdSocketUnitPath)
		if _, _, errMk := conn.Exec(ctx.GoContext(), fmt.Sprintf("mkdir -p %s", socketUnitDir), execOpts); errMk != nil {
		    return fmt.Errorf("failed to create directory for systemd socket unit %s: %w", socketUnitDir, errMk)
		}

		errWrite := conn.CopyContent(ctx.GoContext(), socketBuf.String(), s.SystemdSocketUnitPath, connector.FileStat{Permissions: "0644", Sudo: s.Sudo})
		if errWrite != nil {
			return fmt.Errorf("failed to write cri-dockerd socket unit file to %s: %w", s.SystemdSocketUnitPath, errWrite)
		}
		logger.Info("cri-dockerd systemd socket unit file written.", "path", s.SystemdSocketUnitPath)
	}

	logger.Info("Reloading systemd daemon.")
	daemonReloadCmd := "systemctl daemon-reload"
	_, stderrReload, errReload := conn.Exec(ctx.GoContext(), daemonReloadCmd, execOpts)
	if errReload != nil {
		logger.Warn("Failed to reload systemd daemon after writing unit files.", "stderr", string(stderrReload), "error", errReload)
		// This might not be fatal, user might reload manually or another step might do it.
	} else {
		logger.Info("Systemd daemon reloaded successfully.")
	}

	// Optional: Enable and start the socket/service
	// This might be a separate ManageCriDockerdServiceStepSpec or done by user.
	// Example:
	// enableCmd := fmt.Sprintf("systemctl enable --now %s", filepath.Base(s.SystemdSocketUnitPath)) // Usually enable socket
	// if _, _, errEnable := conn.Exec(ctx.GoContext(), enableCmd, execOpts); errEnable != nil {
	//    logger.Warn("Failed to enable cri-dockerd socket.", "error", errEnable)
	// } else {
	//    logger.Info("cri-dockerd socket enabled and started.")
	// }

	logger.Info("cri-dockerd systemd service and socket setup completed.")
	return nil
}

// Rollback removes the systemd unit and socket files for cri-dockerd.
func (s *SetupCriDockerdServiceStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger) // Ensure paths are populated

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}
	filesRemoved := false

	for _, path := range []string{s.SystemdServiceUnitPath, s.SystemdSocketUnitPath} {
		if path == "" { continue }
		logger.Info("Attempting to remove systemd unit file.", "path", path)
		rmCmd := fmt.Sprintf("rm -f %s", path)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts)
		if errRm != nil {
			logger.Warn("Failed to remove systemd unit file during rollback (best effort).", "path", path, "stderr", string(stderrRm), "error", errRm)
		} else {
			logger.Info("Systemd unit file removed.", "path", path)
			filesRemoved = true
		}
	}

	if filesRemoved {
		logger.Info("Reloading systemd daemon after removing unit files.")
		daemonReloadCmd := "systemctl daemon-reload"
		if _, stderrReload, errReload := conn.Exec(ctx.GoContext(), daemonReloadCmd, execOpts); errReload != nil {
			logger.Warn("Failed to reload systemd daemon during rollback.", "stderr", string(stderrReload), "error", errReload)
		}
	}
	logger.Info("Rollback attempt for cri-dockerd service setup finished.")
	return nil
}

var _ step.Step = (*SetupCriDockerdServiceStepSpec)(nil)
