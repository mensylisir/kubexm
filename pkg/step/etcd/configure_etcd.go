package etcd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/spec"
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
