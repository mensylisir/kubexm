package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
)

type ContainerRuntime struct {
	Type       common.ContainerRuntimeType `json:"type,omitempty" yaml:"type,omitempty"`
	Version    string                      `json:"version,omitempty" yaml:"version,omitempty"`
	Docker     *Docker                     `json:"docker,omitempty" yaml:"docker,omitempty"`
	Containerd *Containerd                 `json:"containerd,omitempty" yaml:"containerd,omitempty"`
	Crio       *Crio                       `json:"crio,omitempty" yaml:"crio,omitempty"`
	Isulad     *Isulad                     `json:"isulad,omitempty" yaml:"isulad,omitempty"`
}

func SetDefaults_ContainerRuntimeConfig(cfg *ContainerRuntime) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		cfg.Type = common.RuntimeTypeContainerd
	}

	switch cfg.Type {
	case common.RuntimeTypeDocker:
		if cfg.Docker == nil {
			cfg.Docker = &Docker{}
		}
		SetDefaults_DockerConfig(cfg.Docker)
	case common.RuntimeTypeContainerd:
		if cfg.Containerd == nil {
			cfg.Containerd = &Containerd{}
		}
		SetDefaults_ContainerdConfig(cfg.Containerd)
	case common.RuntimeTypeCRIO:
		if cfg.Crio == nil {
			cfg.Crio = &Crio{}
		}
		SetDefaults_CrioConfig(cfg.Crio)
	case common.RuntimeTypeIsula:
		if cfg.Isulad == nil {
			cfg.Isulad = &Isulad{}
		}
		SetDefaults_IsuladConfig(cfg.Isulad)
	}
}

func Validate_ContainerRuntimeConfig(cfg *ContainerRuntime, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		verrs.Add(pathPrefix, "containerRuntime section cannot be nil (should be defaulted if not present)")
		return
	}
	isValidType := false
	for _, vt := range common.ValidContainerRuntimeTypes {
		if cfg.Type == vt {
			isValidType = true
			break
		}
	}
	if !isValidType {
		verrs.Add(pathPrefix+".type", fmt.Sprintf("invalid container runtime type '%s', must be one of %v", string(cfg.Type), common.ValidContainerRuntimeTypes))
		return
	}

	if cfg.Version != "" && !helpers.IsValidNonEmptyString(cfg.Version) {
		verrs.Add(pathPrefix+".version", "version cannot be only whitespace if specified")
	}
	if cfg.Version != "" && !helpers.IsValidSemanticVersion(cfg.Version) {
		verrs.Add(pathPrefix+".version", "invalid version format")
	}

	switch cfg.Type {
	case common.RuntimeTypeDocker:
		if cfg.Docker == nil {
			verrs.Add(pathPrefix+".docker", "must be configured when type is 'docker'")
		} else {
			Validate_DockerConfig(cfg.Docker, verrs, pathPrefix+".docker")
		}
		if cfg.Containerd != nil {
			verrs.Add(pathPrefix+".containerd", "must not be set when type is 'docker'")
		}
		if cfg.Crio != nil {
			verrs.Add(pathPrefix+".crio", "must not be set when type is 'docker'")
		}
		if cfg.Isulad != nil {
			verrs.Add(pathPrefix+".isulad", "must not be set when type is 'docker'")
		}
	case common.RuntimeTypeContainerd:
		if cfg.Containerd == nil {
			verrs.Add(pathPrefix+".containerd", "must be configured when type is 'containerd'")
		} else {
			Validate_ContainerdConfig(cfg.Containerd, verrs, pathPrefix+".containerd")
		}
		if cfg.Docker != nil {
			verrs.Add(pathPrefix+".docker", "must not be set when type is 'containerd'")
		}
		if cfg.Crio != nil {
			verrs.Add(pathPrefix+".crio", "must not be set when type is 'containerd'")
		}
		if cfg.Isulad != nil {
			verrs.Add(pathPrefix+".isulad", "must not be set when type is 'containerd'")
		}
	case common.RuntimeTypeCRIO:
		if cfg.Crio == nil {
			verrs.Add(pathPrefix+".crio", "must be configured when type is 'crio'")
		} else {
			Validate_CrioConfig(cfg.Crio, verrs, pathPrefix+".crio")
		}
		if cfg.Docker != nil {
			verrs.Add(pathPrefix+".docker", "must not be set when type is 'crio'")
		}
		if cfg.Containerd != nil {
			verrs.Add(pathPrefix+".containerd", "must not be set when type is 'crio'")
		}
		if cfg.Isulad != nil {
			verrs.Add(pathPrefix+".isulad", "must not be set when type is 'crio'")
		}
	case common.RuntimeTypeIsula:
		if cfg.Isulad == nil {
			verrs.Add(pathPrefix+".isulad", "must be configured when type is 'isula'")
		} else {
			Validate_IsuladConfig(cfg.Isulad, verrs, pathPrefix+".isulad")
		}
		if cfg.Docker != nil {
			verrs.Add(pathPrefix+".docker", "must not be set when type is 'isula'")
		}
		if cfg.Containerd != nil {
			verrs.Add(pathPrefix+".containerd", "must not be set when type is 'isula'")
		}
		if cfg.Crio != nil {
			verrs.Add(pathPrefix+".crio", "must not be set when type is 'isula'")
		}
	}
}
