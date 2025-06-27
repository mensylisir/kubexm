package v1alpha1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
	validServer := HAProxyBackendServer{Name: "s1", Address: "1.1.1.1", Port: 8080, Weight: intPtr(1)} // Defaulted weight
	validCfg := HAProxyConfig{
		FrontendBindAddress: stringPtr("0.0.0.0"), // Defaulted
		FrontendPort:        intPtr(8443),
		Mode:                stringPtr("tcp"), // Defaulted
		BalanceAlgorithm:    stringPtr("roundrobin"), // Defaulted
		BackendServers:      []HAProxyBackendServer{validServer},
		SkipInstall:         boolPtr(false), // Defaulted
	}
	// SetDefaults_HAProxyConfig(&validCfg) // Not strictly needed here as fields are explicitly set to expected defaults or test values
	verrs := &ValidationErrors{}
	Validate_HAProxyConfig(&validCfg, verrs, "haproxy")
	if !verrs.IsEmpty() {
		t.Errorf("Validation failed for valid config: %v", verrs)
	}

	// Test SkipInstall
	skipInstallCfg := HAProxyConfig{SkipInstall: boolPtr(true)}
	// SetDefaults_HAProxyConfig(&skipInstallCfg) // Defaults would set other fields, but SkipInstall=true short-circuits validation
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
		{"bad_frontend_port", HAProxyConfig{FrontendPort: intPtr(0), BackendServers: []HAProxyBackendServer{validServer}}, ".frontendPort: invalid port 0"},
		{"invalid_mode", HAProxyConfig{Mode: stringPtr("udp"), BackendServers: []HAProxyBackendServer{validServer}}, ".mode: invalid mode 'udp'"},
		{"invalid_algo", HAProxyConfig{BalanceAlgorithm: stringPtr("randomest"), BackendServers: []HAProxyBackendServer{validServer}}, ".balanceAlgorithm: invalid algorithm 'randomest'"},
		{"no_backend_servers", HAProxyConfig{FrontendPort: intPtr(123), BackendServers: []HAProxyBackendServer{}}, ".backendServers: must specify at least one backend server"},
		{"backend_empty_name", HAProxyConfig{BackendServers: []HAProxyBackendServer{{Address: "a", Port: 1}}}, ".name: backend server name cannot be empty"},
		{"backend_empty_addr", HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name: "n", Port: 1}}}, ".address: backend server address cannot be empty"},
		{"backend_bad_port", HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name: "n", Address: "a", Port: 0}}}, ".port: invalid backend server port 0"},
		{"backend_bad_weight", HAProxyConfig{BackendServers: []HAProxyBackendServer{{Name: "n", Address: "a", Port: 1, Weight: intPtr(-1)}}}, ".weight: cannot be negative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure necessary fields that are not part of the specific test case logic are valid or defaulted
			// This makes individual test cases cleaner as they only need to specify the invalid part.
			if tt.cfg.FrontendPort == nil && !strings.Contains(tt.wantErrMsg, ".frontendPort") {
				tt.cfg.FrontendPort = intPtr(6443) // Default valid port
			}
			if len(tt.cfg.BackendServers) == 0 && !strings.Contains(tt.wantErrMsg, ".backendServers") && !strings.Contains(tt.wantErrMsg, ".name") && !strings.Contains(tt.wantErrMsg, ".address") && !strings.Contains(tt.wantErrMsg, ".port") && !strings.Contains(tt.wantErrMsg, ".weight") {
				tt.cfg.BackendServers = []HAProxyBackendServer{validServer} // Default valid backend
			}

			SetDefaults_HAProxyConfig(&tt.cfg)
			verrs := &ValidationErrors{}
			Validate_HAProxyConfig(&tt.cfg, verrs, "haproxy")

			assert.False(t, verrs.IsEmpty(), "Expected error for %s, got none", tt.name)
			assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
		})
	}
}
