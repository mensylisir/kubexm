package cache

import (
	"fmt"
	"sync"
)

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

// genericCache provides a thread-safe key-value store with optional parent for fallback reads.
type genericCache struct {
	store  sync.Map
	parent *genericCache // Pointer to parent cache for inherited reads
}

// NewGenericCache creates a new genericCache.
// Parent can be nil for top-level caches (e.g., PipelineCache).
func NewGenericCache(parent *genericCache) *genericCache {
	return &genericCache{
		parent: parent,
	}
}

// Get retrieves a value from the cache.
// It first checks the current cache, then falls back to the parent cache if the key is not found locally.
func (c *genericCache) Get(key string) (interface{}, bool) {
	if val, ok := c.store.Load(key); ok {
		return val, true
	}
	if c.parent != nil {
		return c.parent.Get(key) // Recursive call to parent's Get
	}
	return nil, false
}

// Set stores a value in the cache. Writes are always local to the current cache instance.
func (c *genericCache) Set(key string, value interface{}) {
	c.store.Store(key, value)
}

// SetParent sets the parent cache for the current cache.
// This is intended to be used by the runtime to establish the cache hierarchy.
func (c *genericCache) SetParent(parent *genericCache) {
	c.parent = parent
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
	return NewGenericCache(nil)
}

// NewModuleCache creates a new ModuleCache with the given PipelineCache as its parent.
func NewModuleCache(parent PipelineCache) ModuleCache {
	var parentGenericCache *genericCache
	if parent != nil {
		var ok bool
		parentGenericCache, ok = parent.(*genericCache)
		if !ok {
			panic(fmt.Sprintf("NewModuleCache: parent is not of type *genericCache, got %T", parent))
		}
	}
	return NewGenericCache(parentGenericCache)
}

// NewTaskCache creates a new TaskCache with the given ModuleCache as its parent.
func NewTaskCache(parent ModuleCache) TaskCache {
	var parentGenericCache *genericCache
	if parent != nil {
		var ok bool
		parentGenericCache, ok = parent.(*genericCache)
		if !ok {
			panic(fmt.Sprintf("NewTaskCache: parent is not of type *genericCache, got %T", parent))
		}
	}
	return NewGenericCache(parentGenericCache)
}

// NewStepCache creates a new StepCache with the given TaskCache as its parent.
func NewStepCache(parent TaskCache) StepCache {
	var parentGenericCache *genericCache
	if parent != nil {
		var ok bool
		parentGenericCache, ok = parent.(*genericCache)
		if !ok {
			panic(fmt.Sprintf("NewStepCache: parent is not of type *genericCache, got %T", parent))
		}
	}
	return NewGenericCache(parentGenericCache)
}
