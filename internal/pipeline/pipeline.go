package pipeline

import (
	"github.com/mensylisir/kubexm/internal/spec"
	"time"
)

const (
	// DefaultPipelineTimeout is the default timeout for pipeline execution (30 minutes).
	DefaultPipelineTimeout = 30 * time.Minute
)

type Base struct {
	Meta        spec.PipelineMeta
	Timeout     time.Duration
	IgnoreError bool
}

// NewBase creates a new Base with sensible defaults.
func NewBase(name, description string) *Base {
	return &Base{
		Meta: spec.PipelineMeta{
			Name:        name,
			Description: description,
		},
		Timeout:     DefaultPipelineTimeout,
		IgnoreError: false,
	}
}

// GetTimeout returns the timeout duration, using default if not set.
func (b *Base) GetTimeout() time.Duration {
	if b.Timeout <= 0 {
		return DefaultPipelineTimeout
	}
	return b.Timeout
}

func (b *Base) GetBase() *Base {
	return b
}
