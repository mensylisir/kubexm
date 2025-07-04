package v1alpha1

// PreflightConfig holds configuration for preflight checks.
type PreflightConfig struct {
	MinCPUCores   *int32  `json:"minCPUCores,omitempty"`   // Pointer for optionality
	MinMemoryMB   *uint64 `json:"minMemoryMB,omitempty"`   // Pointer for optionality
	DisableSwap   *bool   `json:"disableSwap,omitempty"`   // Pointer for three-state (true, false, not set)
	// TODO: Add more preflight checks like disk space, specific kernel modules required, etc.
}

// SetDefaults_PreflightConfig sets default values for PreflightConfig.
func SetDefaults_PreflightConfig(cfg *PreflightConfig) {
	if cfg == nil {
		return
	}
	if cfg.DisableSwap == nil {
		cfg.DisableSwap = boolPtr(true) // Default to disabling swap
	}
	// MinCPUCores and MinMemoryMB can have system-wide defaults if desired
	// if cfg.MinCPUCores == nil { defaultCPU := int32(2); cfg.MinCPUCores = &defaultCPU }
	// if cfg.MinMemoryMB == nil { defaultMem := uint64(2048); cfg.MinMemoryMB = &defaultMem }
}

// Validate_PreflightConfig validates PreflightConfig.
func Validate_PreflightConfig(cfg *PreflightConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.MinCPUCores != nil && *cfg.MinCPUCores <= 0 {
		verrs.Add("%s.minCPUCores: must be positive if specified, got %d", pathPrefix, *cfg.MinCPUCores)
	}
	if cfg.MinMemoryMB != nil && *cfg.MinMemoryMB <= 0 { // Memory should be positive
		verrs.Add("%s.minMemoryMB: must be positive if specified, got %d", pathPrefix, *cfg.MinMemoryMB)
	}
}
