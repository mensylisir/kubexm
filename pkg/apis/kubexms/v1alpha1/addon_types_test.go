package v1alpha1

import (
	"strings"
	"testing"
)

// Helper for Addon tests
func pboolAddon(b bool) *bool { return &b }
func pint32Addon(i int32) *int32 { return &i }

func TestSetDefaults_AddonConfig(t *testing.T) {
	cfg := &AddonConfig{Name: "my-addon"} // Name is required for some defaults potentially
	SetDefaults_AddonConfig(cfg)

	if cfg.Enabled == nil || !*cfg.Enabled {
		t.Errorf("Default Enabled = %v, want true", cfg.Enabled)
	}
	if cfg.Retries == nil || *cfg.Retries != 0 {
		t.Errorf("Default Retries = %v, want 0", cfg.Retries)
	}
	if cfg.Delay == nil || *cfg.Delay != 5 {
		t.Errorf("Default Delay = %v, want 5", cfg.Delay)
	}
	if cfg.PreInstall == nil { t.Error("PreInstall should be initialized") }
	if cfg.PostInstall == nil { t.Error("PostInstall should be initialized") }

	// Test chart defaults
	cfgWithChart := &AddonConfig{Name: "chart-addon", Sources: AddonSources{Chart: &ChartSource{}}}
	SetDefaults_AddonConfig(cfgWithChart)
	if cfgWithChart.Sources.Chart.Wait == nil || !*cfgWithChart.Sources.Chart.Wait {
		t.Errorf("Chart.Wait default = %v, want true", cfgWithChart.Sources.Chart.Wait)
	}
	if cfgWithChart.Sources.Chart.Values == nil {t.Error("Chart.Values should be initialized")}


	// Test yaml defaults
	cfgWithYaml := &AddonConfig{Name: "yaml-addon", Sources: AddonSources{Yaml: &YamlSource{}}}
	SetDefaults_AddonConfig(cfgWithYaml)
	if cfgWithYaml.Sources.Yaml.Path == nil {t.Error("Yaml.Path should be initialized")}

}

func TestValidate_AddonConfig_Valid(t *testing.T) {
	cfg := &AddonConfig{
		Name: "metrics-server",
		Enabled: pboolAddon(true),
		Sources: AddonSources{Chart: &ChartSource{Name: "metrics-server", Repo: "https://charts.bitnami.com/bitnami"}},
	}
	SetDefaults_AddonConfig(cfg)
	verrs := &ValidationErrors{}
	Validate_AddonConfig(cfg, verrs, "spec.addons[0]")
	if !verrs.IsEmpty() {
		t.Errorf("Validate_AddonConfig for valid config failed: %v", verrs)
	}
}

func TestValidate_AddonConfig_Invalid(t *testing.T) {
   tests := []struct{
	   name string
	   cfg *AddonConfig
	   wantErrMsg string
   }{
	   {"empty_name", &AddonConfig{Name: " "}, ".name: addon name cannot be empty"},
	   {"chart_no_name_or_path", &AddonConfig{Name:"a", Sources: AddonSources{Chart: &ChartSource{Repo: "r"}}}, "either chart.name (with repo) or chart.path (for local chart) must be specified"},
	   {"chart_repo_no_name", &AddonConfig{Name:"a", Sources: AddonSources{Chart: &ChartSource{Repo: "r", Name:" "}}}, "chart name must be specified if chart.repo is set"},
	   {"yaml_no_path", &AddonConfig{Name:"a", Sources: AddonSources{Yaml: &YamlSource{Path: []string{}}}}, ".yaml.path: must contain at least one YAML path"},
	   {"yaml_empty_path_item", &AddonConfig{Name:"a", Sources: AddonSources{Yaml: &YamlSource{Path: []string{" "}}}}, ".yaml.path[0]: path/URL cannot be empty"},
	   {"negative_retries", &AddonConfig{Name:"a", Retries: pint32Addon(-1)}, ".retries: cannot be negative"},
	   {"negative_delay", &AddonConfig{Name:"a", Delay: pint32Addon(-1)}, ".delay: cannot be negative"},
   }
   for _, tt := range tests {
	   t.Run(tt.name, func(t *testing.T){
		   SetDefaults_AddonConfig(tt.cfg)
		   verrs := &ValidationErrors{}
		   Validate_AddonConfig(tt.cfg, verrs, "spec.addons[test]")
		   if verrs.IsEmpty() {
			   t.Fatalf("Expected error for %s, got none", tt.name)
		   }
		   if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
			   t.Errorf("Error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
		   }
	   })
   }
}
