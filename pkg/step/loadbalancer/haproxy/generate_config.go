package haproxy

import (
	"bytes"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateHAProxyConfigStep struct {
	step.Base
}
type GenerateHAProxyConfigStepBuilder struct {
	step.Builder[GenerateHAProxyConfigStepBuilder, *GenerateHAProxyConfigStep]
}

func NewGenerateHAProxyConfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateHAProxyConfigStepBuilder {
	s := &GenerateHAProxyConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate HAProxy configuration for API Server Load Balancer", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateHAProxyConfigStepBuilder).Init(s)
	return b
}
func (s *GenerateHAProxyConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type HaproxyTemplateData struct {
	FrontendBindAddress string
	FrontendBindPort    int
	BackendServers      []BackendServer
}

type BackendServer struct {
	Name    string
	Address string
	Port    int
}

func (s *GenerateHAProxyConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	cluster := ctx.GetClusterConfig()

	masterNodes := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterNodes) == 0 {
		return nil, fmt.Errorf("no master nodes found to generate HAProxy backend servers")
	}

	data := HaproxyTemplateData{
		FrontendBindAddress: cluster.Spec.ControlPlaneEndpoint.Address,
		FrontendBindPort:    cluster.Spec.ControlPlaneEndpoint.Port,
		BackendServers:      make([]BackendServer, len(masterNodes)),
	}

	for i, node := range masterNodes {
		data.BackendServers[i] = BackendServer{
			Name:    node.GetName(),
			Address: node.GetInternalAddress(),
			Port:    common.DefaultAPIServerPort,
		}
	}

	templateContent, err := templates.Get("haproxy/haproxy.cfg.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get haproxy config template: %w", err)
	}
	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render haproxy config template: %w", err)
	}
	return []byte(renderedConfig), nil
}

func (s *GenerateHAProxyConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteConfigPath := common.HAProxyDefaultConfigFileTarget
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s': %w", remoteConfigPath, err)
	}
	if !exists {
		logger.Info("HAProxy config file does not exist. Step needs to run.")
		return false, nil
	}

	expectedContent, err := s.renderContent(ctx)
	if err != nil {
		return false, err
	}
	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to read remote config file '%s': %w", remoteConfigPath, err)
	}

	if bytes.Equal(bytes.TrimSpace(remoteContent), bytes.TrimSpace(expectedContent)) {
		logger.Info("HAProxy configuration is already up-to-date.")
		return true, nil
	}

	logger.Info("HAProxy configuration differs. Step needs to run to update it.")
	return false, nil
}

func (s *GenerateHAProxyConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	renderedConfig, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	remoteConfigDir := common.HAProxyDefaultConfDirTarget
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true); err != nil {
		return fmt.Errorf("failed to create remote directory '%s' with sudo: %w", remoteConfigDir, err)
	}

	remoteConfigPath := filepath.Join(remoteConfigDir, "haproxy.cfg")
	logger.Infof("Uploading HAProxy configuration to %s:%s", ctx.GetHost().GetName(), remoteConfigPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteConfigPath, "0644", true); err != nil {
		return fmt.Errorf("failed to upload HAProxy config file with sudo: %w", err)
	}

	logger.Info("HAProxy configuration generated and uploaded successfully.")
	return nil
}

func (s *GenerateHAProxyConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteConfigPath := common.HAProxyDefaultConfigFileTarget
	logger.Warnf("Rolling back by removing: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteConfigPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateHAProxyConfigStep)(nil)
