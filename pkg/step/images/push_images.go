package images

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OciIndex 描述了 OCI 布局中的 index.json 结构
type OciIndex struct {
	Manifests []OciManifest `json:"manifests"`
}
type OciManifest struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations"`
}

// PushImagesToRegistryStep 负责从本地 OCI 目录读取镜像，重写名称，并推送到私有仓库。
type PushImagesToRegistryStep struct {
	step.Base
	LocalImagesPath string
	ImageProvider   *images.ImageProvider
	Auths           map[string]v1alpha1.RegistryAuth
}

// PushImagesToRegistryStepBuilder ...
type PushImagesToRegistryStepBuilder struct {
	step.Builder[PushImagesToRegistryStepBuilder, *PushImagesToRegistryStep]
}

// NewPushImagesToRegistryStepBuilder ...
func NewPushImagesToRegistryStepBuilder(ctx runtime.Context, instanceName string) *PushImagesToRegistryStepBuilder {
	s := &PushImagesToRegistryStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Push images from local artifacts to private registry", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Hour // 推送镜像可能很耗时

	// LocalImagesPath 应该指向解压后的 .kubexm/images 目录
	s.LocalImagesPath = filepath.Join(ctx.GetGlobalWorkDir(), "images") // 假设路径
	s.ImageProvider = images.NewImageProvider(ctx)
	s.Auths = ctx.GetClusterConfig().Spec.Registry.Auths

	b := new(PushImagesToRegistryStepBuilder).Init(s)
	return b
}

func (s *PushImagesToRegistryStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)

	// 1. 读取 index.json
	indexPath := filepath.Join(s.LocalImagesPath, "index.json")
	indexFile, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("failed to read OCI index file at %s: %w", indexPath, err)
	}

	var index OciIndex
	if err := json.Unmarshal(indexFile, &index); err != nil {
		return fmt.Errorf("failed to unmarshal OCI index file: %w", err)
	}
	logger.Infof("Found %d image manifests in local OCI directory.", len(index.Manifests))

	// 2. 遍历清单，推送每个镜像
	for _, manifest := range index.Manifests {
		// oci.manifest.ref.name 通常是 image:tag-platform, e.g., "docker.io/library/nginx:1.21-amd64"
		originalRefWithPlatform := manifest.Annotations["oci.manifest.ref.name"]
		if originalRefWithPlatform == "" {
			continue
		}

		// 3. 从引用中解析出组件名
		// e.g., "docker.io/library/nginx:1.21-amd64" -> "docker.io/library/nginx"
		refWithoutTag, _, _ := strings.Cut(originalRefWithPlatform, ":")
		// e.g., "docker.io/library/nginx" -> "nginx"
		componentName := images.getComponentNameByRef(refWithoutTag)
		if componentName == "" {
			logger.Warnf("Could not find component name for image ref '%s', skipping push.", refWithoutTag)
			continue
		}

		// 4. 使用 ImageProvider 获取完整的、重写后的 Image 对象
		imgObj := s.ImageProvider.GetImage(componentName)
		if imgObj == nil {
			logger.Infof("Image for component '%s' is not enabled in current config, skipping push.", componentName)
			continue
		}

		// 5. 准备推送
		// 源: oci:./path:original-ref-with-platform
		srcName := fmt.Sprintf("oci:%s:%s", s.LocalImagesPath, originalRefWithPlatform)
		// 目标: my-harbor.com/rewritten-ns/repo:tag
		destName := imgObj.FullName()

		logger.Infof("Pushing image: %s -> %s", srcName, destName)

		// 获取私有仓库的认证信息
		privateRepoAddr := imgObj.GetPrivateRepoAddr() // 假设 Image 对象有一个方法可以获取私有仓库地址
		auth, _ := s.Auths[privateRepoAddr]

		// 6. 执行复制 (带重试逻辑)
		copyOpts := &imagecopy.CopyOptions{
			SrcImage:  srcName,
			DestImage: destName,
			DestAuth:  auth,
		}

		var lastErr error
		for attempt := 1; attempt <= 3; attempt++ {
			if err := imagecopy.Copy(copyOpts); err != nil {
				lastErr = err
				logger.Warnf("Attempt %d failed to push image %s: %v. Retrying in 5 seconds...", attempt, destName, err)
				time.Sleep(5 * time.Second)
				continue
			}
			lastErr = nil
			break
		}
		if lastErr != nil {
			return fmt.Errorf("failed to push image %s after 3 attempts: %w", destName, lastErr)
		}
	}

	logger.Info("All images from local artifacts have been pushed to the private registry.")
	return nil
}

// ... Meta, Precheck, Rollback (清理私有仓库中的镜像会很复杂，通常不实现) ...
