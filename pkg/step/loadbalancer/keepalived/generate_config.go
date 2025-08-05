package keepalived

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

type GenerateKeepalivedConfigStep struct {
	step.Base
}
type GenerateKeepalivedConfigStepBuilder struct {
	step.Builder[GenerateKeepalivedConfigStepBuilder, *GenerateKeepalivedConfigStep]
}

func NewGenerateKeepalivedConfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateKeepalivedConfigStepBuilder {
	s := &GenerateKeepalivedConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Keepalived configuration for VIP high availability", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(GenerateKeepalivedConfigStepBuilder).Init(s)
	return b
}
func (s *GenerateKeepalivedConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type KeepalivedTemplateData struct {
	State              string
	Interface          string
	Priority           int
	VirtualRouterID    int
	AuthenticationPass string
	UnicastSrcIP       string
	UnicastPeers       []string
	VirtualIP          string
}

func (s *GenerateKeepalivedConfigStep) renderContent(ctx runtime.ExecutionContext) ([]byte, error) {
	logger := ctx.GetLogger().With("host", ctx.GetHost().GetName())
	cluster := ctx.GetClusterConfig()
	currentHost := ctx.GetHost()
	haSpec := cluster.Spec.ControlPlaneEndpoint.HighAvailability
	if haSpec == nil || haSpec.Enabled == nil || !*haSpec.Enabled ||
		haSpec.External == nil || haSpec.External.Enabled == nil || !*haSpec.External.Enabled {
		logger.Info("High availability or external load balancer is not enabled, skipping Keepalived configuration.")
		return nil, nil
	}

	lbNodes := ctx.GetHostsByRole(common.RoleLoadBalancer)
	if len(lbNodes) == 0 {
		return nil, fmt.Errorf("high availability is enabled, but no nodes with role '%s' were found", common.RoleLoadBalancer)
	}

	facts, err := ctx.GetHostFacts(currentHost)
	if err != nil {
		return nil, fmt.Errorf("failed to gather facts for host %s: %w", currentHost.GetName(), err)
	}

	data := KeepalivedTemplateData{
		VirtualRouterID:    60,
		AuthenticationPass: "1111",
		VirtualIP:          cluster.Spec.ControlPlaneEndpoint.Address,
		UnicastSrcIP:       facts.IPv4Default,
	}

	var iface string
	if haSpec.External.Keepalived != nil && len(haSpec.External.Keepalived.VRRPInstances) > 0 {
		iface = haSpec.External.Keepalived.VRRPInstances[0].Interface
	}
	if iface == "" {
		iface = facts.DefaultInterface
	}
	if iface == "" {
		return nil, fmt.Errorf("could not determine network interface for host %s from spec or facts", currentHost.GetName())
	}
	data.Interface = iface

	if currentHost.GetName() == lbNodes[0].GetName() {
		data.State = "MASTER"
		data.Priority = 100
	} else {
		data.State = "BACKUP"
		data.Priority = 100
	}
	if haSpec.External.Keepalived != nil && len(haSpec.External.Keepalived.VRRPInstances) > 0 {
		userInstance := haSpec.External.Keepalived.VRRPInstances[0]
		if userInstance.State != "" {
			data.State = userInstance.State
		}
		if userInstance.Priority != 0 {
			data.Priority = userInstance.Priority
		}
	}

	peers := make([]string, 0, len(lbNodes)-1)
	for _, node := range lbNodes {
		if node.GetName() != currentHost.GetName() {
			peers = append(peers, node.GetInternalAddress())
		}
	}
	data.UnicastPeers = peers
	templateContent, err := templates.Get("keepalived/keepalived.conf.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get keepalived config template: %w", err)
	}
	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render final keepalived config: %w", err)
	}

	return []byte(renderedConfig), nil
}

func (s *GenerateKeepalivedConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteConfigPath := common.KeepalivedDefaultConfigFileTarget
	exists, err := runner.Exists(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil || !exists {
		return false, err
	}

	expectedContent, err := s.renderContent(ctx)
	if err != nil {
		return false, err
	}
	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remoteConfigPath)
	if err != nil {
		return false, err
	}

	if bytes.Equal(bytes.TrimSpace(remoteContent), bytes.TrimSpace(expectedContent)) {
		logger.Info("Keepalived configuration is up to date.")
		return true, nil
	}

	logger.Info("Keepalived configuration differs. Step needs to run.")
	return false, nil
}

func (s *GenerateKeepalivedConfigStep) Run(ctx runtime.ExecutionContext) error {
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

	remoteConfigDir := common.KeepalivedDefaultConfDirTarget
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true); err != nil {
		return fmt.Errorf("failed to create remote directory '%s' with sudo: %w", remoteConfigDir, err)
	}

	remoteConfigPath := filepath.Join(remoteConfigDir, common.DefaultKeepalivedConfig)
	logger.Infof("Uploading Keepalived configuration to %s:%s", ctx.GetHost().GetName(), remoteConfigPath)
	if err := helpers.WriteContentToRemote(ctx, conn, string(renderedConfig), remoteConfigPath, "0644", true); err != nil {
		return fmt.Errorf("failed to upload Keepalived config file with sudo: %w", err)
	}

	logger.Info("Keepalived configuration generated and uploaded successfully.")
	return nil
}

func (s *GenerateKeepalivedConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteConfigPath := common.KeepalivedDefaultConfigFileTarget
	logger.Warnf("Rolling back by removing: %s", remoteConfigPath)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigPath, true, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", remoteConfigPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateKeepalivedConfigStep)(nil)
