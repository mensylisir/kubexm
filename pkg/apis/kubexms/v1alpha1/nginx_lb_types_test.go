package v1alpha1

import (
	"strings"
	"testing"
)
// Helper for NginxLB tests
func pintNginxLB(i int) *int { v := i; return &v }
func pstrNginxLB(s string) *string { v := s; return &v }
func pboolNginxLB(b bool) *bool { v := b; return &v }


func TestSetDefaults_NginxLBConfig(t *testing.T) {
	cfg := &NginxLBConfig{}
	SetDefaults_NginxLBConfig(cfg)

	if cfg.ListenAddress == nil || *cfg.ListenAddress != "0.0.0.0" {t.Error("ListenAddress default failed")}
	if cfg.ListenPort == nil || *cfg.ListenPort != 6443 {t.Error("ListenPort default failed")}
	if cfg.Mode == nil || *cfg.Mode != "tcp" {t.Error("Mode default failed")}
	if cfg.UpstreamServers == nil || cap(cfg.UpstreamServers) != 0 {t.Error("UpstreamServers default failed")}
	if cfg.ExtraHTTPConfig == nil || cap(cfg.ExtraHTTPConfig) != 0 {t.Error("ExtraHTTPConfig default failed")}
	if cfg.ExtraStreamConfig == nil || cap(cfg.ExtraStreamConfig) != 0 {t.Error("ExtraStreamConfig default failed")}
	if cfg.ExtraServerConfig == nil || cap(cfg.ExtraServerConfig) != 0 {t.Error("ExtraServerConfig default failed")}
	if cfg.ConfigFilePath == nil || *cfg.ConfigFilePath != "/etc/nginx/nginx.conf" {t.Error("ConfigFilePath default failed")}
	if cfg.SkipInstall == nil || *cfg.SkipInstall != false {t.Error("SkipInstall default failed")}

	cfgWithServer := &NginxLBConfig{UpstreamServers: []NginxLBUpstreamServer{{Address:"a:1"}}}
	SetDefaults_NginxLBConfig(cfgWithServer)
	if cfgWithServer.UpstreamServers[0].Weight == nil || *cfgWithServer.UpstreamServers[0].Weight != 1 {
	   t.Error("UpstreamServer Weight default failed")
	}
}

func TestValidate_NginxLBConfig(t *testing.T) {
	validServer := NginxLBUpstreamServer{Address: "1.1.1.1:8080"}
	validCfg := NginxLBConfig{
		ListenPort:   pintNginxLB(8443),
		UpstreamServers: []NginxLBUpstreamServer{validServer},
	}
	SetDefaults_NginxLBConfig(&validCfg)
	verrs := &ValidationErrors{}
	Validate_NginxLBConfig(&validCfg, verrs, "nginxLB")
	if !verrs.IsEmpty() {
		t.Errorf("Validation failed for valid config: %v", verrs)
	}

	skipInstallCfg := NginxLBConfig{SkipInstall: pboolNginxLB(true)}
	SetDefaults_NginxLBConfig(&skipInstallCfg)
	verrsSkip := &ValidationErrors{}
	Validate_NginxLBConfig(&skipInstallCfg, verrsSkip, "nginxLB")
	if !verrsSkip.IsEmpty() {
		t.Errorf("Validation should pass (mostly skipped) if SkipInstall is true: %v", verrsSkip)
	}

	tests := []struct {
		name       string
		cfg        NginxLBConfig
		wantErrMsg string
	}{
	   {"nil_listen_port", NginxLBConfig{UpstreamServers:[]NginxLBUpstreamServer{validServer}}, ".listenPort: must be specified"},
	   {"bad_listen_port", NginxLBConfig{ListenPort: pintNginxLB(0)}, ".listenPort: invalid port 0"},
	   {"invalid_mode", NginxLBConfig{Mode: pstrNginxLB("udp")}, ".mode: invalid mode 'udp'"},
	   {"invalid_algo", NginxLBConfig{BalanceAlgorithm: pstrNginxLB("foo")}, ".balanceAlgorithm: invalid algorithm 'foo'"},
	   {"no_upstreams", NginxLBConfig{ListenPort:pintNginxLB(123)}, ".upstreamServers: must specify at least one upstream server"},
	   {"upstream_empty_addr", NginxLBConfig{UpstreamServers:[]NginxLBUpstreamServer{{Address:" "}}}, ".address: upstream server address cannot be empty"},
	   {"upstream_bad_addr_format", NginxLBConfig{UpstreamServers:[]NginxLBUpstreamServer{{Address:"hostonly"}}}, ".address: upstream server address 'hostonly' must be in 'host:port' format"},
	   {"upstream_bad_weight", NginxLBConfig{UpstreamServers:[]NginxLBUpstreamServer{{Address:"h:1",Weight:pintNginxLB(-1)}}}, ".weight: cannot be negative"},
	   {"empty_config_path", NginxLBConfig{ConfigFilePath: pstrNginxLB(" ")}, ".configFilePath: cannot be empty if specified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		   if tt.cfg.ListenPort == nil && !strings.Contains(tt.name, "listen_port") {tt.cfg.ListenPort = pintNginxLB(80)}
		   if len(tt.cfg.UpstreamServers) == 0 && !strings.Contains(tt.name, "upstreams") {tt.cfg.UpstreamServers = []NginxLBUpstreamServer{validServer}}

		   SetDefaults_NginxLBConfig(&tt.cfg)
		   verrs := &ValidationErrors{}
		   Validate_NginxLBConfig(&tt.cfg, verrs, "nginxLB")
		   if verrs.IsEmpty() {
			   t.Fatalf("Expected error for %s, got none", tt.name)
		   }
		   if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
			   t.Errorf("Error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
		   }
		})
	}
}
