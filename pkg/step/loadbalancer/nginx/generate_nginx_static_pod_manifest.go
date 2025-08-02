package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateNginxStaticPodManifestStep struct {
	step.Base
}

func NewGenerateNginxStaticPodManifestStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateNginxStaticPodManifestStep] {
	s := &GenerateNginxStaticPodManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate nginx static pod manifest"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type NginxStaticPodTemplateData struct {
	Image string
}

func (s *GenerateNginxStaticPodManifestStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	data := NginxStaticPodTemplateData{
		Image: "nginx:1.21",
	}

	dummyTemplate := `
apiVersion: v1
kind: Pod
metadata:
  name: kube-nginx
  namespace: kube-system
spec:
  containers:
  - name: kube-nginx
    image: {{ .Image }}
    command:
    - nginx
    - -g
    - "daemon off;"
    livenessProbe:
      httpGet:
        path: /
        port: 80
    volumeMounts:
    - mountPath: /etc/nginx/nginx.conf
      name: nginx-cfg
      readOnly: true
  hostNetwork: true
  volumes:
  - hostPath:
      path: /etc/nginx/nginx.conf
      type: FileOrCreate
    name: nginx-cfg
`
	renderedManifest, err := templates.Render(dummyTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render nginx static pod manifest: %w", err)
	}

	ctx.Set("nginx.yaml", renderedManifest)
	logger.Info("nginx.yaml generated successfully.")

	return nil
}

func (s *GenerateNginxStaticPodManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("nginx.yaml")
	return nil
}

func (s *GenerateNginxStaticPodManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
