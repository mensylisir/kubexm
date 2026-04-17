package nginx

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	lbcommon "github.com/mensylisir/kubexm/internal/step/loadbalancer/common"
	"github.com/mensylisir/kubexm/internal/types"
)

// RenderNginxConfigStep renders NGINX configuration.
type RenderNginxConfigStep struct {
	step.Base
	TemplateData *NginxConfigTemplateData
}

type NginxConfigTemplateData struct {
	ListenAddress   string
	ListenPort      int
	UpstreamServers []lbcommon.BackendServer
}

type RenderNginxConfigStepBuilder struct {
	step.Builder[RenderNginxConfigStepBuilder, *RenderNginxConfigStep]
}

func NewRenderNginxConfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RenderNginxConfigStepBuilder {
	s := &RenderNginxConfigStep{
		TemplateData: &NginxConfigTemplateData{},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Render NGINX config", instanceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(RenderNginxConfigStepBuilder).Init(s)
}

func (s *RenderNginxConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderNginxConfigStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RenderNginxConfigStep) BuildTemplateData(ctx runtime.ExecutionContext) error {
	cluster := ctx.GetClusterConfig()
	cpEndpoint := cluster.Spec.ControlPlaneEndpoint

	// Collect backends
	masterNodes := ctx.GetHostsByRole(common.RoleMaster)
	backends := make([]lbcommon.BackendServer, len(masterNodes))
	for i, node := range masterNodes {
		backends[i] = lbcommon.BackendServer{
			Name:    node.GetName(),
			Address: node.GetInternalAddress(),
			Port:    common.DefaultAPIServerPort,
		}
	}

	listenAddress := cpEndpoint.Address
	// Try to get bind address from HA config
	if cpEndpoint.HighAvailability != nil {
		if cpEndpoint.HighAvailability.External != nil &&
			cpEndpoint.HighAvailability.External.KubeVIP != nil &&
			cpEndpoint.HighAvailability.External.KubeVIP.VIP != nil &&
			*cpEndpoint.HighAvailability.External.KubeVIP.VIP != "" {
			listenAddress = *cpEndpoint.HighAvailability.External.KubeVIP.VIP
		}
	}

	s.TemplateData.ListenAddress = listenAddress
	s.TemplateData.ListenPort = cpEndpoint.Port
	s.TemplateData.UpstreamServers = backends

	return nil
}

func (s *RenderNginxConfigStep) RenderContent(ctx runtime.ExecutionContext) (string, error) {
	if s.TemplateData == nil {
		if err := s.BuildTemplateData(ctx); err != nil {
			return "", err
		}
	}

	tmpl := `stream {
    log /var/log/nginx/stream.log;
    error_log /var/log/nginx/stream_error.log;

    upstream kubernetes_apiserver {
{{ range .UpstreamServers }}
        server {{ .Address }}:{{ .Port }};
{{ end }}
    }

    server {
        listen {{ .ListenAddress }}:{{ .ListenPort }};
        proxy_pass kubernetes_apiserver;
        proxy_timeout 10s;
        proxy_connect_timeout 1s;
    }
}
`

	t, err := template.New("nginx").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, s.TemplateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (s *RenderNginxConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	content, err := s.RenderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render NGINX config")
		return result, err
	}

	ctx.Export("task", "nginx_rendered_config", content)

	logger.Infof("NGINX config rendered successfully")
	result.MarkCompleted("NGINX config rendered")
	return result, nil
}

func (s *RenderNginxConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RenderNginxConfigStep)(nil)

// GetNginxRenderedConfig retrieves the rendered config from context.
func GetNginxRenderedConfig(ctx runtime.ExecutionContext) (string, bool) {
	val, ok := ctx.Import("", "nginx_rendered_config")
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}
