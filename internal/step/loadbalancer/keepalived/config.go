package keepalived

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

// GenerateKeepalivedConfig generates Keepalived configuration
type GenerateKeepalivedConfig struct {
	step.Base
}

type GenerateKeepalivedConfigStepBuilder struct {
	step.Builder[GenerateKeepalivedConfigStepBuilder, *GenerateKeepalivedConfig]
}

type KeepalivedConfigData struct {
	State               string
	Interface           string
	VirtualRouterID     int
	Priority            int
	AuthenticationPass  string
	UnicastSrcIP        string
	UnicastPeers        []string
	VirtualIP           string
}

func NewGenerateKeepalivedConfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *GenerateKeepalivedConfigStepBuilder {
	s := &GenerateKeepalivedConfig{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Keepalived configuration", s.Base.Meta.Name)
	s.Base.Timeout = 2 * time.Minute

	b := new(GenerateKeepalivedConfigStepBuilder).Init(s)
	return b
}

func (s *GenerateKeepalivedConfig) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *GenerateKeepalivedConfig) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *GenerateKeepalivedConfig) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
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
		State:               state,
		Interface:           facts.DefaultInterface,
		VirtualRouterID:     60,
		Priority:            priority,
		AuthenticationPass:  "1111",
		UnicastSrcIP:        facts.IPv4Default,
		UnicastPeers:        peers,
		VirtualIP:           cluster.Spec.ControlPlaneEndpoint.Address,
	}

	templateContent, err := templates.Get("loadbalancer/keepalived/keepalived.conf.tmpl")
	if err != nil {
		result.MarkFailed(err, "failed to get keepalived config template")
		return result, err
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		result.MarkFailed(err, "failed to render keepalived config")
		return result, err
	}

	// Publish to DataBus for downstream steps
	dm := runtime.NewDataManager(ctx)
	dm.Publish("keepalived.config", renderedConfig)

	result.MarkCompleted("Keepalived config generated successfully")
	return result, nil
}

func (s *GenerateKeepalivedConfig) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*GenerateKeepalivedConfig)(nil)

// DeployKeepalivedConfig deploys Keepalived configuration to host
type DeployKeepalivedConfig struct {
	step.Base
}

type DeployKeepalivedConfigStepBuilder struct {
	step.Builder[DeployKeepalivedConfigStepBuilder, *DeployKeepalivedConfig]
}

func NewDeployKeepalivedConfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DeployKeepalivedConfigStepBuilder {
	s := &DeployKeepalivedConfig{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Deploy Keepalived configuration", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 2 * time.Minute

	b := new(DeployKeepalivedConfigStepBuilder).Init(s)
	return b
}

func (s *DeployKeepalivedConfig) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *DeployKeepalivedConfig) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *DeployKeepalivedConfig) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	dm := runtime.NewDataManager(ctx)

	config, _ := dm.Subscribe("keepalived.config")

	remoteConfigDir := common.KeepalivedDefaultConfDirTarget
	ctx.GetRunner().Mkdirp(ctx.GoContext(), conn, remoteConfigDir, "0755", true)

	remoteConfigPath := filepath.Join(remoteConfigDir, common.DefaultKeepalivedConfig)
	err := helpers.WriteContentToRemote(ctx, conn, config.(string), remoteConfigPath, "0644", true)
	if err != nil {
		result.MarkFailed(err, "failed to deploy keepalived config")
		return result, err
	}
	result.MarkCompleted("Keepalived config deployed successfully")
	return result, nil
}

func (s *DeployKeepalivedConfig) Rollback(ctx runtime.ExecutionContext) error {
	conn, _ := ctx.GetCurrentHostConnector()
	ctx.GetRunner().Remove(ctx.GoContext(), conn, common.KeepalivedDefaultConfigFileTarget, true, false)
	return nil
}

var _ step.Step = (*DeployKeepalivedConfig)(nil)