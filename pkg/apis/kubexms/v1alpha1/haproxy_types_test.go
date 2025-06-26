package v1alpha1

import (
	"strings"
	"testing"
)

// Helper for HAProxy tests
func pintHAProxy(i int) *int { v := i; return &v }
func pstrHAProxy(s string) *string { v := s; return &v }
func pboolHAProxy(b bool) *bool { v := b; return &v }

func TestSetDefaults_HAProxyConfig(t *testing.T) {
	cfg := &HAProxyConfig{}
	SetDefaults_HAProxyConfig(cfg)

	if cfg.FrontendBindAddress == nil || *cfg.FrontendBindAddress != "0.0.0.0" {t.Error("FrontendBindAddress default failed")}
	if cfg.FrontendPort == nil || *cfg.FrontendPort != 6443 {t.Error("FrontendPort default failed")}
	if cfg.Mode == nil || *cfg.Mode != "tcp" {t.Error("Mode default failed")}
	if cfg.BalanceAlgorithm == nil || *cfg.BalanceAlgorithm != "roundrobin" {t.Error("BalanceAlgorithm default failed")}
	if cfg.BackendServers == nil || cap(cfg.BackendServers) != 0 {t.Error("BackendServers default failed")}
	if cfg.ExtraGlobalConfig == nil || cap(cfg.ExtraGlobalConfig) != 0 {t.Error("ExtraGlobalConfig default failed")}
	// ... similar for ExtraDefaultsConfig, ExtraFrontendConfig, ExtraBackendConfig
	if cfg.ExtraDefaultsConfig == nil || cap(cfg.ExtraDefaultsConfig) != 0 {t.Error("ExtraDefaultsConfig default failed")}
	if cfg.ExtraFrontendConfig == nil || cap(cfg.ExtraFrontendConfig) != 0 {t.Error("ExtraFrontendConfig default failed")}
	if cfg.ExtraBackendConfig == nil || cap(cfg.ExtraBackendConfig) != 0 {t.Error("ExtraBackendConfig default failed")}
	if cfg.SkipInstall == nil || *cfg.SkipInstall != false {t.Error("SkipInstall default failed")}

	cfgWithServer := &HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name:"s1", Address:"a1", Port:1}}}
	SetDefaults_HAProxyConfig(cfgWithServer)
	if cfgWithServer.BackendServers[0].Weight == nil || *cfgWithServer.BackendServers[0].Weight != 1 {
	   t.Error("BackendServer Weight default failed")
	}
}

func TestValidate_HAProxyConfig(t *testing.T) {
	validServer := HAProxyBackendServer{Name: "s1", Address: "1.1.1.1", Port: 8080}
	validCfg := HAProxyConfig{
		FrontendPort:   pintHAProxy(8443),
		BackendServers: []HAProxyBackendServer{validServer},
	}
	SetDefaults_HAProxyConfig(&validCfg)
	verrs := &ValidationErrors{}
	Validate_HAProxyConfig(&validCfg, verrs, "haproxy")
	if !verrs.IsEmpty() {
		t.Errorf("Validation failed for valid config: %v", verrs)
	}

	// Test SkipInstall
	skipInstallCfg := HAProxyConfig{SkipInstall: pboolHAProxy(true)}
	SetDefaults_HAProxyConfig(&skipInstallCfg)
	verrsSkip := &ValidationErrors{}
	Validate_HAProxyConfig(&skipInstallCfg, verrsSkip, "haproxy")
	if !verrsSkip.IsEmpty() {
		t.Errorf("Validation should pass (mostly skipped) if SkipInstall is true: %v", verrsSkip)
	}

	tests := []struct {
		name       string
		cfg        HAProxyConfig
		wantErrMsg string
	}{
	   {"bad_frontend_port", HAProxyConfig{FrontendPort: pintHAProxy(0)}, ".frontendPort: invalid port 0"},
	   {"invalid_mode", HAProxyConfig{Mode: pstrHAProxy("udp")}, ".mode: invalid mode 'udp'"},
	   {"invalid_algo", HAProxyConfig{BalanceAlgorithm: pstrHAProxy("randomest")}, ".balanceAlgorithm: invalid algorithm 'randomest'"},
	   {"no_backend_servers", HAProxyConfig{FrontendPort: pintHAProxy(123)}, ".backendServers: must specify at least one backend server"},
	   {"backend_empty_name", HAProxyConfig{BackendServers:[]HAProxyBackendServer{{Address:"a",Port:1}}}, ".name: backend server name cannot be empty"},
	   {"backend_empty_addr", HAProxyConfig{BackendServers:[]HAProxyBackendServer{{Name:"n",Port:1}}}, ".address: backend server address cannot be empty"},
	   {"backend_bad_port", HAProxyConfig{BackendServers:[]HAProxyBackendServer{{Name:"n",Address:"a",Port:0}}}, ".port: invalid backend server port 0"},
	   {"backend_bad_weight", HAProxyConfig{BackendServers:[]HAProxyBackendServer{{Name:"n",Address:"a",Port:1, Weight: pintHAProxy(-1)}}}, ".weight: cannot be negative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		   if tt.cfg.FrontendPort == nil && !strings.Contains(tt.name, "frontend_port") {tt.cfg.FrontendPort = pintHAProxy(80)}
		   if len(tt.cfg.BackendServers) == 0 && !strings.Contains(tt.name, "backend_servers") {tt.cfg.BackendServers = []HAProxyBackendServer{validServer}}

		   SetDefaults_HAProxyConfig(&tt.cfg)
		   verrs := &ValidationErrors{}
		   Validate_HAProxyConfig(&tt.cfg, verrs, "haproxy")
		   if verrs.IsEmpty() {
			   t.Fatalf("Expected error for %s, got none", tt.name)
		   }
		   if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
			   t.Errorf("Error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
		   }
		})
	}
}
