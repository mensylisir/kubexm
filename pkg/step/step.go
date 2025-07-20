package step

import (
	"github.com/mensylisir/kubexm/pkg/spec"
	"time"
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
