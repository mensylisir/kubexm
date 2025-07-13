package images

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/logger"
	"os"
	"strings"
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
}

type Images struct {
	Images []Image
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
	var prefix string

	if os.Getenv("KXZONE") == "cn" {
		if image.RepoAddr == "" || image.RepoAddr == common.CnRegistry {
			image.RepoAddr = common.CnRegistry
			image.NamespaceOverride = common.CnNamespaceOverride
		}
	}

	if image.NamespaceRewrite != nil {
		switch image.NamespaceRewrite.Policy {
		case ChangePrefix:
			matchSrc := ""
			for _, src := range image.NamespaceRewrite.Src {
				if strings.Contains(image.Namespace, src) {
					matchSrc = src
				}
			}
			modifiedNamespace := ""
			if matchSrc == "" {
				modifiedNamespace = fmt.Sprintf("%s/%s", image.NamespaceRewrite.Dest, image.Namespace)
			} else {
				modifiedNamespace = strings.ReplaceAll(image.Namespace, matchSrc, image.NamespaceRewrite.Dest)
			}
			logger.Debug("changed image namespace: %s -> %s", image.Namespace, modifiedNamespace)
			image.Namespace = modifiedNamespace
		default:
			logger.Warn("namespace rewrite action not specified")
		}
	}

	if image.RepoAddr == "" {
		if image.Namespace == "" {
			prefix = ""
		} else {
			prefix = fmt.Sprintf("%s/", image.Namespace)
		}
	} else {
		if image.NamespaceOverride == "" {
			if image.Namespace == "" {
				prefix = fmt.Sprintf("%s/library/", image.RepoAddr)
			} else {
				prefix = fmt.Sprintf("%s/%s/", image.RepoAddr, image.Namespace)
			}
		} else {
			prefix = fmt.Sprintf("%s/%s/", image.RepoAddr, image.NamespaceOverride)
		}
	}

	return fmt.Sprintf("%s%s", prefix, image.Repo)
}

func DefaultRegistry() string {
	if os.Getenv("KXZONE") == "cn" {
		return common.CnRegistry
	}
	return "docker.io"
}
