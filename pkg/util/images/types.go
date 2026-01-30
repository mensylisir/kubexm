package images

import (
	"fmt"
	"path"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

type Image struct {
	OriginalRepoAddr  string // 原始仓库地址, e.g., "quay.io"
	OriginalNamespace string // 原始命名空间, e.g., "calico"
	OriginalRepo      string // 原始仓库名, e.g., "node"
	OriginalTag       string // 原始标签, e.g., "v3.28.0"

	privateRepoAddr   string
	namespaceOverride string
	namespaceRewrite  *v1alpha1.NamespaceRewrite
}

// NewImage 创建一个新的 Image 实例。
// 这是创建 Image 对象的唯一入口，确保了所有必要信息都被填充。
func newImage(bom *ImageBOM, cfg *v1alpha1.RegistryMirroringAndRewriting) *Image {
	return &Image{
		OriginalRepoAddr:  bom.RepoAddr,
		OriginalNamespace: bom.Namespace,
		OriginalRepo:      bom.Repo,
		OriginalTag:       bom.Tag,
		privateRepoAddr:   cfg.PrivateRegistry,
		namespaceOverride: cfg.NamespaceOverride,
		namespaceRewrite:  cfg.NamespaceRewrite,
	}
}

// --- 公共方法 ---

// Name 返回镜像的最终仓库名 (不含仓库地址、命名空间和标签), e.g., "node"
func (i *Image) Name() string {
	return i.OriginalRepo
}

// Tag 返回镜像的最终标签, e.g., "v3.28.0"
func (i *Image) Tag() string {
	return i.OriginalTag
}

func (i *Image) RegistryAddr() string {
	return i.privateRepoAddr
}

func (i *Image) Namespace() string {
	finalNamespace := i.finalNamespace()
	return finalNamespace
}

func (i *Image) RegistryAddrWithNamespace() string {
	finalNamespace := i.finalNamespace()
	if finalNamespace == "" {
		return i.privateRepoAddr
	}
	return path.Join(i.privateRepoAddr, finalNamespace)
}

// FullName 返回镜像在**私有仓库**中的最终完整名称，包含了所有重写逻辑。
// 这是在 Kubernetes manifest 中使用或推送到私有仓库时应该使用的名称。
// Example: "my-harbor.com/public-proxy/node:v3.28.0"
func (i *Image) FullName() string {
	finalNamespace := i.finalNamespace()
	repoAndTag := fmt.Sprintf("%s:%s", i.OriginalRepo, i.OriginalTag)

	if finalNamespace == "" {
		return path.Join(i.privateRepoAddr, repoAndTag)
	}
	return path.Join(i.privateRepoAddr, finalNamespace, repoAndTag)
}

// FullNameWithoutTag 返回镜像在**私有仓库**中的名称，不含标签。
// Example: "my-harbor.com/public-proxy/node"
func (i *Image) FullNameWithoutTag() string {
	finalNamespace := i.finalNamespace()

	if finalNamespace == "" {
		return path.Join(i.privateRepoAddr, i.OriginalRepo)
	}
	return path.Join(i.privateRepoAddr, finalNamespace, i.OriginalRepo)
}

// OriginalFullName 返回镜像在**公共仓库**中的原始完整名称。
// 这是 `SaveImagesStep` 在下载时需要使用的源地址。
// Example: "quay.io/calico/node:v3.28.0"
func (i *Image) OriginalFullName() string {
	repoAndTag := fmt.Sprintf("%s:%s", i.OriginalRepo, i.OriginalTag)

	if i.OriginalNamespace == "" {
		return path.Join(i.OriginalRepoAddr, repoAndTag)
	}
	return path.Join(i.OriginalRepoAddr, i.OriginalNamespace, repoAndTag)
}

// --- 私有辅助方法 ---

// finalNamespace 计算应用重写规则后的最终命名空间。
func (i *Image) finalNamespace() string {
	// 1. 优先应用全局覆盖
	if i.namespaceOverride != "" {
		return i.namespaceOverride
	}

	// 2. 其次应用细粒度重写规则
	if i.namespaceRewrite != nil && i.namespaceRewrite.Enabled != nil && *i.namespaceRewrite.Enabled {
		for _, rule := range i.namespaceRewrite.Rules {
			// 检查规则是否匹配原始仓库和原始命名空间
			if (rule.Registry == "" || rule.Registry == i.OriginalRepoAddr) && rule.OldNamespace == i.OriginalNamespace {
				return rule.NewNamespace // 找到第一个匹配的规则就返回
			}
		}
	}

	// 3. 如果没有规则匹配，则使用原始命名空间
	return i.OriginalNamespace
}
