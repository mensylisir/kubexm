package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"strings"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CheckHostConnectivityStep struct {
	step.Base
}

type CheckHostConnectivityStepBuilder struct {
	step.Builder[CheckHostConnectivityStepBuilder, *CheckHostConnectivityStep]
}

func NewCheckHostConnectivityStepBuilder(ctx runtime.Context, instanceName string) *CheckHostConnectivityStepBuilder {
	s := &CheckHostConnectivityStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check network connectivity between nodes on critical ports"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CheckHostConnectivityStepBuilder).Init(s)
	return b
}

func (s *CheckHostConnectivityStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckHostConnectivityStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for host connectivity check...")
	logger.Info("Precheck passed: Host connectivity check will always be attempted.")
	return false, nil
}

func (s *CheckHostConnectivityStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Checking network connectivity to other nodes...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	currentHost := ctx.GetHost()
	allHosts := ctx.GetHostsByRole("")
	clusterSpec := ctx.GetClusterConfig().Spec

	var wg sync.WaitGroup
	errChan := make(chan error, len(allHosts))

	for _, targetHost := range allHosts {
		if targetHost.GetName() == currentHost.GetName() {
			continue
		}

		wg.Add(1)
		go func(target connector.Host) {
			defer wg.Done()

			log := logger.With("target_host", target.GetName(), "target_address", target.GetAddress())

			portsToCheck := s.getPortsForHost(clusterSpec, target)
			if len(portsToCheck) == 0 {
				log.Info("No critical ports to check for this host's roles.")
				return
			}

			log.Infof("Checking connectivity to ports: %v", portsToCheck)

			for _, port := range portsToCheck {
				cmd := fmt.Sprintf("nc -z -w 1 %s %d", target.GetAddress(), port)

				if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
					errChan <- fmt.Errorf("cannot connect from %s to %s:%d", currentHost.GetName(), target.GetAddress(), port)
					return
				}
			}
			log.Info("All critical ports are reachable.")

		}(targetHost)
	}

	wg.Wait()
	close(errChan)

	var allErrors []string
	for e := range errChan {
		allErrors = append(allErrors, e.Error())
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("one or more connectivity checks failed: %s", strings.Join(allErrors, "; "))
	}

	logger.Info("All connectivity checks to other nodes passed.")
	return nil
}

func (s *CheckHostConnectivityStep) getPortsForHost(spec *v1alpha1.ClusterSpec, host connector.Host) []int {
	ports := make(map[int]bool)

	isMaster := false
	isEtcd := false

	if spec.RoleGroups != nil {
		for _, name := range spec.RoleGroups.Master {
			if name == host.GetName() {
				isMaster = true
				break
			}
		}
		for _, name := range spec.RoleGroups.Etcd {
			if name == host.GetName() {
				isEtcd = true
				break
			}
		}
	}

	if host.GetPort() > 0 {
		ports[host.GetPort()] = true
	} else if spec.Global != nil && spec.Global.Port > 0 {
		ports[spec.Global.Port] = true
	} else {
		ports[22] = true
	}

	if isMaster {
		if spec.ControlPlaneEndpoint != nil && spec.ControlPlaneEndpoint.Port > 0 {
			ports[spec.ControlPlaneEndpoint.Port] = true
		} else {
			ports[6443] = true
		}
	}

	if isEtcd {
		ports[2379] = true
		ports[2380] = true
	}

	portList := make([]int, 0, len(ports))
	for p := range ports {
		portList = append(portList, p)
	}
	return portList
}

func (s *CheckHostConnectivityStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a check-only step.")
	return nil
}

var _ step.Step = (*CheckHostConnectivityStep)(nil)
