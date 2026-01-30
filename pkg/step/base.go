package step

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/types"
)

type Base struct {
	Meta        spec.StepMeta
	Timeout     time.Duration
	Sudo        bool
	IgnoreError bool
}

func (b *Base) GetBase() *Base {
	return b
}

// Validate returns nil by default - steps can override if they need validation
func (b *Base) Validate(ctx runtime.ExecutionContext) error {
	return nil
}

// Cleanup is a no-op implementation by default
// Step implementations can override this if they need to clean up resources
func (b *Base) Cleanup(ctx runtime.ExecutionContext) error {
	return nil
}

// Rollback returns an error by default - not implemented for most steps
// Step implementations can override this if they support rollback
func (b *Base) Rollback(ctx runtime.ExecutionContext) error {
	return fmt.Errorf("rollback not implemented for step: %s", b.Meta.Name)
}

// GetStatus returns pending status by default
// Step implementations can override this to provide actual status
func (b *Base) GetStatus(ctx runtime.ExecutionContext) (types.StepStatus, error) {
	return types.StepStatusPending, nil
}
