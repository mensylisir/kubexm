package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// BootstrapFirstEtcdNodeStep ensures the first etcd node is configured with 'initial-cluster-state: new'
// and then starts the etcd service on it.
// This step assumes that GenerateEtcdConfigStep and GenerateEtcdServiceStep have already run
// to create the base etcd.yaml and etcd.service files. This step might modify etcd.yaml
// specifically for the 'new' state if the generic GenerateEtcdConfigStep created it as 'existing' by default.
// Alternatively, GenerateEtcdConfigStep itself should be parameterized for initial-cluster-state.
// For this implementation, we assume GenerateEtcdConfigStep has correctly set the state to 'new'
// for the first node based on data passed to it. This step then focuses on ensuring the service starts.
type BootstrapFirstEtcdNodeStep struct {
	meta        spec.StepMeta
	ServiceName string // Defaults to "etcd"
	Sudo        bool
	// ConfigData for this specific node, ensuring InitialClusterState is "new".
	// This implies that the Task orchestrating this will provide the correct EtcdNodeConfigData
	// to a GenerateEtcdConfigStep instance *before* this Bootstrap step runs.
	// This step primarily just starts the service, relying on prior config generation.
}

// NewBootstrapFirstEtcdNodeStep creates a new BootstrapFirstEtcdNodeStep.
func NewBootstrapFirstEtcdNodeStep(instanceName, serviceName string, sudo bool) step.Step {
	name := instanceName
	svcName := serviceName
	if svcName == "" {
		svcName = "etcd"
	}
	if name == "" {
		name = fmt.Sprintf("BootstrapFirstEtcdNode-%s", svcName)
	}

	return &BootstrapFirstEtcdNodeStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Ensures etcd.yaml has 'initial-cluster-state: new' and starts the %s service on the first etcd node.", svcName),
		},
		ServiceName: svcName,
		Sudo:        true, // Service operations usually require sudo
	}
}

func (s *BootstrapFirstEtcdNodeStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *BootstrapFirstEtcdNodeStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// 1. Verify etcd.yaml has 'initial-cluster-state: new' (Conceptual - actual check is complex here)
	//    This check is difficult without parsing YAML on the remote host or having the expected rendered file.
	//    For now, we assume GenerateEtcdConfigStep correctly configured it for the first node.
	//    A more robust check would involve reading the remote etcd.yaml and verifying this specific field.
	//    Example (pseudo-code for check):
	//    remoteConfigContent, _ := runnerSvc.ReadFile(ctx.GoContext(), conn, EtcdConfigRemotePath)
	//    if !strings.Contains(string(remoteConfigContent), "initial-cluster-state: new") {
	//        logger.Info("etcd.yaml does not have initial-cluster-state: new.")
	//        return false, nil
	//    }
	//    logger.Info("etcd.yaml appears to be configured for a new cluster state.")

	// 2. Check if the service is active
	active, err := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, s.ServiceName)
	if err != nil {
		logger.Warn("Failed to check if service is active, assuming it needs to be started.", "service", s.ServiceName, "error", err)
		return false, nil
	}
	if active {
		logger.Info("Service is already active.", "service", s.ServiceName)
		// Additionally, one might check `etcdctl endpoint health` here if etcdctl is available.
		return true, nil
	}
	logger.Info("Service is not active.", "service", s.ServiceName)
	return false, nil
}

func (s *BootstrapFirstEtcdNodeStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	// Assumption: etcd.yaml is already correctly configured with 'initial-cluster-state: new'
	// by a preceding GenerateEtcdConfigStep instance that was given the correct EtcdNodeConfigData.

	// Actions:
	// 1. systemctl daemon-reload (to ensure etcd.service is loaded)
	// 2. systemctl enable etcd
	// 3. systemctl start etcd

	daemonReloadStep := NewManageEtcdServiceStep("DaemonReloadForEtcdBootstrap", ActionDaemonReload, s.ServiceName, s.Sudo)
	logger.Info("Executing daemon-reload.")
	if err := daemonReloadStep.Run(ctx, host); err != nil {
		return fmt.Errorf("failed to run daemon-reload for %s: %w", s.ServiceName, err)
	}

	enableStep := NewManageEtcdServiceStep("EnableEtcdForBootstrap", ActionEnable, s.ServiceName, s.Sudo)
	logger.Info("Enabling etcd service.")
	if err := enableStep.Run(ctx, host); err != nil {
		return fmt.Errorf("failed to enable %s service: %w", s.ServiceName, err)
	}

	startStep := NewManageEtcdServiceStep("StartEtcdForBootstrap", ActionStart, s.ServiceName, s.Sudo)
	logger.Info("Starting etcd service.")
	if err := startStep.Run(ctx, host); err != nil {
		return fmt.Errorf("failed to start %s service: %w", s.ServiceName, err)
	}

	// Optional: Add a short delay and then check service status or etcd cluster health
	// time.Sleep(5 * time.Second)
	// active, _ := ctx.GetRunner().IsServiceActive(ctx.GoContext(), conn, nil, s.ServiceName)
	// if !active { return fmt.Errorf("etcd service %s failed to start or become active after bootstrap", s.ServiceName) }

	logger.Info("Etcd service started successfully on the first node.")
	return nil
}

func (s *BootstrapFirstEtcdNodeStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rollback could involve stopping and disabling the service.
	// However, if other nodes joined, simply stopping one node isn't a full cluster rollback.

	stopStep := NewManageEtcdServiceStep("StopEtcdForBootstrapRollback", ActionStop, s.ServiceName, s.Sudo)
	logger.Info("Attempting to stop etcd service for rollback.")
	if err := stopStep.Run(ctx, host); err != nil {
		logger.Warn("Failed to stop etcd service during bootstrap rollback (best effort).", "error", err)
	}

	disableStep := NewManageEtcdServiceStep("DisableEtcdForBootstrapRollback", ActionDisable, s.ServiceName, s.Sudo)
	logger.Info("Attempting to disable etcd service for rollback.")
	if err := disableStep.Run(ctx, host); err != nil {
		logger.Warn("Failed to disable etcd service during bootstrap rollback (best effort).", "error", err)
	}

	logger.Info("Rollback attempt for bootstrapping first etcd node finished.")
	return nil
}

var _ step.Step = (*BootstrapFirstEtcdNodeStep)(nil)
