package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateHAProxyStaticPodManifestStep struct {
	step.Base
}

func NewGenerateHAProxyStaticPodManifestStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateHAProxyStaticPodManifestStep] {
	s := &GenerateHAProxyStaticPodManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate haproxy static pod manifest"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type HAProxyStaticPodTemplateData struct {
	Image string
}

func (s *GenerateHAProxyStaticPodManifestStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	// In a real scenario, the image would come from the ImageProvider/BOM
	data := HAProxyStaticPodTemplateData{
		Image: "haproxy:2.5",
	}

	// The template would be in pkg/templates/loadbalancer/haproxy/haproxy.yaml.tmpl
	dummyTemplate := `
apiVersion: v1
kind: Pod
metadata:
  name: kube-haproxy
  namespace: kube-system
spec:
  containers:
  - name: kube-haproxy
    image: {{ .Image }}
    command:
    - haproxy
    - -f
    - /usr/local/etc/haproxy/haproxy.cfg
    livenessProbe:
      failureThreshold: 8
      httpGet:
        host: 127.0.0.1
        path: /healthz
        port: 10257
        scheme: HTTPS
      initialDelaySeconds: 10
      periodSeconds: 10
      successThreshold: 1
      timeoutSeconds: 15
    volumeMounts:
    - mountPath: /usr/local/etc/haproxy/haproxy.cfg
      name: haproxy-cfg
      readOnly: true
  hostNetwork: true
  volumes:
  - hostPath:
      path: /etc/haproxy/haproxy.cfg
      type: FileOrCreate
    name: haproxy-cfg
`
	renderedManifest, err := templates.Render(dummyTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render haproxy static pod manifest: %w", err)
	}

	ctx.Set("haproxy.yaml", renderedManifest)
	logger.Info("haproxy.yaml generated successfully.")

	return nil
}

func (s *GenerateHAProxyStaticPodManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("haproxy.yaml")
	return nil
}

func (s *GenerateHAProxyStaticPodManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
