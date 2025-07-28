package util

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/logger"
	"os"
	"path"
)

type Image struct {
	RepoAddr          string
	Namespace         string
	NamespaceOverride string
	Repo              string
	Tag               string
	Group             string
	Enable            bool
	NamespaceRewrite  *v1alpha1.NamespaceRewrite
	OriginalRepoAddr  string `json:"-"`
}
type Images struct {
	Images []Image
}

func isUseCNRegistry(image Image) bool {
	return os.Getenv("KXZONE") == "cn" && (image.RepoAddr == "" || image.RepoAddr == common.CnRegistry)
}

func (image Image) getFinalRegistry() string {
	if isUseCNRegistry(image) {
		return common.CnRegistry
	}
	if image.RepoAddr != "" {
		return image.RepoAddr
	}
	return ""
}

func (image Image) getFinalNamespace() string {
	if isUseCNRegistry(image) {
		return common.CnNamespaceOverride
	}

	if image.NamespaceOverride != "" {
		return image.NamespaceOverride
	}

	return image.applyNamespaceRewrite()
}

func (image Image) applyNamespaceRewrite() string {
	rewriteConfig := image.NamespaceRewrite
	if rewriteConfig == nil || rewriteConfig.Enabled == nil || !*rewriteConfig.Enabled || len(rewriteConfig.Rules) == 0 {
		return image.Namespace
	}
	currentRegistry := image.getFinalRegistry()
	if currentRegistry == "" {
		currentRegistry = "docker.io"
	}

	for _, rule := range rewriteConfig.Rules {
		registryMatch := (rule.Registry == "" || rule.Registry == currentRegistry)
		namespaceMatch := (rule.OldNamespace == image.Namespace)

		if registryMatch && namespaceMatch {
			logger.Debug("Applied namespace rewrite rule for registry '%s': '%s' -> '%s'", currentRegistry, rule.OldNamespace, rule.NewNamespace)
			return rule.NewNamespace
		}
	}

	return image.Namespace
}

func (image Image) ImageName() string {
	return fmt.Sprintf("%s:%s", image.ImageRepo(), image.Tag)
}

func (image Image) ImageNamespace() string {
	if os.Getenv("KXZONE") == "cn" {
		if image.RepoAddr == "" || image.RepoAddr == common.CnRegistry {
			image.NamespaceOverride = common.CnNamespaceOverride
		}
	}

	if image.NamespaceOverride != "" {
		return image.NamespaceOverride
	} else {
		return image.Namespace
	}
}

func (image Image) ImageRegistryAddr() string {
	if os.Getenv("KXZONE") == "cn" {
		if image.RepoAddr == "" || image.RepoAddr == common.CnRegistry {
			image.RepoAddr = common.CnRegistry
		}
	}
	if image.RepoAddr != "" {
		return image.RepoAddr
	} else {
		return "docker.io"
	}
}

func (image Image) ImageRepo() string {
	finalRegistry := image.getFinalRegistry()
	finalNamespace := image.getFinalNamespace()
	if finalRegistry != "" {
		if finalNamespace != "" {
			return path.Join(finalRegistry, finalNamespace, image.Repo)
		}
		return path.Join(finalRegistry, "library", image.Repo)
	}
	if finalNamespace != "" {
		return path.Join(finalNamespace, image.Repo)
	}
	return path.Join("library", image.Repo)
}

func DefaultRegistry() string {
	if os.Getenv("KXZONE") == "cn" {
		return common.CnRegistry
	}
	return "docker.io"
}
