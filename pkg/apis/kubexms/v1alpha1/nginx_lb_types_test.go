package v1alpha1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Local helpers removed, using global ones from zz_helpers.go

func TestSetDefaults_NginxLBConfig(t *testing.T) {
	cfg := &NginxLBConfig{}
	SetDefaults_NginxLBConfig(cfg)

	assert.NotNil(t, cfg.ListenAddress)
	assert.Equal(t, "0.0.0.0", *cfg.ListenAddress)
	assert.NotNil(t, cfg.ListenPort)
	assert.Equal(t, 6443, *cfg.ListenPort)
	assert.NotNil(t, cfg.Mode)
	assert.Equal(t, "tcp", *cfg.Mode)

	assert.NotNil(t, cfg.UpstreamServers)
	assert.Len(t, cfg.UpstreamServers, 0)
	assert.NotNil(t, cfg.ExtraHTTPConfig)
	assert.Len(t, cfg.ExtraHTTPConfig, 0)
	assert.NotNil(t, cfg.ExtraStreamConfig)
	assert.Len(t, cfg.ExtraStreamConfig, 0)
	assert.NotNil(t, cfg.ExtraServerConfig)
	assert.Len(t, cfg.ExtraServerConfig, 0)

	assert.NotNil(t, cfg.ConfigFilePath)
	assert.Equal(t, "/etc/nginx/nginx.conf", *cfg.ConfigFilePath)
	assert.NotNil(t, cfg.SkipInstall)
	assert.False(t, *cfg.SkipInstall)

	cfgWithServer := &NginxLBConfig{UpstreamServers: []NginxLBUpstreamServer{{Address: "a:1"}}}
	SetDefaults_NginxLBConfig(cfgWithServer)
	assert.NotNil(t, cfgWithServer.UpstreamServers[0].Weight)
	assert.Equal(t, 1, *cfgWithServer.UpstreamServers[0].Weight)
}

func TestValidate_NginxLBConfig(t *testing.T) {
	validServer := NginxLBUpstreamServer{Address: "1.1.1.1:8080", Weight: intPtr(1)}
	validCfg := NginxLBConfig{
		ListenAddress: stringPtr("0.0.0.0"),
		ListenPort:    intPtr(8443),
		Mode:          stringPtr("tcp"),
		UpstreamServers: []NginxLBUpstreamServer{validServer},
		ConfigFilePath: stringPtr("/etc/nginx/nginx.conf"),
		SkipInstall:   boolPtr(false),
	}
	// SetDefaults_NginxLBConfig(&validCfg) // Values are set to defaults or test values
	verrs := &ValidationErrors{}
	Validate_NginxLBConfig(&validCfg, verrs, "nginxLB")
	assert.True(t, verrs.IsEmpty(), "Validation failed for valid config: %v", verrs.Error())

	skipInstallCfg := NginxLBConfig{SkipInstall: boolPtr(true)}
	// SetDefaults_NginxLBConfig(&skipInstallCfg) // SkipInstall short-circuits validation
	verrsSkip := &ValidationErrors{}
	Validate_NginxLBConfig(&skipInstallCfg, verrsSkip, "nginxLB")
	assert.True(t, verrsSkip.IsEmpty(), "Validation should pass if SkipInstall is true: %v", verrsSkip.Error())

	tests := []struct {
		name       string
		cfg        NginxLBConfig
		wantErrMsg string
	}{
		{"bad_listen_port", NginxLBConfig{ListenPort: intPtr(0), UpstreamServers: []NginxLBUpstreamServer{validServer}}, ".listenPort: invalid port 0"},
		{"invalid_mode", NginxLBConfig{Mode: stringPtr("udp"), UpstreamServers: []NginxLBUpstreamServer{validServer}}, ".mode: invalid mode 'udp'"},
		{"invalid_algo", NginxLBConfig{BalanceAlgorithm: stringPtr("foo"), UpstreamServers: []NginxLBUpstreamServer{validServer}}, ".balanceAlgorithm: invalid algorithm 'foo'"},
		{"no_upstreams", NginxLBConfig{ListenPort: intPtr(123)}, ".upstreamServers: must specify at least one upstream server"},
		{"upstream_empty_addr", NginxLBConfig{UpstreamServers: []NginxLBUpstreamServer{{Address: " "}}}, ".address: upstream server address cannot be empty"},
		{"upstream_bad_addr_format_no_port", NginxLBConfig{UpstreamServers: []NginxLBUpstreamServer{{Address: "hostonly"}}}, "upstream server address 'hostonly' must be in 'host:port' format"},
		{"upstream_bad_addr_format_bad_host", NginxLBConfig{UpstreamServers: []NginxLBUpstreamServer{{Address: "inva!id:80"}}}, ".address: host part 'inva!id' of upstream server address 'inva!id:80' is not a valid host or IP"},
		{"upstream_bad_addr_format_bad_port_str", NginxLBConfig{UpstreamServers: []NginxLBUpstreamServer{{Address: "validhost:port"}}}, ".address: port part 'port' of upstream server address 'validhost:port' is not a valid port number"},
		{"upstream_bad_addr_format_bad_port_num", NginxLBConfig{UpstreamServers: []NginxLBUpstreamServer{{Address: "validhost:70000"}}}, ".address: port part '70000' of upstream server address 'validhost:70000' is not a valid port number"},
		{"upstream_bad_weight", NginxLBConfig{UpstreamServers: []NginxLBUpstreamServer{{Address: "h:1", Weight: intPtr(-1)}}}, ".weight: cannot be negative"},
		{"empty_config_path", NginxLBConfig{ConfigFilePath: stringPtr(" "), UpstreamServers: []NginxLBUpstreamServer{validServer}}, ".configFilePath: cannot be empty if specified"},
		{"listen_address_empty", NginxLBConfig{ListenAddress: stringPtr(" "), UpstreamServers: []NginxLBUpstreamServer{validServer}}, ".listenAddress: cannot be empty if specified"},
		{"listen_address_invalid_ip", NginxLBConfig{ListenAddress: stringPtr("not-an-ip"), UpstreamServers: []NginxLBUpstreamServer{validServer}}, ".listenAddress: invalid IP address format 'not-an-ip'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure necessary fields that are not part of the specific test case logic are valid or defaulted
			if tt.cfg.ListenPort == nil && !strings.Contains(tt.wantErrMsg, ".listenPort") {
				tt.cfg.ListenPort = intPtr(6443) // Default valid port
			}
			if len(tt.cfg.UpstreamServers) == 0 && !strings.Contains(tt.wantErrMsg, ".upstreamServers") && !strings.Contains(tt.wantErrMsg, "server address") && !strings.Contains(tt.wantErrMsg, ".weight") {
				tt.cfg.UpstreamServers = []NginxLBUpstreamServer{validServer}
			}
			if tt.cfg.ConfigFilePath == nil && !strings.Contains(tt.wantErrMsg, ".configFilePath") {
				tt.cfg.ConfigFilePath = stringPtr("/etc/nginx/nginx.conf")
			}
			if tt.cfg.Mode == nil && !strings.Contains(tt.wantErrMsg, ".mode") {
				tt.cfg.Mode = stringPtr("tcp")
			}


			SetDefaults_NginxLBConfig(&tt.cfg)
			verrs := &ValidationErrors{}
			Validate_NginxLBConfig(&tt.cfg, verrs, "nginxLB")

			assert.False(t, verrs.IsEmpty(), "Expected error for %s, got none", tt.name)
			assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
		})
	}
}
