package haproxy

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	pkgcommon "github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	lbcommon "github.com/mensylisir/kubexm/internal/step/loadbalancer/common"
	"github.com/mensylisir/kubexm/internal/types"
)

// RenderHAProxyConfigStep renders HAProxy configuration from template.
type RenderHAProxyConfigStep struct {
	step.Base
	TemplateData *HAProxyConfigTemplateData
}

type HAProxyConfigTemplateData struct {
	FrontendBindAddress string
	FrontendBindPort    int
	BackendServers      []lbcommon.BackendServer
}

type RenderHAProxyConfigStepBuilder struct {
	step.Builder[RenderHAProxyConfigStepBuilder, *RenderHAProxyConfigStep]
}

func NewRenderHAProxyConfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RenderHAProxyConfigStepBuilder {
	s := &RenderHAProxyConfigStep{
		TemplateData: &HAProxyConfigTemplateData{},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Render HAProxy config", instanceName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(RenderHAProxyConfigStepBuilder).Init(s)
}

func (s *RenderHAProxyConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderHAProxyConfigStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil // Always render to ensure up-to-date
}

func (s *RenderHAProxyConfigStep) BuildTemplateData(ctx runtime.ExecutionContext) error {
	cluster := ctx.GetClusterConfig()
	cpEndpoint := cluster.Spec.ControlPlaneEndpoint

	// Collect identity - use ControlPlaneEndpoint.Address as default
	identity := &lbcommon.LBIdentity{
		VIP:          cpEndpoint.Address,
		BindAddress:  cpEndpoint.Address,
		FrontendPort: cpEndpoint.Port,
		BackendPort:  pkgcommon.DefaultAPIServerPort,
	}

	// Override with HA config if present
	if cpEndpoint.HighAvailability != nil {
		// Try to get VIP from KubeVIP config
		if cpEndpoint.HighAvailability.External != nil &&
			cpEndpoint.HighAvailability.External.KubeVIP != nil &&
			cpEndpoint.HighAvailability.External.KubeVIP.VIP != nil &&
			*cpEndpoint.HighAvailability.External.KubeVIP.VIP != "" {
			identity.VIP = *cpEndpoint.HighAvailability.External.KubeVIP.VIP
			identity.BindAddress = identity.VIP
		}
		// Try to get VIP from Keepalived config
		if cpEndpoint.HighAvailability.External != nil &&
			cpEndpoint.HighAvailability.External.Keepalived != nil &&
			len(cpEndpoint.HighAvailability.External.Keepalived.VRRPInstances) > 0 &&
			len(cpEndpoint.HighAvailability.External.Keepalived.VRRPInstances[0].VirtualIPs) > 0 {
			vip := cpEndpoint.HighAvailability.External.Keepalived.VRRPInstances[0].VirtualIPs[0]
			if vip != "" {
				identity.VIP = vip
				identity.BindAddress = vip
			}
		}
	}

	// Collect backends
	masterNodes := ctx.GetHostsByRole(pkgcommon.RoleMaster)
	backends := make([]lbcommon.BackendServer, len(masterNodes))
	for i, node := range masterNodes {
		backends[i] = lbcommon.BackendServer{
			Name:    node.GetName(),
			Address: node.GetInternalAddress(),
			Port:    pkgcommon.DefaultAPIServerPort,
		}
	}

	s.TemplateData.FrontendBindAddress = identity.BindAddress
	s.TemplateData.FrontendBindPort = identity.FrontendPort
	s.TemplateData.BackendServers = backends

	return nil
}

func (s *RenderHAProxyConfigStep) RenderContent(ctx runtime.ExecutionContext) (string, error) {
	if s.TemplateData == nil {
		if err := s.BuildTemplateData(ctx); err != nil {
			return "", err
		}
	}

	tmpl := `global
  log /dev/log local0
  log /dev/log local1 notice
  user haproxy
  group haproxy
  daemon

defaults
  log     global
  mode    tcp
  option  tcplog
  option  dontlognull
  timeout connect 5000ms
  timeout client  50000ms
  timeout server  50000ms

frontend kubernetes-apiserver
  bind {{ .FrontendBindAddress }}:{{ .FrontendBindPort }}
  mode tcp
  option tcplog
  default_backend kubernetes-apiserver

backend kubernetes-apiserver
  option httpchk
  http-check expect status 200
  mode tcp
{{ range .BackendServers }}
  server {{ .Name }} {{ .Address }}:{{ .Port }} check inter 2000 fall 3 rise 2
{{ end }}
`

	t, err := template.New("haproxy").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, s.TemplateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

func (s *RenderHAProxyConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	content, err := s.RenderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render HAProxy config")
		return result, err
	}

	// Store rendered content in context for downstream steps
	ctx.Export("task", "haproxy_rendered_config", content)

	logger.Infof("HAProxy config rendered successfully")
	result.MarkCompleted("HAProxy config rendered")
	return result, nil
}

func (s *RenderHAProxyConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RenderHAProxyConfigStep)(nil)
