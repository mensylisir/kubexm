package keepalived

import (
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

// ===================================================================
// GenerateKeepalivedConfig - 生成 Keepalived 配置
// ===================================================================

type GenerateKeepalivedConfig struct {
	step.Base
}

type KeepalivedConfigData struct {
	State              string
	Interface          string
	VirtualRouterID    int
	Priority           int
	AuthenticationPass string
	UnicastSrcIP       string
	UnicastPeers       []string
	VirtualIP          string
}

func NewGenerateKeepalivedConfig(ctx runtime.Context, name string) *GenerateKeepalivedConfig {
	s := &GenerateKeepalivedConfig{}
	s.Base.Meta.Name = name
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Keepalived configuration", name)
	s.Base.Timeout = 2 * time.Minute
	return s
}

func (s *GenerateKeepalivedConfig) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *GenerateKeepalivedConfig) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *GenerateKeepalivedConfig) Run(ctx runtime.ExecutionContext) error {
	cluster := ctx.GetClusterConfig()
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	lbNodes := ctx.GetHostsByRole(common.RoleLoadBalancer)

	state, priority := "BACKUP", 100
	if len(lbNodes) > 0 && ctx.GetHost().GetName() == lbNodes[0].GetName() {
		state, priority = "MASTER", 100
	}

	peers := make([]string, 0)
	for _, node := range lbNodes {
		if node.GetName() != ctx.GetHost().GetName() {
			peers = append(peers, node.GetInternalAddress())
		}
	}

	data := KeepalivedConfigData{
		State:              state,
		Interface:          facts.DefaultInterface,
		VirtualRouterID:    60,
		Priority:           priority,
		AuthenticationPass: "1111",
		UnicastSrcIP:       facts.IPv4Default,
		UnicastPeers:       peers,
		VirtualIP:          cluster.Spec.ControlPlaneEndpoint.Address,
	}

	templateContent, err := templates.Get("keepalived/keepalived.conf.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get keepalived config template: %w", err)
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render keepalived config: %w", err)
	}

	// Publish to DataBus for downstream steps
	dm := runtime.NewDataManager(ctx)
	dm.Publish("keepalived.config", renderedConfig)

	return nil
}

func (s *GenerateKeepalivedConfig) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

// ===================================================================
// DeployKeepalivedConfig - 部署 Keepalived 配置到主机
// ===================================================================

type DeployKeepalivedConfig struct {
	step.Base
}

func NewDeployKeepalivedConfig(ctx runtime.Context, name string) *DeployKeepalivedConfig {
	s := &DeployKeepalivedConfig{}
	s.Base.Meta.Name = name
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Deploy Keepalived configuration", name)
	s.Base.Sudo = true
	s.Base.Timeout = 2 * time.Minute
	return s
}

func (s *DeployKeepalivedConfig) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *DeployKeepalivedConfig) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *DeployKeepalivedConfig) Run(ctx runtime.ExecutionContext) error {
	conn, _ := ctx.GetCurrentHostConnector()
	dm := runtime.NewDataManager(ctx)

	config, _ := dm.Subscribe("keepalived.config")

	remoteConfigDir := common.KeepalivedDefaultConfDirTarget
	ctx.GetRunner().Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true)

	remoteConfigPath := filepath.Join(remoteConfigDir, common.DefaultKeepalivedConfig)
	return helpers.WriteContentToRemote(ctx, conn, config.(string), remoteConfigPath, "0644", true)
}

func (s *DeployKeepalivedConfig) Rollback(ctx runtime.ExecutionContext) error {
	conn, _ := ctx.GetCurrentHostConnector()
	ctx.GetRunner().Remove(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget, true, false)
	return nil
}
