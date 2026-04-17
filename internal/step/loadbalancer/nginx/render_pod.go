package nginx

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/util/images"
	"github.com/mensylisir/kubexm/internal/types"
)

// RenderNginxPodManifestStep renders NGINX static pod manifest.
type RenderNginxPodManifestStep struct {
	step.Base
	ConfigHash string
}

type RenderNginxPodManifestStepBuilder struct {
	step.Builder[RenderNginxPodManifestStepBuilder, *RenderNginxPodManifestStep]
}

func NewRenderNginxPodManifestStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RenderNginxPodManifestStepBuilder {
	s := &RenderNginxPodManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Render NGINX static pod manifest", instanceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(RenderNginxPodManifestStepBuilder).Init(s)
}

func (s *RenderNginxPodManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderNginxPodManifestStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RenderNginxPodManifestStep) buildPodManifest(imageName, configHash string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: kube-nginx
  namespace: kube-system
  labels:
    component: kube-nginx
spec:
  containers:
  - name: nginx
    image: %s
    imagePullPolicy: IfNotPresent
    livenessProbe:
      tcpSocket:
        port: 6443
      initialDelaySeconds: 10
      periodSeconds: 10
    readinessProbe:
      tcpSocket:
        port: 6443
      initialDelaySeconds: 5
      periodSeconds: 5
    resources:
      requests:
        cpu: "100m"
        memory: "64Mi"
      limits:
        cpu: "500m"
        memory: "128Mi"
    volumeMounts:
    - name: nginx-config
      mountPath: /etc/nginx/nginx.conf
      subPath: nginx.conf
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  volumes:
  - name: nginx-config
    hostPath:
      path: %s
      type: DirectoryOrCreate
`, imageName, common.DefaultNginxConfigDir)
}

func (s *RenderNginxPodManifestStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	// Get image
	imageProvider := images.NewImageProvider(ctx)
	nginxImage := imageProvider.GetImage("nginx")
	fullImageName := nginxImage.FullName()

	// Get rendered config from context
	configContentRaw, ok := ctx.Import("", "nginx_rendered_config")
	if !ok {
		result.MarkFailed(fmt.Errorf("rendered config not found in context"), "no config found")
		return result, nil
	}
	configContent, ok := configContentRaw.(string)
	if !ok {
		result.MarkFailed(fmt.Errorf("rendered config has invalid type"), "invalid type")
		return result, nil
	}

	configHash := fmt.Sprintf("%x", sha256.Sum256([]byte(configContent)))
	s.ConfigHash = configHash

	manifestContent := s.buildPodManifest(fullImageName, configHash)

	ctx.Export("task", "nginx_rendered_pod_manifest", manifestContent)

	logger.Infof("NGINX static pod manifest rendered successfully")
	result.MarkCompleted("NGINX static pod manifest rendered")
	return result, nil
}

func (s *RenderNginxPodManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RenderNginxPodManifestStep)(nil)

// GetNginxRenderedPodManifest retrieves the rendered pod manifest from context.
func GetNginxRenderedPodManifest(ctx runtime.ExecutionContext) (string, bool) {
	val, ok := ctx.Import("", "nginx_rendered_pod_manifest")
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// NginxManifestPath returns the static pod manifest path for NGINX.
func NginxManifestPath() string {
	return filepath.Join(common.KubernetesManifestsDir, "kube-nginx.yaml")
}
