package module

import (
	"github.com/mensylisir/kubexm/pkg/spec"
	"time"
)

type Base struct {
	Meta        spec.ModuleMeta
	Timeout     time.Duration
	IgnoreError bool
}

func (b *Base) GetBase() *Base {
	return b
}
