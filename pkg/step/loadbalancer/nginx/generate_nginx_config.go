package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateNginxConfigStep struct {
	step.Base
}

func NewGenerateNginxConfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateNginxConfigStep] {
	s := &GenerateNginxConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate nginx config file"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type NginxTemplateData struct {
	MasterNodes []spec.Host
	BindPort    int
}

func (s *GenerateNginxConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	data := NginxTemplateData{
		MasterNodes: ctx.GetHostsByRole("master"),
		BindPort:    common.DefaultLBPort,
	}

	// Template for TCP stream load balancing
	dummyTemplate := `
stream {
    upstream kube_apiserver {
        least_conn;
        {{- range .MasterNodes }}
        server {{ .GetInternalAddress }}:6443;
        {{- end }}
    }

    server {
        listen {{ .BindPort }};
        proxy_connect_timeout 5s;
        proxy_timeout 24h; # For kubectl exec
        proxy_pass kube_apiserver;
    }
}
`
	renderedConfig, err := templates.Render(dummyTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render nginx config template: %w", err)
	}

	ctx.Set("nginx.conf", renderedConfig)
	logger.Info("nginx.conf for TCP load balancing generated successfully.")

	return nil
}

func (s *GenerateNginxConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("nginx.conf")
	return nil
}

func (s *GenerateNginxConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
