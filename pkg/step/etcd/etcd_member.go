package etcd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// AddEtcdMemberStepSpec defines parameters for adding a new member to the etcd cluster.
type AddEtcdMemberStepSpec struct {
	spec.StepMeta // Embed common meta fields

	MemberName      string `json:"memberName,omitempty"`      // Name for the new member
	MemberPeerURLs  string `json:"memberPeerURLs,omitempty"`  // Comma-separated peer URLs for the new member
	EtcdCtlEndpoint string `json:"etcdCtlEndpoint,omitempty"` // Endpoint for etcdctl, e.g., "http://127.0.0.1:2379"
	// Additional etcdctl flags like --cacert, --cert, --key could be added if needed
}

// NewAddEtcdMemberStepSpec creates a new AddEtcdMemberStepSpec.
func NewAddEtcdMemberStepSpec(stepName, memberName, memberPeerURLs, etcdCtlEndpoint string) *AddEtcdMemberStepSpec {
	if stepName == "" {
		stepName = fmt.Sprintf("Add etcd member %s", memberName)
	}
	ep := etcdCtlEndpoint
	if ep == "" {
		ep = "http://127.0.0.1:2379" // Default etcdctl endpoint
	}

	return &AddEtcdMemberStepSpec{
		StepMeta: spec.StepMeta{
			Name:        stepName,
			Description: fmt.Sprintf("Adds member '%s' with peer URLs '%s' to the etcd cluster via endpoint '%s'.", memberName, memberPeerURLs, ep),
		},
		MemberName:      memberName,
		MemberPeerURLs:  memberPeerURLs,
		EtcdCtlEndpoint: ep,
	}
}

// GetName returns the step's name.
func (s *AddEtcdMemberStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
func (s *AddEtcdMemberStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec.
func (s *AddEtcdMemberStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *AddEtcdMemberStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Name returns the step's name (implementing step.Step).
func (s *AddEtcdMemberStepSpec) Name() string { return s.GetName() }

// Description returns the step's description (implementing step.Step).
func (s *AddEtcdMemberStepSpec) Description() string { return s.GetDescription() }

// Precheck checks if the member already exists in the cluster.
func (s *AddEtcdMemberStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	if s.MemberName == "" || s.MemberPeerURLs == "" {
		return false, fmt.Errorf("MemberName and MemberPeerURLs must be specified for AddEtcdMember: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// etcdctl command to list members. Output format: ID, status, name, peerURLs, clientURLs, isLearner
	// Example: 8e9e05c52164694d, started, infra1, http://10.0.1.10:2380, http://10.0.1.10:2379, false
	cmd := fmt.Sprintf("etcdctl --endpoints=%s member list", s.EtcdCtlEndpoint)
	logger.Debug("Checking existing members.", "command", cmd)
	stdout, stderr, err := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{})
	if err != nil {
		logger.Warn("Failed to list etcd members, will attempt to add.", "error", err, "stderr", string(stderr))
		return false, nil
	}

	memberList := string(stdout)
	lines := strings.Split(memberList, "\n")
	for _, line := range lines {
		if strings.Contains(line, s.MemberName) && strings.Contains(line, s.MemberPeerURLs) {
			logger.Info("Etcd member already exists with the same name and peer URLs.", "name", s.MemberName, "peerURLs", s.MemberPeerURLs)
			return true, nil
		}
		// More sophisticated check might parse member ID and compare just peerURLs if name can change or is not unique before join.
	}

	logger.Info("Etcd member does not appear to exist or does not match specification. Add will proceed.", "name", s.MemberName)
	return false, nil
}

// Run executes the etcdctl member add command.
func (s *AddEtcdMemberStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	if s.MemberName == "" || s.MemberPeerURLs == "" {
		return fmt.Errorf("MemberName and MemberPeerURLs must be specified for AddEtcdMember: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmd := fmt.Sprintf("etcdctl --endpoints=%s member add %s --peer-urls=%s",
		s.EtcdCtlEndpoint, s.MemberName, s.MemberPeerURLs)

	logger.Info("Adding etcd member.", "command", cmd)
	// etcdctl member add usually needs to be run on one of the existing members.
	// The host parameter here should be one of those members.
	// Output of 'member add' includes ETCD_INITIAL_CLUSTER and ETCD_INITIAL_CLUSTER_STATE for the new member.
	// This output might need to be captured and stored in StepCache or TaskCache if subsequent steps need it.
	stdout, stderr, err := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{})
	if err != nil {
		return fmt.Errorf("failed to add etcd member '%s' (stderr: %s): %w", s.MemberName, string(stderr), err)
	}

	logger.Info("Etcd member add command executed.", "stdout", string(stdout), "stderr", string(stderr))
	// Example: Parse stdout for member ID and cache it if needed for rollback or other operations.
	// ETCD_INITIAL_CLUSTER='infra2=http://10.0.1.12:2380,infra1=http://10.0.1.11:2380,infra0=http://10.0.1.10:2380'
    // ETCD_INITIAL_CLUSTER_STATE='existing'
    // The command also prints "Added member XXXXXXX to cluster YYYYYYYY"
    // For now, just logging.
	return nil
}

// Rollback for adding a member is complex as it might require knowing the member ID.
// For simplicity, this is a no-op with a warning.
func (s *AddEtcdMemberStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for AddEtcdMember is not automatically implemented. Manual removal of member may be required if add was partially successful or if desired.")
	// To implement, would need memberID. `etcdctl member remove <id>`
	return nil
}

var _ step.Step = (*AddEtcdMemberStepSpec)(nil)

// RemoveEtcdMemberStepSpec defines parameters for removing a member from the etcd cluster.
type RemoveEtcdMemberStepSpec struct {
	spec.StepMeta // Embed common meta fields

	MemberID        string `json:"memberID,omitempty"`        // ID of the member to remove
	EtcdCtlEndpoint string `json:"etcdCtlEndpoint,omitempty"` // Endpoint for etcdctl
}

// NewRemoveEtcdMemberStepSpec creates a new RemoveEtcdMemberStepSpec.
func NewRemoveEtcdMemberStepSpec(stepName, memberID, etcdCtlEndpoint string) *RemoveEtcdMemberStepSpec {
	if stepName == "" {
		stepName = fmt.Sprintf("Remove etcd member %s", memberID)
	}
	ep := etcdCtlEndpoint
	if ep == "" {
		ep = "http://127.0.0.1:2379" // Default etcdctl endpoint
	}
	return &RemoveEtcdMemberStepSpec{
		StepMeta: spec.StepMeta{
			Name:        stepName,
			Description: fmt.Sprintf("Removes member with ID '%s' from the etcd cluster via endpoint '%s'.", memberID, ep),
		},
		MemberID:        memberID,
		EtcdCtlEndpoint: ep,
	}
}

// GetName returns the step's name.
func (s *RemoveEtcdMemberStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
func (s *RemoveEtcdMemberStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec.
func (s *RemoveEtcdMemberStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *RemoveEtcdMemberStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Name returns the step's name (implementing step.Step).
func (s *RemoveEtcdMemberStepSpec) Name() string { return s.GetName() }

// Description returns the step's description (implementing step.Step).
func (s *RemoveEtcdMemberStepSpec) Description() string { return s.GetDescription() }

// Precheck checks if the member still exists in the cluster.
func (s *RemoveEtcdMemberStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	if s.MemberID == "" {
		return false, fmt.Errorf("MemberID must be specified for RemoveEtcdMember: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmd := fmt.Sprintf("etcdctl --endpoints=%s member list", s.EtcdCtlEndpoint)
	logger.Debug("Checking existing members.", "command", cmd)
	stdout, stderr, err := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{})
	if err != nil {
		// If listing members fails, it's hard to know the state. Assume removal is needed or let Run handle it.
		logger.Warn("Failed to list etcd members, will attempt to remove.", "error", err, "stderr", string(stderr))
		return false, nil
	}

	memberList := string(stdout)
	if strings.Contains(memberList, s.MemberID) {
		logger.Info("Etcd member ID found in cluster. Removal will proceed.", "memberID", s.MemberID)
		return false, nil // Member exists, so "done" is false (removal is needed)
	}

	logger.Info("Etcd member ID not found in cluster. Assuming already removed.", "memberID", s.MemberID)
	return true, nil // Member does not exist, so "done" is true
}

// Run executes the etcdctl member remove command.
func (s *RemoveEtcdMemberStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	if s.MemberID == "" {
		return fmt.Errorf("MemberID must be specified for RemoveEtcdMember: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmd := fmt.Sprintf("etcdctl --endpoints=%s member remove %s", s.EtcdCtlEndpoint, s.MemberID)
	logger.Info("Removing etcd member.", "command", cmd)
	_, stderr, err := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{})
	if err != nil {
		// Check if error indicates member not found - this could be considered success for removal.
		if strings.Contains(string(stderr), "member not found") || strings.Contains(string(stderr), "is not a member of cluster") {
			logger.Info("Etcd member remove command indicates member already removed or not found.", "memberID", s.MemberID, "stderr", string(stderr))
			return nil // Idempotency: if member is not there, it's effectively removed.
		}
		return fmt.Errorf("failed to remove etcd member '%s' (stderr: %s): %w", s.MemberID, string(stderr), err)
	}

	logger.Info("Etcd member removed successfully.", "memberID", s.MemberID)
	return nil
}

// Rollback for removing a member is generally a no-op, as re-adding is a complex, stateful operation.
func (s *RemoveEtcdMemberStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for RemoveEtcdMember is a no-op. Re-adding a member requires specific configuration and is not done automatically.")
	return nil
}

var _ step.Step = (*RemoveEtcdMemberStepSpec)(nil)
