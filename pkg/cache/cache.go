package cache

import (
	"sync"

	"github.com/kubexms/kubexms/pkg/spec" // For spec.StepSpec
)

// === 1. Generic Cache Interface ===

// Cache is a generic interface for a key-value store.
type Cache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V)
	Delete(key K)
	Clear()
}

// === 2. Scoped Cache Interfaces ===

// PipelineCache provides a cache scoped to a pipeline's lifetime.
// Keys are strings, values can be any type.
type PipelineCache interface {
	Cache[string, any]
}

// ModuleCache provides a cache scoped to a module's execution within a pipeline.
type ModuleCache interface {
	Cache[string, any]
}

// TaskCache provides a cache scoped to a task's execution within a module.
type TaskCache interface {
	Cache[string, any]
}

// StepCache provides a cache scoped to a step's execution within a task.
// It also includes methods to get/set the current step's specification.
type StepCache interface {
	Cache[string, any]
	GetCurrentStepSpec() (spec.StepSpec, bool)
	SetCurrentStepSpec(s spec.StepSpec)
}

// === 3. mapCache Implementation (Generic) ===

// mapCache is a thread-safe, map-based implementation of the Cache interface.
type mapCache[K comparable, V any] struct {
	store map[K]V
	mu    sync.RWMutex
}

// NewMapCache creates a new instance of mapCache.
func NewMapCache[K comparable, V any]() *mapCache[K, V] {
	return &mapCache[K, V]{store: make(map[K]V)}
}

// Get retrieves a value from the cache by key.
// It returns the value and true if the key exists, otherwise the zero value for V and false.
func (mc *mapCache[K, V]) Get(key K) (V, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	val, exists := mc.store[key]
	return val, exists
}

// Set adds or updates a key-value pair in the cache.
func (mc *mapCache[K, V]) Set(key K, value V) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.store[key] = value
}

// Delete removes a key-value pair from the cache.
func (mc *mapCache[K, V]) Delete(key K) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	delete(mc.store, key)
}

// Clear removes all key-value pairs from the cache.
func (mc *mapCache[K, V]) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	// Create a new map to clear. This is often preferred over iterating and deleting
	// for simplicity and to ensure capacity is reset if that's desired.
	mc.store = make(map[K]V)
}

// === 4. mapStepCache Implementation (Specific for StepCache) ===

// mapStepCache implements the StepCache interface.
// It embeds a generic mapCache for key-value storage and adds step-specific fields.
type mapStepCache struct {
	*mapCache[string, any] // Embed generic map cache for string keys, any values
	currentStepSpec spec.StepSpec
	specMu          sync.RWMutex // Separate mutex for currentStepSpec field
}

// NewMapStepCache creates a new instance of mapStepCache.
func NewMapStepCache() *mapStepCache {
	return &mapStepCache{
		mapCache: NewMapCache[string, any](),
		// currentStepSpec is initially nil
	}
}

// GetCurrentStepSpec retrieves the currently executing step's specification.
func (sc *mapStepCache) GetCurrentStepSpec() (spec.StepSpec, bool) {
	sc.specMu.RLock()
	defer sc.specMu.RUnlock()
	if sc.currentStepSpec == nil {
		var zeroVal spec.StepSpec // Explicitly return typed nil
		return zeroVal, false
	}
	return sc.currentStepSpec, true
}

// SetCurrentStepSpec sets the specification for the currently executing step.
func (sc *mapStepCache) SetCurrentStepSpec(s spec.StepSpec) {
	sc.specMu.Lock()
	defer sc.specMu.Unlock()
	sc.currentStepSpec = s
}

// The Get, Set, Delete, and Clear methods for mapStepCache's string-keyed cache
// are automatically provided by the embedded *mapCache[string, any].

// === 5. Factory Functions ===

// NewPipelineCache creates a new PipelineCache.
func NewPipelineCache() PipelineCache {
	return NewMapCache[string, any]()
}

// NewModuleCache creates a new ModuleCache.
func NewModuleCache() ModuleCache {
	return NewMapCache[string, any]()
}

// NewTaskCache creates a new TaskCache.
func NewTaskCache() TaskCache {
	return NewMapCache[string, any]()
}

// NewStepCache creates a new StepCache.
func NewStepCache() StepCache {
	return NewMapStepCache()
}
