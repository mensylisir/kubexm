package preflight

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CheckDNSConfigStep struct {
	step.Base
	resolvConfPath string
}

type CheckDNSConfigStepBuilder struct {
	step.Builder[CheckDNSConfigStepBuilder, *CheckDNSConfigStep]
}

func NewCheckDNSConfigStepBuilder(ctx runtime.Context, instanceName string) *CheckDNSConfigStepBuilder {
	s := &CheckDNSConfigStep{
		resolvConfPath: "/etc/resolv.conf",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check if the node has a valid DNS configuration in /etc/resolv.conf"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(CheckDNSConfigStepBuilder).Init(s)
	return b
}

func (s *CheckDNSConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckDNSConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for DNS configuration check...")
	logger.Info("Precheck passed: DNS configuration check will always be attempted.")
	return false, nil
}

func (s *CheckDNSConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Infof("Checking DNS configuration file: %s", s.resolvConfPath)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	fileContent, err := runner.ReadFile(ctx.GoContext(), conn, s.resolvConfPath)
	if err != nil {
		return fmt.Errorf("failed to read '%s' on host %s: %w", s.resolvConfPath, ctx.GetHost().GetName(), err)
	}

	contentStr := string(fileContent)
	if strings.TrimSpace(contentStr) == "" {
		return fmt.Errorf("'%s' is empty or contains only whitespace", s.resolvConfPath)
	}

	scanner := bufio.NewScanner(strings.NewReader(contentStr))
	var nameserverFound bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			ipStr := fields[1]
			ip := net.ParseIP(ipStr)
			if ip == nil {
				return fmt.Errorf("found an invalid IP address for a nameserver in '%s': %s", s.resolvConfPath, ipStr)
			}
			logger.Infof("Found valid nameserver: %s", ip.String())
			nameserverFound = true
		}
	}

	if !nameserverFound {
		return fmt.Errorf("no valid 'nameserver' entry found in '%s'", s.resolvConfPath)
	}

	logger.Infof("DNS configuration in '%s' is valid.", s.resolvConfPath)
	return nil
}

func (s *CheckDNSConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a check-only step.")
	return nil
}

var _ step.Step = (*CheckDNSConfigStep)(nil)
