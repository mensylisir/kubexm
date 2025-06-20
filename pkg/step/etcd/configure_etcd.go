package etcd

import (
	"fmt"
	"bytes"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils"
)

// ConfigureEtcdStepSpec defines the parameters for configuring an etcd member.
// This could involve writing a configuration file (e.g., etcd.conf.yaml)
// and/or a systemd unit file.
type ConfigureEtcdStepSpec struct {
	spec.StepMeta // Embed common meta fields

	NodeName            string   `json:"nodeName,omitempty"`            // Name of the etcd node, e.g., "etcd1"
	ConfigFilePath      string   `json:"configFilePath,omitempty"`      // Path to the etcd configuration file (e.g., /etc/etcd/etcd.conf.yaml)
	DataDir             string   `json:"dataDir,omitempty"`             // Path to the etcd data directory
	InitialCluster      string   `json:"initialCluster,omitempty"`      // Comma-separated list of "name=peerURL"
	InitialClusterState string   `json:"initialClusterState,omitempty"` // "new" or "existing"
	ClientPort          int      `json:"clientPort,omitempty"`          // Port for client communication (e.g., 2379)
	PeerPort            int      `json:"peerPort,omitempty"`            // Port for peer communication (e.g., 2380)
	ListenClientURLs    []string `json:"listenClientURLs,omitempty"`    // List of URLs to listen on for client traffic
	ListenPeerURLs      []string `json:"listenPeerURLs,omitempty"`      // List of URLs to listen on for peer traffic
	AdvertiseClientURLs []string `json:"advertiseClientURLs,omitempty"` // List of URLs to advertise to clients
	AdvertisePeerURLs   []string `json:"advertisePeerURLs,omitempty"`   // List of URLs to advertise to peers
	ExtraArgs           []string `json:"extraArgs,omitempty"`           // Extra command-line arguments for etcd process
	SystemdUnitPath     string   `json:"systemdUnitPath,omitempty"`     // Path to write the systemd unit file (e.g., /etc/systemd/system/etcd.service)
	ReloadSystemd       bool     `json:"reloadSystemd,omitempty"`       // Whether to run 'systemctl daemon-reload' after writing files
}

// NewConfigureEtcdStepSpec creates a new ConfigureEtcdStepSpec.
func NewConfigureEtcdStepSpec(
	stepName, nodeName, configFilePath, dataDir, initialCluster, initialClusterState string,
	clientPort, peerPort int,
	listenClientURLs, listenPeerURLs, advertiseClientURLs, advertisePeerURLs, extraArgs []string,
	systemdUnitPath string, reloadSystemd bool,
) *ConfigureEtcdStepSpec {
	if stepName == "" {
		stepName = fmt.Sprintf("Configure etcd node %s", nodeName)
	}
	if configFilePath == "" {
		configFilePath = "/etc/etcd/etcd.conf.yaml" // Default config file path
	}
	if dataDir == "" {
		dataDir = "/var/lib/etcd" // Default data directory
	}
	if systemdUnitPath == "" {
		systemdUnitPath = "/etc/systemd/system/etcd.service"
	}

	return &ConfigureEtcdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        stepName,
			Description: fmt.Sprintf("Configures etcd service for node %s. Config: %s, DataDir: %s, Systemd: %s", nodeName, configFilePath, dataDir, systemdUnitPath),
		},
		NodeName:            nodeName,
		ConfigFilePath:      configFilePath,
		DataDir:             dataDir,
		InitialCluster:      initialCluster,
		InitialClusterState: initialClusterState,
		ClientPort:          clientPort,
		PeerPort:            peerPort,
		ListenClientURLs:    listenClientURLs,
		ListenPeerURLs:      listenPeerURLs,
		AdvertiseClientURLs: advertiseClientURLs,
		AdvertisePeerURLs:   advertisePeerURLs,
		ExtraArgs:           extraArgs,
		SystemdUnitPath:     systemdUnitPath,
		ReloadSystemd:       reloadSystemd,
	}
}

// GetName returns the step's name.
func (s *ConfigureEtcdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
func (s *ConfigureEtcdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec validates and returns the spec.
func (s *ConfigureEtcdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ConfigureEtcdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Name returns the step's name (implementing step.Step).
func (s *ConfigureEtcdStepSpec) Name() string { return s.GetName() }

// Description returns the step's description (implementing step.Step).
func (s *ConfigureEtcdStepSpec) Description() string { return s.GetDescription() }

const etcdSystemdUnitTemplate = `[Unit]
Description=etcd service
Documentation=https://github.com/etcd-io/etcd
After=network.target network-online.target
Wants=network-online.target

[Service]
User=etcd
Type=notify
ExecStart=/usr/local/bin/etcd \
--name={{.NodeName}} \
--data-dir={{.DataDir}} \
{{if .InitialCluster}}--initial-cluster={{.InitialCluster}} \{{end}}
{{if .InitialClusterState}}--initial-cluster-state={{.InitialClusterState}} \{{end}}
{{if .AdvertisePeerURLs}}--advertise-peer-urls={{join .AdvertisePeerURLs ","}} \{{end}}
{{if .ListenPeerURLs}}--listen-peer-urls={{join .ListenPeerURLs ","}} \{{end}}
{{if .AdvertiseClientURLs}}--advertise-client-urls={{join .AdvertiseClientURLs ","}} \{{end}}
{{if .ListenClientURLs}}--listen-client-urls={{join .ListenClientURLs ","}} \{{end}}
{{range .ExtraArgs}}    {{.}} \
{{end}}--initial-cluster-token=etcd-cluster-1
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
`

// Precheck checks if the etcd configuration (systemd unit) seems to be in place.
// A more thorough check would compare the content of the existing file with the desired one.
func (s *ConfigureEtcdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.SystemdUnitPath == "" {
		logger.Debug("SystemdUnitPath is not specified; cannot precheck systemd unit.")
		// Depending on whether ConfigFilePath is used, other checks could be added here.
		return false, nil // Let Run proceed to ensure configuration.
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.SystemdUnitPath)
	if err != nil {
		logger.Warn("Failed to check if systemd unit file exists, will attempt configuration.", "path", s.SystemdUnitPath, "error", err)
		return false, nil
	}

	if exists {
		// TODO: Add content check for the systemd unit file and etcd.conf.yaml if used.
		// For now, if the systemd unit file exists, assume it's correctly configured.
		// This makes the step less idempotent if only content changes.
		logger.Info("Etcd systemd unit file already exists. Assuming configured.", "path", s.SystemdUnitPath)
		// Check data dir as well
		dataDirExists, _ := conn.Exists(ctx.GoContext(), s.DataDir)
		if dataDirExists {
			logger.Info("Etcd data directory also exists.", "path", s.DataDir)
			return true, nil
		}
		logger.Info("Etcd systemd unit exists, but data directory does not. Will re-run configuration.", "dataDir", s.DataDir)
	} else {
		logger.Info("Etcd systemd unit file does not exist. Configuration needed.", "path", s.SystemdUnitPath)
	}

	return false, nil
}

// Run generates and writes the etcd systemd unit file and creates data directories.
func (s *ConfigureEtcdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if s.NodeName == "" || s.DataDir == "" || s.SystemdUnitPath == "" {
		return fmt.Errorf("NodeName, DataDir, and SystemdUnitPath must be specified for step: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	execOptsSudo := &connector.ExecOptions{Sudo: true}

	// Create data directory
	logger.Info("Ensuring etcd data directory exists.", "path", s.DataDir)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", s.DataDir)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOptsSudo)
	if errMkdir != nil {
		return fmt.Errorf("failed to create etcd data directory %s (stderr: %s): %w", s.DataDir, string(stderrMkdir), errMkdir)
	}
	// Set ownership for data directory (e.g., to 'etcd:etcd' user/group if it exists)
	// This might require user/group creation in a separate step.
	// chownCmdDataDir := fmt.Sprintf("chown etcd:etcd %s", s.DataDir)
	// _, _, errChownData := conn.Exec(ctx.GoContext(), chownCmdDataDir, execOptsSudo)
	// if errChownData != nil { logger.Warn("Failed to chown etcd data directory", "error", errChownData)}


	// Render systemd unit content
	tmpl, err := template.New("etcdSystemdUnit").Funcs(template.FuncMap{"join": strings.Join}).Parse(etcdSystemdUnitTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse etcd systemd unit template: %w", err)
	}
	var unitContent bytes.Buffer
	if err := tmpl.Execute(&unitContent, s); err != nil {
		return fmt.Errorf("failed to execute etcd systemd unit template: %w", err)
	}

	logger.Info("Writing etcd systemd unit file.", "path", s.SystemdUnitPath)
	// Ensure directory for systemd unit file exists
	systemdUnitDir := filepath.Dir(s.SystemdUnitPath)
	_, stderrMkdirUnit, errMkdirUnit := conn.Exec(ctx.GoContext(), fmt.Sprintf("mkdir -p %s", systemdUnitDir), execOptsSudo)
	if errMkdirUnit != nil {
		return fmt.Errorf("failed to create directory for systemd unit %s (stderr: %s): %w", systemdUnitDir, string(stderrMkdirUnit), errMkdirUnit)
	}

	err = conn.CopyContent(ctx.GoContext(), unitContent.String(), s.SystemdUnitPath, connector.FileStat{
		Permissions: "0644", // Standard systemd unit permissions
		Sudo:        true,     // Writing to /etc/systemd/system requires sudo
	})
	if err != nil {
		return fmt.Errorf("failed to write etcd systemd unit file to %s: %w", s.SystemdUnitPath, err)
	}

	if s.ReloadSystemd {
		logger.Info("Reloading systemd daemon.")
		reloadCmd := "systemctl daemon-reload"
		_, stderrReload, errReload := conn.Exec(ctx.GoContext(), reloadCmd, execOptsSudo)
		if errReload != nil {
			return fmt.Errorf("failed to reload systemd daemon (stderr: %s): %w", string(stderrReload), errReload)
		}
		logger.Info("Systemd daemon reloaded successfully.")
	}

	// Note: Writing etcd.conf.yaml is omitted for brevity but would involve similar templating/CopyContent logic
	// if s.ConfigFilePath was used.

	logger.Info("Etcd configured successfully.")
	return nil
}

// Rollback removes the etcd systemd unit file and potentially the data directory.
func (s *ConfigureEtcdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	execOptsSudo := &connector.ExecOptions{Sudo: true}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	if s.SystemdUnitPath != "" {
		logger.Info("Attempting to remove etcd systemd unit file.", "path", s.SystemdUnitPath)
		rmCmd := fmt.Sprintf("rm -f %s", s.SystemdUnitPath)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOptsSudo)
		if errRm != nil {
			logger.Error("Failed to remove etcd systemd unit file (best effort).", "path", s.SystemdUnitPath, "stderr", string(stderrRm), "error", errRm)
		} else {
			logger.Info("Etcd systemd unit file removed.", "path", s.SystemdUnitPath)
			if s.ReloadSystemd { // Reload daemon after removing unit file
				logger.Info("Reloading systemd daemon after removing unit file.")
				reloadCmd := "systemctl daemon-reload"
				_, stderrReload, errReload := conn.Exec(ctx.GoContext(), reloadCmd, execOptsSudo)
				if errReload != nil {
					logger.Error("Failed to reload systemd daemon during rollback (best effort).", "stderr", string(stderrReload), "error", errReload)
				}
			}
		}
	}

	// Optionally, remove DataDir. This is often risky if it contains valuable data.
	// Add a specific flag like `RemoveDataDirOnRollback bool` to the spec if this is desired.
	// For now, we'll skip removing DataDir.
	// if s.DataDir != "" && s.RemoveDataDirOnRollback {
	//    logger.Info("Attempting to remove etcd data directory.", "path", s.DataDir)
	//    rmDataCmd := fmt.Sprintf("rm -rf %s", s.DataDir)
	//    // ... exec ...
	// }

	logger.Info("Etcd configuration rollback attempt finished.")
	return nil
}

var _ step.Step = (*ConfigureEtcdStepSpec)(nil)
