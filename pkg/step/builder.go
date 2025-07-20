package step

import (
	"time"
)

type Builder[B any, T Step] struct {
	Step T
}

func (b *Builder[B, T]) Init(step T) *B {
	b.Step = step
	return b.this()
}

func (b *Builder[B, T]) this() *B {
	return any(b).(*B)
}

func (b *Builder[B, T]) Build() T {
	return b.Step
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
