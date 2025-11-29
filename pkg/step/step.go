package step

import (
	"time"

	"github.com/mensylisir/kubexm/pkg/spec"
)

type Base struct {
	Meta        spec.StepMeta
	Sudo        bool
	Timeout     time.Duration
	IgnoreError bool
}

func (b *Base) GetBase() *Base {
	return b
}
