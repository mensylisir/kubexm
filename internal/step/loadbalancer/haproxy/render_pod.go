package haproxy

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

// RenderHAProxyPodManifestStep renders HAProxy static pod manifest.
type RenderHAProxyPodManifestStep struct {
	step.Base
	ConfigHash string
}

type RenderHAProxyPodManifestStepBuilder struct {
	step.Builder[RenderHAProxyPodManifestStepBuilder, *RenderHAProxyPodManifestStep]
}

func NewRenderHAProxyPodManifestStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RenderHAProxyPodManifestStepBuilder {
	s := &RenderHAProxyPodManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Render HAProxy static pod manifest", instanceName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(RenderHAProxyPodManifestStepBuilder).Init(s)
}

func (s *RenderHAProxyPodManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderHAProxyPodManifestStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RenderHAProxyPodManifestStep) buildPodManifest(imageName, configHash string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: kube-haproxy
  namespace: kube-system
  labels:
    component: kube-haproxy
spec:
  containers:
  - name: haproxy
    image: %s
    imagePullPolicy: IfNotPresent
    livenessProbe:
      httpGet:
        path: /healthz
        port: 6443
        scheme: HTTPS
    readinessProbe:
      httpGet:
        path: /healthz
        port: 6443
        scheme: HTTPS
    resources:
      requests:
        cpu: "100m"
        memory: "64Mi"
      limits:
        cpu: "500m"
        memory: "128Mi"
    volumeMounts:
    - name: haproxy-config
      mountPath: /usr/local/etc/haproxy/haproxy.cfg
      subPath: haproxy.cfg
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  volumes:
  - name: haproxy-config
    hostPath:
      path: %s
      type: DirectoryOrCreate
`, imageName, common.HAProxyDefaultConfDirTarget)
}

func (s *RenderHAProxyPodManifestStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	// Get image
	imageProvider := images.NewImageProvider(ctx)
	haproxyImage := imageProvider.GetImage("haproxy")
	fullImageName := haproxyImage.FullName()

	// Get rendered config from context
	configContentRaw, ok := ctx.Import("", "haproxy_rendered_config")
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

	// Store manifest content in context for downstream steps
	ctx.Export("task", "haproxy_rendered_pod_manifest", manifestContent)

	logger.Infof("HAProxy static pod manifest rendered successfully")
	result.MarkCompleted("HAProxy static pod manifest rendered")
	return result, nil
}

func (s *RenderHAProxyPodManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RenderHAProxyPodManifestStep)(nil)

// GetHAProxyRenderedConfig retrieves the rendered config from context.
func GetHAProxyRenderedConfig(ctx runtime.ExecutionContext) (string, bool) {
	val, ok := ctx.Import("", "haproxy_rendered_config")
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetHAProxyRenderedPodManifest retrieves the rendered pod manifest from context.
func GetHAProxyRenderedPodManifest(ctx runtime.ExecutionContext) (string, bool) {
	val, ok := ctx.Import("", "haproxy_rendered_pod_manifest")
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// ManifestPath returns the static pod manifest path for HAProxy.
func ManifestPath() string {
	return filepath.Join(common.KubernetesManifestsDir, "kube-haproxy.yaml")
}
