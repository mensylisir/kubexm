package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/spec"
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
