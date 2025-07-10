package etcd

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/common" // Added for common constants
)

// EtcdConfigDataCacheKey can be used to pass etcd config data if not directly available.
const EtcdConfigDataCacheKey = "EtcdConfigData"

// EtcdNodeConfigData holds the configuration parameters for a single etcd node, used for templating.
type EtcdNodeConfigData struct {
	Name                   string // https://etcd.io/docs/v3.5/op-guide/configuration/#name
	DataDir                string // https://etcd.io/docs/v3.5/op-guide/configuration/#data-dir
	WalDir                 string // https://etcd.io/docs/v3.5/op-guide/configuration/#wal-dir
	ListenPeerURLs         string // e.g., "https://<node_ip>:2380"
	ListenClientURLs       string // e.g., "https://<node_ip>:2379,https://127.0.0.1:2379"
	InitialAdvertisePeerURLs string // e.g., "https://<node_ip>:2380"
	AdvertiseClientURLs    string // e.g., "https://<node_ip>:2379"
	InitialCluster         string // e.g., "etcd-node1=https://<ip1>:2380,etcd-node2=https://<ip2>:2380"
	InitialClusterToken    string // e.g., "kubexm-etcd-cluster"
	InitialClusterState    string // "new" or "existing"
	ClientCertAuth         bool   // Default true
	ClientAutoTLS          bool   // Default false
	TrustedCAFile          string // Path to ca.pem for client connections
	CertFile               string // Path to server.pem for client connections
	KeyFile                string // Path to server-key.pem for client connections
	PeerClientCertAuth     bool   // Default true
	PeerAutoTLS            bool   // Default false
	PeerTrustedCAFile      string // Path to ca.pem for peer connections
	PeerCertFile           string // Path to peer.pem for peer connections
	PeerKeyFile            string // Path to peer-key.pem for peer connections
	// Add other necessary fields like snapshot-count, auto-compaction-retention etc.
	SnapshotCount            string // e.g., "10000"
	AutoCompactionRetention  string // e.g., "1" for 1 hour
	MaxRequestBytes          string // e.g., "10485760" (10 MiB)
	QuotaBackendBytes        string // e.g., "8589934592" (8 GiB)
}

// GenerateEtcdConfigStep renders the etcd.yaml configuration file on an etcd node.
type GenerateEtcdConfigStep struct {
	meta                spec.StepMeta
	ConfigData          EtcdNodeConfigData // Direct data for this node.
	ConfigDataCacheKey  string             // OR: Cache key to retrieve EtcdNodeConfigData for this host.
	RemoteConfigPath    string             // Path on the target node where etcd.yaml will be written.
	Sudo                bool
	TemplateContent     string // Optional: if not using default, provide custom template content.
}

// NewGenerateEtcdConfigStep creates a new GenerateEtcdConfigStep.
func NewGenerateEtcdConfigStep(instanceName string, config EtcdNodeConfigData, remotePath string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "GenerateEtcdConfigurationFile"
	}
	rPath := remotePath
	if rPath == "" {
		rPath = common.EtcdDefaultConfFile
	}
	return &GenerateEtcdConfigStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Generates etcd.yaml configuration file on the node at %s.", rPath),
		},
		ConfigData:       config, // Config must be provided per-host for this step
		RemoteConfigPath: rPath,
		Sudo:             sudo,
	}
}

// NewGenerateEtcdConfigStepFromCache creates a step that reads config data from cache.
func NewGenerateEtcdConfigStepFromCache(instanceName, cacheKey, remotePath string, sudo bool) step.Step {
	// Similar to above, but sets ConfigDataCacheKey instead of ConfigData
	// ... implementation ...
	name := instanceName
	if name == "" {
		name = "GenerateEtcdConfigurationFileFromCache"
	}
	rPath := remotePath
	if rPath == "" {
		rPath = common.EtcdDefaultConfFile
	}
	return &GenerateEtcdConfigStep{
		meta: spec.StepMeta{Name: name, Description: "Generates etcd.yaml from cached data."},
		ConfigDataCacheKey: cacheKey,
		RemoteConfigPath:   rPath,
		Sudo:               sudo,
	}
}


func (s *GenerateEtcdConfigStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *GenerateEtcdConfigStep) getConfigDataForHost(ctx runtime.StepContext, host connector.Host) (*EtcdNodeConfigData, error) {
	if s.ConfigDataCacheKey != "" {
		configVal, found := ctx.TaskCache().Get(s.ConfigDataCacheKey) // Or a host-specific key: fmt.Sprintf("%s.%s", s.ConfigDataCacheKey, host.GetName())
		if !found {
			return nil, fmt.Errorf("etcd config data not found in cache with key '%s' for host %s", s.ConfigDataCacheKey, host.GetName())
		}
		data, ok := configVal.(EtcdNodeConfigData)
		if !ok {
			return nil, fmt.Errorf("invalid etcd config data type in cache for host %s: expected EtcdNodeConfigData, got %T", host.GetName(), configVal)
		}
		return &data, nil
	}
	// If not using cache key, ConfigData should be set directly for this specific host.
	// This implies the Task creating this step instance would create one per host with specific data.
	if s.ConfigData.Name == "" { // Basic check if ConfigData was populated
		return nil, fmt.Errorf("EtcdNodeConfigData not provided directly and no cache key specified for host %s", host.GetName())
	}
	return &s.ConfigData, nil
}

func (s *GenerateEtcdConfigStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	configData, err := s.getConfigDataForHost(ctx, host)
	if err != nil {
		logger.Error("Failed to get config data for precheck", "error", err)
		return false, err // Cannot proceed if config data is missing/invalid
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.RemoteConfigPath)
	if err != nil {
		logger.Warn("Failed to check for existing etcd config, will attempt generation.", "path", s.RemoteConfigPath, "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("Etcd config file does not exist.", "path", s.RemoteConfigPath)
		return false, nil
	}

	// If file exists, compare its content (or a hash) with what would be generated.
	// This is more robust than just existence.
	logger.Info("Etcd config file exists. Verifying content...")
	currentContentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.RemoteConfigPath)
	if err != nil {
		logger.Warn("Failed to read existing etcd config for comparison, will regenerate.", "path", s.RemoteConfigPath, "error", err)
		return false, nil
	}

	expectedContent, err := s.renderEtcdConfig(configData)
	if err != nil {
		logger.Error("Failed to render expected etcd config for comparison.", "error", err)
		return false, fmt.Errorf("failed to render expected etcd config: %w", err) // Error in template/data
	}

	// Normalize line endings for comparison if necessary (e.g., remove \r)
	currentContent := strings.ReplaceAll(string(currentContentBytes), "\r\n", "\n")
	if strings.TrimSpace(currentContent) == strings.TrimSpace(expectedContent) {
		logger.Info("Existing etcd config content matches expected content.")
		return true, nil
	}

	logger.Info("Existing etcd config content does not match expected content. Regeneration needed.")
	// For debugging:
	// logger.Debug("Current etcd config", "content", currentContent)
	// logger.Debug("Expected etcd config", "content", expectedContent)
	return false, nil
}

func (s *GenerateEtcdConfigStep) renderEtcdConfig(data *EtcdNodeConfigData) (string, error) {
	tmplContent := s.TemplateContent
	if tmplContent == "" {
		tmplContent = defaultEtcdConfigTemplate
	}

	tmpl, err := template.New("etcdConfig").Funcs(sprig.TxtFuncMap()).Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse etcd config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute etcd config template: %w", err)
	}
	return buf.String(), nil
}

func (s *GenerateEtcdConfigStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	configData, err := s.getConfigDataForHost(ctx, host)
	if err != nil {
		return err
	}

	configContent, err := s.renderEtcdConfig(configData)
	if err != nil {
		return err
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	remoteDir := filepath.Dir(s.RemoteConfigPath)
	logger.Info("Ensuring remote directory for etcd config exists.", "path", remoteDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote directory %s for etcd config: %w", remoteDir, err)
	}

	logger.Info("Writing etcd configuration file.", "path", s.RemoteConfigPath)
	// Permissions for etcd.yaml are typically 0600 or 0640.
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(configContent), s.RemoteConfigPath, "0600", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write etcd configuration to %s: %w", s.RemoteConfigPath, err)
	}

	logger.Info("Etcd configuration file generated successfully.")
	return nil
}

func (s *GenerateEtcdConfigStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove etcd configuration file for rollback.", "path", s.RemoteConfigPath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil // Best effort
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.RemoteConfigPath, s.Sudo); err != nil {
		logger.Warn("Failed to remove etcd configuration file during rollback (best effort).", "path", s.RemoteConfigPath, "error", err)
	} else {
		logger.Info("Successfully removed etcd configuration file (if it existed).", "path", s.RemoteConfigPath)
	}
	return nil
}

var _ step.Step = (*GenerateEtcdConfigStep)(nil)

const defaultEtcdConfigTemplate = `name: {{ .Name }}
data-dir: {{ .DataDir }}
{{- if .WalDir }}
wal-dir: {{ .WalDir }}
{{- end }}
listen-peer-urls: {{ .ListenPeerURLs }}
listen-client-urls: {{ .ListenClientURLs }}
initial-advertise-peer-urls: {{ .InitialAdvertisePeerURLs }}
advertise-client-urls: {{ .AdvertiseClientURLs }}
initial-cluster: "{{ .InitialCluster }}"
initial-cluster-token: {{ .InitialClusterToken }}
initial-cluster-state: {{ .InitialClusterState }}
client-transport-security:
  client-cert-auth: {{ .ClientCertAuth | default true }}
  auto-tls: {{ .ClientAutoTLS | default false }}
  trusted-ca-file: {{ .TrustedCAFile }}
  cert-file: {{ .CertFile }}
  key-file: {{ .KeyFile }}
peer-transport-security:
  client-cert-auth: {{ .PeerClientCertAuth | default true }} # For peer, client-cert-auth means it requires incoming peers to have a client cert signed by its CA.
  auto-tls: {{ .PeerAutoTLS | default false }}
  trusted-ca-file: {{ .PeerTrustedCAFile }}
  cert-file: {{ .PeerCertFile }}
  key-file: {{ .PeerKeyFile }}
snapshot-count: {{ .SnapshotCount | default "10000" }}
auto-compaction-retention: "{{ .AutoCompactionRetention | default "1" }}" # Default to 1 hour if not specified
max-request-bytes: {{ .MaxRequestBytes | default "10485760" }} # 10 MiB
quota-backend-bytes: {{ .QuotaBackendBytes | default "8589934592" }} # 8 GiB
# Enable V2 Emulation for older clients if needed, though generally discouraged for new clusters.
# enable-v2: false
# Logging (example, adjust as needed)
# log-level: "info"
# log-outputs: ["stderr"] # Or ["systemd/journal"] if using systemd service
`
