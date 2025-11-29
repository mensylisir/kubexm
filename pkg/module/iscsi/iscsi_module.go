package iscsi

import (
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
)

// ISCSIModule defines the module for managing iSCSI client tools.
type ISCSIModule struct {
	module.BaseModule
}

// NewISCSIModule creates a new ISCSIModule.
func NewISCSIModule() module.Module {
	// Stub implementation for now
	base := module.NewBaseModule("ISCSIClientManagement", nil)
	return &ISCSIModule{BaseModule: base}
}

func (m *ISCSIModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	// Stub: always return empty fragment for now as it was disabled in original code
	logger.Info("ISCSI module is currently disabled/stubbed.")
	return plan.NewEmptyFragment(m.Name()), nil
}

// GetTasks returns the list of tasks for the ISCSIModule.
func (m *ISCSIModule) GetTasks(ctx runtime.ModuleContext) ([]task.Task, error) {
	return []task.Task{}, nil
}

var _ module.Module = (*ISCSIModule)(nil)
