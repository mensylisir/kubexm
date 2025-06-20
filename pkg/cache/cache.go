package cache

import "sync"

// PipelineCache stores data scoped to a pipeline's execution.
type PipelineCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
}

// ModuleCache stores data scoped to a module's execution.
type ModuleCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
}

// TaskCache stores data scoped to a task's execution.
type TaskCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
}

// StepCache stores data scoped to a step's execution, including the current step's spec.
type StepCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
	SetCurrentStepSpec(spec interface{}) // spec is of type spec.StepSpec, but use interface{} to avoid cycle
	GetCurrentStepSpec() (interface{}, bool) // returns spec.StepSpec, but use interface{}
}

// genericCache provides a thread-safe key-value store.
type genericCache struct {
	store sync.Map
}

// NewGenericCache creates a new genericCache.
func NewGenericCache() *genericCache {
	return &genericCache{}
}

// Get retrieves a value from the cache.
func (c *genericCache) Get(key string) (interface{}, bool) {
	return c.store.Load(key)
}

// Set stores a value in the cache.
func (c *genericCache) Set(key string, value interface{}) {
	c.store.Store(key, value)
}

// Delete removes a value from the cache.
func (c *genericCache) Delete(key string) {
	c.store.Delete(key)
}

const currentStepSpecKey = "_currentStepSpec"

// SetCurrentStepSpec stores the current step's specification.
func (c *genericCache) SetCurrentStepSpec(spec interface{}) {
	c.store.Store(currentStepSpecKey, spec)
}

// GetCurrentStepSpec retrieves the current step's specification.
// The caller is responsible for type asserting the returned interface{}.
func (c *genericCache) GetCurrentStepSpec() (interface{}, bool) {
	return c.store.Load(currentStepSpecKey)
}

// NewPipelineCache creates a new PipelineCache.
func NewPipelineCache() PipelineCache {
	return NewGenericCache()
}

// NewModuleCache creates a new ModuleCache.
func NewModuleCache() ModuleCache {
	return NewGenericCache()
}

// NewTaskCache creates a new TaskCache.
func NewTaskCache() TaskCache {
	return NewGenericCache()
}

// NewStepCache creates a new StepCache.
func NewStepCache() StepCache {
	return NewGenericCache()
}
