package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
)

type Preflight struct {
	MinCPUCores      *int32  `json:"minCPUCores,omitempty" yaml:"minCPUCores,omitempty"`
	MinMemoryMB      *uint64 `json:"minMemoryMB,omitempty" yaml:"minMemoryMB,omitempty"`
	DisableSwap      *bool   `json:"disableSwap,omitempty" yaml:"disableSwap,omitempty"`
	DisableFirewalld *bool   `json:"disableFirewalld,omitempty" yaml:"disableFirewalld,omitempty"`
	DisableSelinux   *bool   `json:"disableSelinux,omitempty" yaml:"disableSelinux,omitempty"`

	SkipChecks []string `json:"skipChecks,omitempty" yaml:"skipChecks,omitempty"`
}

func SetDefaults_Preflight(cfg *Preflight) {
	if cfg == nil {
		return
	}
	if cfg.DisableSwap == nil {
		cfg.DisableSwap = helpers.BoolPtr(true)
	}
	if cfg.DisableFirewalld == nil {
		cfg.DisableFirewalld = helpers.BoolPtr(true)
	}
	if cfg.DisableSelinux == nil {
		cfg.DisableSelinux = helpers.BoolPtr(true)
	}
	if cfg.MinMemoryMB == nil {
		cfg.MinMemoryMB = helpers.Uint64Ptr(common.DefaultMinMemoryMB)
	}
	if cfg.MinCPUCores == nil {
		cfg.MinCPUCores = helpers.Int32Ptr(common.DefaultMinCPUCores)
	}
}

func Validate_Preflight(cfg *Preflight, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.MinCPUCores != nil && *cfg.MinCPUCores <= 0 {
		verrs.Add(pathPrefix + ".minCPUCores: must be positive if specified, got " + fmt.Sprintf("%d", *cfg.MinCPUCores))
	}
	if cfg.MinMemoryMB != nil && *cfg.MinMemoryMB <= 0 {
		verrs.Add(pathPrefix + ".minMemoryMB: must be positive if specified, got " + fmt.Sprintf("%d", *cfg.MinMemoryMB))
	}
	for i, checkToSkip := range cfg.SkipChecks {
		if !helpers.ContainsString(common.SupportedChecks, checkToSkip) {
			verrs.Add(fmt.Sprintf("%s.skipChecks[%d]: unsupported check '%s', must be one of %v",
				pathPrefix, i, checkToSkip, common.SupportedChecks))
		}
	}

}
