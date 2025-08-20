package chrony

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type ChronyServerTemplateData struct {
	UpstreamServers []string
	AllowNetworks   []string
}

type ConfigureChronyAsServerStep struct {
	step.Base
	ChronyConfPath string
}

type ConfigureChronyAsServerStepBuilder struct {
	step.Builder[ConfigureChronyAsServerStepBuilder, *ConfigureChronyAsServerStep]
}

func NewConfigureChronyAsServerStepBuilder(ctx runtime.Context, instanceName string) *ConfigureChronyAsServerStepBuilder {
	s := &ConfigureChronyAsServerStep{
		ChronyConfPath: "/etc/chrony.conf",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Configure chronyd as an NTP server for the cluster"
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(ConfigureChronyAsServerStepBuilder).Init(s)
	return b
}

func (s *ConfigureChronyAsServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureChronyAsServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for chronyd server configuration...")
	logger.Info("Precheck passed: Configuration will be applied.")
	return false, nil
}

func (s *ConfigureChronyAsServerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Configuring chronyd as a server...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	clusterSpec := ctx.GetClusterConfig().Spec

	var upstreamServers []string
	if clusterSpec.System != nil && len(clusterSpec.System.NTPServers) > 0 {
		hostNames := make(map[string]bool)
		for _, host := range ctx.GetHostsByRole("") {
			hostNames[host.GetName()] = true
		}
		for _, server := range clusterSpec.System.NTPServers {
			if !hostNames[server] {
				upstreamServers = append(upstreamServers, server)
			}
		}
	}

	var allowNetworks []string
	if clusterSpec.Network != nil {
		if clusterSpec.Network.KubePodsCIDR != "" {
			allowNetworks = append(allowNetworks, clusterSpec.Network.KubePodsCIDR)
		}
		if clusterSpec.Network.KubeServiceCIDR != "" {
			allowNetworks = append(allowNetworks, clusterSpec.Network.KubeServiceCIDR)
		}
	}

	templateData := ChronyServerTemplateData{
		UpstreamServers: upstreamServers,
		AllowNetworks:   allowNetworks,
	}

	tmpl, err := templates.Get("chrony/chrony.server.conf.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get chrony server template: %w", err)
	}

	content, err := templates.Render(tmpl, templateData)
	if err != nil {
		return fmt.Errorf("failed to render chrony server config: %w", err)
	}

	logger.Infof("Writing chrony server configuration to %s", s.ChronyConfPath)
	if err := runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.ChronyConfPath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write chrony server config file: %w", err)
	}

	logger.Info("Chronyd server configuration applied successfully.")
	return nil
}

func (s *ConfigureChronyAsServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for chrony configuration is not implemented. Manual restore may be needed.")
	return nil
}

var _ step.Step = (*ConfigureChronyAsServerStep)(nil)
