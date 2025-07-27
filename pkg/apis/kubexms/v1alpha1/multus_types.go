package v1alpha1

import (
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
)

type MultusConfig struct {
	Source       AddonSource               `json:"source,omitempty" yaml:"sources,omitempty"`
	Installation *MultusInstallationConfig `json:"installation,omitempty" yaml:"installation,omitempty"`
}

type MultusInstallationConfig struct {
	Enabled   *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Image     string `json:"image,omitempty" yaml:"image,omitempty"`
}

func SetDefaults_MultusConfig(cfg *MultusConfig) {
	if cfg == nil {
		return
	}

	if cfg.Installation == nil {
		cfg.Installation = &MultusInstallationConfig{}
	}

	if cfg.Installation.Enabled == nil {
		cfg.Installation.Enabled = helpers.BoolPtr(false)
	}

	if cfg.Installation.Namespace == "" {
		cfg.Installation.Namespace = "kube-system"
	}

	if cfg.Installation.Image == "" {
		cfg.Installation.Image = common.MultusInstallationConfigImage
	}
}

func Validate_MultusConfig(cfg *MultusConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.Installation != nil {
	}
}
