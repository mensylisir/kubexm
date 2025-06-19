package v1alpha1

import (
	"fmt"
	"strings"
)

const (
	EtcdTypeKubeXMSInternal = "stacked"
	EtcdTypeExternal        = "external"
)

// EtcdConfig defines the configuration for the Etcd cluster.
type EtcdConfig struct {
	Type string `json:"type,omitempty"`
	Version string `json:"version,omitempty"`
	External *ExternalEtcdConfig `json:"external,omitempty"`
	DataDir *string `json:"dataDir,omitempty"`
	ClientPort *int `json:"clientPort,omitempty"`
	PeerPort *int `json:"peerPort,omitempty"`
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
}

// ExternalEtcdConfig describes how to connect to an external etcd cluster.
type ExternalEtcdConfig struct {
	Endpoints []string `json:"endpoints"`
	CAFile string `json:"caFile,omitempty"`
	CertFile string `json:"certFile,omitempty"`
	KeyFile string `json:"keyFile,omitempty"`
}

// SetDefaults_EtcdConfig sets default values for EtcdConfig.
func SetDefaults_EtcdConfig(cfg *EtcdConfig) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		cfg.Type = EtcdTypeKubeXMSInternal
	}
	if cfg.ClientPort == nil {
		defaultPort := 2379
		cfg.ClientPort = &defaultPort
	}
	if cfg.PeerPort == nil {
		defaultPort := 2380
		cfg.PeerPort = &defaultPort
	}
	if cfg.DataDir == nil {
		defaultDataDir := "/var/lib/etcd"
		cfg.DataDir = &defaultDataDir
	}
	if cfg.Type == EtcdTypeExternal && cfg.External == nil {
		cfg.External = &ExternalEtcdConfig{}
	}
	if cfg.External != nil && cfg.External.Endpoints == nil {
	   cfg.External.Endpoints = []string{}
	}
	if cfg.ExtraArgs == nil {
	   cfg.ExtraArgs = make(map[string]string)
	}
}

// Validate_EtcdConfig validates EtcdConfig.
func Validate_EtcdConfig(cfg *EtcdConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validTypes := []string{EtcdTypeKubeXMSInternal, EtcdTypeExternal}
	isValidType := false
	for _, vt := range validTypes {
		if cfg.Type == vt {
			isValidType = true
			break
		}
	}
	if !isValidType {
		verrs.Add("%s.type: invalid type '%s', must be one of %v", pathPrefix, cfg.Type, validTypes)
	}
	if cfg.Type == EtcdTypeExternal {
		if cfg.External == nil {
			verrs.Add("%s.external: must be defined if etcd.type is '%s'", pathPrefix, EtcdTypeExternal)
		} else {
			if len(cfg.External.Endpoints) == 0 {
				verrs.Add("%s.external.endpoints: must contain at least one endpoint if etcd.type is '%s'", pathPrefix, EtcdTypeExternal)
			}
			for i, ep := range cfg.External.Endpoints {
				if strings.TrimSpace(ep) == "" {
					verrs.Add("%s.external.endpoints[%d]: endpoint cannot be empty", pathPrefix, i)
				}
			}
			if (cfg.External.CertFile != "" && cfg.External.KeyFile == "") || (cfg.External.CertFile == "" && cfg.External.KeyFile != "") {
				verrs.Add("%s.external: certFile and keyFile must be specified together for mTLS", pathPrefix)
			}
		}
	}
	if cfg.ClientPort != nil && (*cfg.ClientPort <= 0 || *cfg.ClientPort > 65535) {
	   verrs.Add("%s.clientPort: invalid port %d, must be between 1-65535", pathPrefix, *cfg.ClientPort)
	}
	if cfg.PeerPort != nil && (*cfg.PeerPort <= 0 || *cfg.PeerPort > 65535) {
	   verrs.Add("%s.peerPort: invalid port %d, must be between 1-65535", pathPrefix, *cfg.PeerPort)
	}
	if cfg.DataDir != nil && strings.TrimSpace(*cfg.DataDir) == "" {
		verrs.Add("%s.dataDir: cannot be empty if specified", pathPrefix)
	}
}

func (e *EtcdConfig) GetClientPort() int {
	if e != nil && e.ClientPort != nil { return *e.ClientPort }
	return 2379
}
func (e *EtcdConfig) GetPeerPort() int {
	if e != nil && e.PeerPort != nil { return *e.PeerPort }
	return 2380
}
func (e *EtcdConfig) GetDataDir() string {
   if e != nil && e.DataDir != nil && *e.DataDir != "" { return *e.DataDir }
   return "/var/lib/etcd"
}
