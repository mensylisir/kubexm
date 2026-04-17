package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

// RenderHAProxyServiceStep renders the HAProxy systemd service file and exports it to context.
type RenderHAProxyServiceStep struct {
	step.Base
}

type RenderHAProxyServiceStepBuilder struct {
	step.Builder[RenderHAProxyServiceStepBuilder, *RenderHAProxyServiceStep]
}

func NewRenderHAProxyServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RenderHAProxyServiceStepBuilder {
	s := &RenderHAProxyServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Render HAProxy systemd service file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(RenderHAProxyServiceStepBuilder).Init(s)
}

func (s *RenderHAProxyServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderHAProxyServiceStep) render(ctx runtime.ExecutionContext) (string, error) {
	tmplContent, err := templates.Get("loadbalancer/haproxy/haproxy.service.tmpl")
	if err != nil {
		// Fallback to hardcoded service file if template doesn't exist
		return `[Unit]
Description=HAProxy Load Balancer
After=network-online.target
Wants=network-online.target

[Service]
Environment="CONFIG=/etc/haproxy/haproxy.cfg" "PIDFILE=/run/haproxy.pid"
ExecStartPre=/usr/sbin/haproxy -f $CONFIG -c -q
ExecStart=/usr/sbin/haproxy -Ws -f $CONFIG -p $PIDFILE
ExecReload=/usr/sbin/haproxy -f $CONFIG -c -q
ExecReload=/bin/kill -USR2 $MAINPID
Restart=on-failure
SuccessExitStatus=143
KillMode=mixed
Type=notify

[Install]
WantedBy=multi-user.target
`, nil
	}

	return templates.Render(tmplContent, nil)
}

func (s *RenderHAProxyServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	// Always render to ensure content is in context
	return false, nil
}

func (s *RenderHAProxyServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	content, err := s.render(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render HAProxy service file")
		return result, err
	}

	// Export to context for CopyFileStep to use
	ctx.Export("task", "haproxy_worker_service", content)

	logger.Info("HAProxy systemd service file rendered and exported to context.")
	result.MarkCompleted("HAProxy service file rendered successfully")
	return result, nil
}

func (s *RenderHAProxyServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	// Nothing to rollback - just context export
	return nil
}

var _ step.Step = (*RenderHAProxyServiceStep)(nil)
