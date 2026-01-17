package step

import (
	"fmt"
	"time"
)

type Builder[B any, T Step] struct {
	Step  T
	Error error
}

func (b *Builder[B, T]) Init(step T) *B {
	b.Step = step
	return b.this()
}

func (b *Builder[B, T]) this() *B {
	return any(b).(*B)
}

func (b *Builder[B, T]) Build() (T, error) {
	if b.Error != nil {
		var zero T
		return zero, fmt.Errorf("failed to build step: %v", b.Error)
	}
	return b.Step, nil
}

func (b *Builder[B, T]) Err() error {
	return b.Error
}
func (b *Builder[B, T]) WithSudo(sudo bool) *B {
	b.Step.GetBase().Sudo = sudo
	return b.this()
}

func (b *Builder[B, T]) WithTimeout(t time.Duration) *B {
	b.Step.GetBase().Timeout = t
	return b.this()
}

func (b *Builder[B, T]) WithIgnoreError(ignore bool) *B {
	b.Step.GetBase().IgnoreError = ignore
	return b.this()
}

func (b *Builder[B, T]) WithDescription(desc string) *B {
	b.Step.GetBase().Meta.Description = desc
	return b.this()
}

func (b *Builder[B, T]) WithName(name string) *B {
	b.Step.GetBase().Meta.Name = name
	return b.this()
}
