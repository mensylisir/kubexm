package etcd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/spec"
)

// ServiceAction defines the action to be performed on a service.
type ServiceAction string

const (
	ServiceActionStart   ServiceAction = "start"
	ServiceActionStop    ServiceAction = "stop"
	ServiceActionRestart ServiceAction = "restart"
	ServiceActionEnable  ServiceAction = "enable"
	ServiceActionDisable ServiceAction = "disable"
	ServiceActionReload  ServiceAction = "daemon-reload" // For systemd daemon-reload
)

// ManageEtcdServiceStepSpec defines parameters for managing the etcd systemd service.
type ManageEtcdServiceStepSpec struct {
	spec.StepMeta // Embed common meta fields

	ServiceName string        `json:"serviceName,omitempty"` // Name of the etcd service, e.g., "etcd"
	Action      ServiceAction `json:"action,omitempty"`      // e.g., "start", "stop", "enable"
}

// NewManageEtcdServiceStepSpec creates a new ManageEtcdServiceStepSpec.
func NewManageEtcdServiceStepSpec(stepName, serviceName string, action ServiceAction) *ManageEtcdServiceStepSpec {
	sName := serviceName
	if sName == "" {
		sName = "etcd" // Default etcd service name
	}

	if stepName == "" {
		stepName = fmt.Sprintf("%s %s service", strings.Title(string(action)), sName)
	}

	return &ManageEtcdServiceStepSpec{
		StepMeta: spec.StepMeta{
			Name:        stepName,
			Description: fmt.Sprintf("Performs action '%s' on the %s systemd service.", action, sName),
		},
		ServiceName: sName,
		Action:      action,
	}
}

// GetName returns the step's name.
func (s *ManageEtcdServiceStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
func (s *ManageEtcdServiceStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec validates and returns the spec.
func (s *ManageEtcdServiceStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ManageEtcdServiceStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }
