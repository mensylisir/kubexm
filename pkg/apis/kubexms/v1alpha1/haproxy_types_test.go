package v1alpha1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// Helper for HAProxy tests are replaced by global helpers from zz_helpers.go

func TestSetDefaults_HAProxyConfig(t *testing.T) {
	cfg := &HAProxyConfig{}
	SetDefaults_HAProxyConfig(cfg)

	assert.NotNil(t, cfg.FrontendBindAddress)
	assert.Equal(t, "0.0.0.0", *cfg.FrontendBindAddress, "FrontendBindAddress default failed")
	assert.NotNil(t, cfg.FrontendPort)
	assert.Equal(t, 6443, *cfg.FrontendPort, "FrontendPort default failed")
	assert.NotNil(t, cfg.Mode)
	assert.Equal(t, "tcp", *cfg.Mode, "Mode default failed")
	assert.NotNil(t, cfg.BalanceAlgorithm)
	assert.Equal(t, "roundrobin", *cfg.BalanceAlgorithm, "BalanceAlgorithm default failed")

	assert.NotNil(t, cfg.BackendServers, "BackendServers should be initialized")
	assert.Len(t, cfg.BackendServers, 0, "BackendServers should be empty by default")

	assert.NotNil(t, cfg.ExtraGlobalConfig, "ExtraGlobalConfig should be initialized")
	assert.Len(t, cfg.ExtraGlobalConfig, 0, "ExtraGlobalConfig should be empty by default")

	assert.NotNil(t, cfg.ExtraDefaultsConfig, "ExtraDefaultsConfig should be initialized")
	assert.Len(t, cfg.ExtraDefaultsConfig, 0, "ExtraDefaultsConfig should be empty by default")

	assert.NotNil(t, cfg.ExtraFrontendConfig, "ExtraFrontendConfig should be initialized")
	assert.Len(t, cfg.ExtraFrontendConfig, 0, "ExtraFrontendConfig should be empty by default")

	assert.NotNil(t, cfg.ExtraBackendConfig, "ExtraBackendConfig should be initialized")
	assert.Len(t, cfg.ExtraBackendConfig, 0, "ExtraBackendConfig should be empty by default")

	assert.NotNil(t, cfg.SkipInstall)
	assert.False(t, *cfg.SkipInstall, "SkipInstall default failed")

	cfgWithServer := &HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name:"s1", Address:"a1", Port:1}}}
	SetDefaults_HAProxyConfig(cfgWithServer)
	assert.NotNil(t, cfgWithServer.BackendServers[0].Weight)
	assert.Equal(t, 1, *cfgWithServer.BackendServers[0].Weight, "BackendServer Weight default failed")
}

func TestValidate_HAProxyConfig(t *testing.T) {
	validServer := HAProxyBackendServer{Name: "s1", Address: "1.1.1.1", Port: 8080, Weight: intPtr(1)}
	validCfg := HAProxyConfig{
		FrontendBindAddress: stringPtr("0.0.0.0"),
		FrontendPort:        intPtr(8443),
		Mode:                stringPtr("tcp"),
		BalanceAlgorithm:    stringPtr("roundrobin"),
		BackendServers:      []HAProxyBackendServer{validServer},
		SkipInstall:         boolPtr(false),
	}
	verrs := &validation.ValidationErrors{}
	Validate_HAProxyConfig(&validCfg, verrs, "haproxy")
	if verrs.HasErrors() { // Corrected: Should be HasErrors() and expect NO error for validCfg
		t.Errorf("Validation failed for valid config: %v", verrs.Error())
	}

	skipInstallCfg := HAProxyConfig{SkipInstall: boolPtr(true)}
	verrsSkip := &validation.ValidationErrors{}
	Validate_HAProxyConfig(&skipInstallCfg, verrsSkip, "haproxy")
	if verrsSkip.HasErrors() { // Updated to use HasErrors()
		t.Errorf("Validation should pass (mostly skipped) if SkipInstall is true: %v", verrsSkip.Error()) // Updated to use Error()
	}

	tests := []struct {
		name       string
		cfg        HAProxyConfig
		wantErrMsg string
	}{
		{"bad_frontend_port", HAProxyConfig{FrontendPort: intPtr(0), BackendServers: []HAProxyBackendServer{validServer}}, ".frontendPort: invalid port 0"},
		{"invalid_mode", HAProxyConfig{Mode: stringPtr("udp"), BackendServers: []HAProxyBackendServer{validServer}}, ".mode: invalid mode 'udp'"},
		{"invalid_algo", HAProxyConfig{BalanceAlgorithm: stringPtr("randomest"), BackendServers: []HAProxyBackendServer{validServer}}, ".balanceAlgorithm: invalid algorithm 'randomest'"},
		{"no_backend_servers", HAProxyConfig{FrontendPort: intPtr(123), BackendServers: []HAProxyBackendServer{}}, ".backendServers: must specify at least one backend server"},
		{"backend_empty_name", HAProxyConfig{BackendServers: []HAProxyBackendServer{{Address: "valid-addr", Port: 1}}}, ".name: backend server name cannot be empty"},
		{"backend_empty_addr", HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name: "n", Port: 1}}}, ".address: backend server address cannot be empty"},
		{"backend_invalid_addr_format", HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name: "n", Address: "invalid!", Port: 1}}}, ".address: invalid backend server address format 'invalid!'"},
		{"backend_bad_port", HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name: "n", Address: "a", Port: 0}}}, ".port: invalid backend server port 0"},
		{"backend_bad_weight", HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name: "n", Address: "a", Port: 1, Weight: intPtr(-1)}}}, ".weight: cannot be negative"},
		{"frontend_bind_address_empty", HAProxyConfig{FrontendBindAddress: stringPtr(" "), BackendServers: []HAProxyBackendServer{validServer}}, ".frontendBindAddress: cannot be empty if specified"},
		{"frontend_bind_address_invalid_ip", HAProxyConfig{FrontendBindAddress: stringPtr("not-an-ip"), BackendServers: []HAProxyBackendServer{validServer}}, ".frontendBindAddress: invalid IP address format 'not-an-ip'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.FrontendPort == nil && !strings.Contains(tt.wantErrMsg, ".frontendPort") {
				tt.cfg.FrontendPort = intPtr(6443)
			}
			if len(tt.cfg.BackendServers) == 0 && !strings.Contains(tt.wantErrMsg, ".backendServers") && !strings.Contains(tt.wantErrMsg, ".name") && !strings.Contains(tt.wantErrMsg, ".address") && !strings.Contains(tt.wantErrMsg, ".port") && !strings.Contains(tt.wantErrMsg, ".weight") {
				tt.cfg.BackendServers = []HAProxyBackendServer{validServer}
			}

			SetDefaults_HAProxyConfig(&tt.cfg)
			verrs := &validation.ValidationErrors{}
			Validate_HAProxyConfig(&tt.cfg, verrs, "haproxy")

			assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name) // Updated
			assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg) // Updated
		})
	}
}
