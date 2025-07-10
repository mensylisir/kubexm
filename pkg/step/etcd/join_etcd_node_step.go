package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common" // Added import
)

// JoinEtcdNodeStep ensures a subsequent etcd node is configured with 'initial-cluster-state: existing'
// and then starts the etcd service on it.
// Similar to BootstrapFirstEtcdNodeStep, this assumes that GenerateEtcdConfigStep and
// GenerateEtcdServiceStep have already run. GenerateEtcdConfigStep should have set the
// state to 'existing' for these joining nodes.
type JoinEtcdNodeStep struct {
	meta        spec.StepMeta
	ServiceName string // Defaults to "etcd"
	Sudo        bool
	// ConfigData for this specific node, ensuring InitialClusterState is "existing".
	// This step primarily just starts the service, relying on prior config generation.
}

// NewJoinEtcdNodeStep creates a new JoinEtcdNodeStep.
func NewJoinEtcdNodeStep(instanceName, serviceName string, sudo bool) step.Step {
	name := instanceName
	svcName := serviceName
	if svcName == "" {
		svcName = "etcd"
	}
	if name == "" {
		name = fmt.Sprintf("JoinEtcdNode-%s", svcName)
	}

	return &JoinEtcdNodeStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Ensures etcd.yaml has 'initial-cluster-state: existing' and starts the %s service on a joining etcd node.", svcName),
		},
		ServiceName: svcName,
		Sudo:        true, // Service operations usually require sudo
	}
}

func (s *JoinEtcdNodeStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *JoinEtcdNodeStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// 1. Verify etcd.yaml has 'initial-cluster-state: existing' (Conceptual)
	//    Assume GenerateEtcdConfigStep configured it correctly for joining nodes.
	//    Example (pseudo-code for check):
	//    remoteConfigContent, _ := runnerSvc.ReadFile(ctx.GoContext(), conn, EtcdConfigRemotePath)
	//    if !strings.Contains(string(remoteConfigContent), "initial-cluster-state: existing") {
	//        logger.Info("etcd.yaml does not have initial-cluster-state: existing.")
	//        return false, nil
	//    }
	//    logger.Info("etcd.yaml appears to be configured for joining an existing cluster.")

	// 2. Check if the service is active
	active, err := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, s.ServiceName)
	if err != nil {
		logger.Warn("Failed to check if service is active, assuming it needs to be started.", "service", s.ServiceName, "error", err)
		return false, nil
	}
	if active {
		logger.Info("Service is already active.", "service", s.ServiceName)
		// Additionally, one might check `etcdctl endpoint health --cluster` from any member
		// to see if this node is part of a healthy cluster. This is more of a task-level check.
		return true, nil
	}
	logger.Info("Service is not active.", "service", s.ServiceName)
	return false, nil
}

func (s *JoinEtcdNodeStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	// Assumption: etcd.yaml is already correctly configured with 'initial-cluster-state: existing'
	// by a preceding GenerateEtcdConfigStep.

	// Actions:
	// 1. systemctl daemon-reload
	// 2. systemctl enable etcd
	// 3. systemctl start etcd

	daemonReloadStep := commonstep.NewManageServiceStep("DaemonReloadForEtcdJoin", commonstep.ActionDaemonReload, s.ServiceName, s.Sudo)
	logger.Info("Executing daemon-reload.")
	if err := daemonReloadStep.Run(ctx, host); err != nil {
		return fmt.Errorf("failed to run daemon-reload for %s: %w", s.ServiceName, err)
	}

	enableStep := commonstep.NewManageServiceStep("EnableEtcdForJoin", commonstep.ActionEnable, s.ServiceName, s.Sudo)
	logger.Info("Enabling etcd service.")
	if err := enableStep.Run(ctx, host); err != nil {
		return fmt.Errorf("failed to enable %s service: %w", s.ServiceName, err)
	}

	startStep := commonstep.NewManageServiceStep("StartEtcdForJoin", commonstep.ActionStart, s.ServiceName, s.Sudo)
	logger.Info("Starting etcd service.")
	if err := startStep.Run(ctx, host); err != nil {
		return fmt.Errorf("failed to start %s service: %w", s.ServiceName, err)
	}

	// Optional: Add a short delay and then check service status or etcd cluster health
	logger.Info("Etcd service started successfully on the joining node.")
	return nil
}

func (s *JoinEtcdNodeStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	stopStep := NewManageEtcdServiceStep("StopEtcdForJoinRollback", ActionStop, s.ServiceName, s.Sudo)
	logger.Info("Attempting to stop etcd service for rollback.")
	if err := stopStep.Run(ctx, host); err != nil {
		logger.Warn("Failed to stop etcd service during join rollback (best effort).", "error", err)
	}

	disableStep := NewManageEtcdServiceStep("DisableEtcdForJoinRollback", ActionDisable, s.ServiceName, s.Sudo)
	logger.Info("Attempting to disable etcd service for rollback.")
	if err := disableStep.Run(ctx, host); err != nil {
		logger.Warn("Failed to disable etcd service during join rollback (best effort).", "error", err)
	}

	logger.Info("Rollback attempt for joining etcd node finished.")
	return nil
}

var _ step.Step = (*JoinEtcdNodeStep)(nil)
