package v1alpha1

import (
	"strings"
	"testing"
)

func TestSetDefaults_KernelConfig(t *testing.T) {
	cfg := &KernelConfig{}
	SetDefaults_KernelConfig(cfg)
	if cfg.Modules == nil {
		t.Error("Modules should be initialized to empty slice")
	}
	if cfg.SysctlParams == nil {
		t.Error("SysctlParams should be initialized to empty map")
	}
	// Add tests for specific default sysctl params if any are added
}

func TestValidate_KernelConfig(t *testing.T) {
	validCfg := &KernelConfig{Modules: []string{"br_netfilter"}, SysctlParams: map[string]string{"net.ipv4.ip_forward": "1"}}
	verrsValid := &ValidationErrors{}
	Validate_KernelConfig(validCfg, verrsValid, "spec.kernel")
	if !verrsValid.IsEmpty() {
		t.Errorf("Validate_KernelConfig for valid config failed: %v", verrsValid)
	}

	tests := []struct{
	   name string
	   cfg *KernelConfig
	   wantErrMsg string
	}{
	   {"empty_module", &KernelConfig{Modules: []string{" "}}, ".modules[0]: module name cannot be empty"},
	   {"empty_sysctl_key", &KernelConfig{SysctlParams: map[string]string{" ": "1"}}, ".sysctlParams: sysctl key cannot be empty"},
	}
	for _, tt := range tests {
	   t.Run(tt.name, func(t *testing.T){
		   SetDefaults_KernelConfig(tt.cfg)
		   verrs := &ValidationErrors{}
		   Validate_KernelConfig(tt.cfg, verrs, "spec.kernel")
		   if verrs.IsEmpty() {
			   t.Fatalf("Expected error for %s, got none", tt.name)
		   }
		   if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
			   t.Errorf("Error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
		   }
	   })
	}
}
