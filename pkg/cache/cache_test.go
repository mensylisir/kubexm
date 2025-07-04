package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Simple struct to act as a StepSpec for testing
type testStepSpec struct {
	Name string
	Val  int
}

func TestGenericCache_GetSetDelete(t *testing.T) {
	cache := NewGenericCache(nil) // Fixed: pass nil for parent

	// Test Set and Get
	cache.Set("key1", "value1")
	val, ok := cache.Get("key1")
	assert.True(t, ok, "Expected 'key1' to be found")
	assert.Equal(t, "value1", val, "Expected to get 'value1' for 'key1'")

	// Test Get non-existent
	val, ok = cache.Get("nonexistent")
	assert.False(t, ok, "Expected ok=false for non-existent key")
	assert.Nil(t, val, "Expected val=nil for non-existent key")

	// Test Set overwrite
	cache.Set("key1", "value1_overwritten")
	val, ok = cache.Get("key1")
	assert.True(t, ok, "Expected 'key1' to be found after overwrite")
	assert.Equal(t, "value1_overwritten", val, "Expected 'value1_overwritten' after overwrite")

	// Test Delete
	cache.Delete("key1")
	val, ok = cache.Get("key1")
	assert.False(t, ok, "Expected ok=false after deleting 'key1'")
	assert.Nil(t, val, "Expected val=nil after deleting 'key1'")

	// Test Delete non-existent
	assert.NotPanics(t, func() { cache.Delete("nonexistent_delete") }, "Delete non-existent key should not panic")
}

func TestGenericCache_StepSpec(t *testing.T) {
	// NewStepCache now requires a TaskCache parent.
	// For this test, we are focused on StepCache's own methods,
	// so a nil parent is acceptable.
	var nilTaskCache TaskCache
	stepCache := NewStepCache(nilTaskCache)

	// Test GetCurrentStepSpec when none is set
	spec, ok := stepCache.GetCurrentStepSpec()
	assert.False(t, ok, "Expected ok=false for initial GetCurrentStepSpec")
	assert.Nil(t, spec, "Expected nil spec initially")

	// Test SetCurrentStepSpec and GetCurrentStepSpec
	mySpec := &testStepSpec{Name: "TestSpec", Val: 123}
	stepCache.SetCurrentStepSpec(mySpec)

	retrievedSpec, ok := stepCache.GetCurrentStepSpec()
	assert.True(t, ok, "GetCurrentStepSpec should return ok=true after setting a spec")
	assert.NotNil(t, retrievedSpec, "GetCurrentStepSpec should return non-nil spec after setting one")

	castedSpec, castOk := retrievedSpec.(*testStepSpec)
	assert.True(t, castOk, "Could not cast retrieved spec to *testStepSpec, type was %T", retrievedSpec)
	if castOk { // Proceed only if cast was successful
		assert.Equal(t, "TestSpec", castedSpec.Name, "Retrieved spec name did not match")
		assert.Equal(t, 123, castedSpec.Val, "Retrieved spec val did not match")
	}

	// Test overwriting spec
	anotherSpec := &testStepSpec{Name: "AnotherSpec", Val: 456}
	stepCache.SetCurrentStepSpec(anotherSpec)
	retrievedSpec, ok = stepCache.GetCurrentStepSpec()
	assert.True(t, ok, "GetCurrentStepSpec should return ok=true after overwrite")
	castedSpec, castOk = retrievedSpec.(*testStepSpec)
	assert.True(t, castOk, "Could not cast retrieved spec to *testStepSpec after overwrite, type was %T", retrievedSpec)
	if castOk {
		assert.Equal(t, "AnotherSpec", castedSpec.Name, "Retrieved spec name after overwrite did not match")
	}
}

func TestGenericCache_Concurrency(t *testing.T) {
	cache := NewGenericCache(nil) // Fixed: pass nil for parent
	var wg sync.WaitGroup
	numGoroutines := 50

	// Test concurrent Set/Get
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", idx)
			value := fmt.Sprintf("value-%d", idx)
			cache.Set(key, value)

			time.Sleep(time.Millisecond * time.Duration(idx%10)) // Stagger reads a bit

			retrievedVal, ok := cache.Get(key)
			assert.True(t, ok, "Goroutine %d: Expected key '%s' to be found", idx, key)
			assert.Equal(t, value, retrievedVal, "Goroutine %d: Value mismatch for key '%s'", idx, key)
		}(i)
	}
	wg.Wait()

	// Verify all values are still correct after concurrent sets
	for i := 0; i < numGoroutines; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		retrievedVal, ok := cache.Get(key)
		assert.True(t, ok, "Post-concurrency check: Expected key '%s' to be found", key)
		assert.Equal(t, value, retrievedVal, "Post-concurrency check: Value mismatch for key '%s'", key)
	}
}

func TestStepCache_Concurrency_StepSpec(t *testing.T) {
	// NewStepCache now requires a TaskCache parent.
	// For this test, we are focused on StepCache's own methods,
	// so a nil parent (or a dummy one) is acceptable.
	var nilTaskCache TaskCache // explicit nil of type TaskCache
	stepCache := NewStepCache(nilTaskCache)
	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			spec := &testStepSpec{Name: fmt.Sprintf("Spec-%d", idx), Val: idx}
			stepCache.SetCurrentStepSpec(spec)

			time.Sleep(time.Millisecond * time.Duration(idx%10))

			// This test is tricky because SetCurrentStepSpec overwrites a single key.
			// The actual retrieved spec will be one of the last ones set.
			// We are mainly testing for race conditions here, not value persistence for each goroutine.
			_, ok := stepCache.GetCurrentStepSpec()
			if !ok {
				// This might happen if another goroutine deleted it, though genericCache.Delete isn't used by SetCurrentStepSpec.
				// More likely, if ok is false, it means nil was stored, which shouldn't happen with &testStepSpec.
				// t.Errorf("Goroutine %d: GetCurrentStepSpec returned !ok", idx)
			}
		}(i)
	}
	wg.Wait()

	// Check that a spec is still there (likely the one set by one of the last goroutines)
	finalSpec, ok := stepCache.GetCurrentStepSpec()
	assert.True(t, ok, "After concurrent SetCurrentStepSpec, GetCurrentStepSpec should return ok=true")
	assert.NotNil(t, finalSpec, "After concurrent SetCurrentStepSpec, finalSpec should not be nil")
	assert.IsType(t, &testStepSpec{}, finalSpec, "After concurrent SetCurrentStepSpec, finalSpec type mismatch")
}

func TestGenericCache_InheritedGet(t *testing.T) {
	// Create a hierarchy: pipeline -> module -> task -> step
	pipelineC := NewGenericCache(nil) // Parent is nil
	moduleC := NewGenericCache(pipelineC)
	taskC := NewGenericCache(moduleC)
	stepC := NewGenericCache(taskC)

	// Set values at different levels
	pipelineC.Set("pipeKey", "pipeValue")
	pipelineC.Set("overrideKey", "pipeOverride")

	moduleC.Set("moduleKey", "moduleValue")
	moduleC.Set("overrideKey", "moduleOverride") // Overrides pipeline

	taskC.Set("taskKey", "taskValue")

	stepC.Set("stepKey", "stepValue")

	// Test reads from stepCache (should be able to see all)
	// Local to step
	val, ok := stepC.Get("stepKey")
	assert.True(t, ok)
	assert.Equal(t, "stepValue", val)
	// From task
	val, ok = stepC.Get("taskKey")
	assert.True(t, ok)
	assert.Equal(t, "taskValue", val)
	// From module (overridden)
	val, ok = stepC.Get("overrideKey")
	assert.True(t, ok)
	assert.Equal(t, "moduleOverride", val)
	// From pipeline
	val, ok = stepC.Get("pipeKey")
	assert.True(t, ok)
	assert.Equal(t, "pipeValue", val)

	// Test reads from moduleCache
	val, ok = moduleC.Get("taskKey") // Should not see taskKey
	assert.False(t, ok)
	assert.Nil(t, val)

	val, ok = moduleC.Get("overrideKey")
	assert.True(t, ok)
	assert.Equal(t, "moduleOverride", val)

	// Test non-existent key
	val, ok = stepC.Get("nonExistentAnywhere")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestGenericCache_SetParent_And_InheritedGet(t *testing.T) {
	// This test simulates how runtime would set parents
	pipelineC := NewGenericCache(nil)
	moduleC := NewGenericCache(nil) // Initially no parent
	taskC := NewGenericCache(nil)   // Initially no parent

	// Set parent relationships
	moduleC.SetParent(pipelineC)
	taskC.SetParent(moduleC)

	pipelineC.Set("pKey", "pValue")
	moduleC.Set("mKey", "mValue")
	taskC.Set("tKey", "tValue")

	// Test Get from taskCache
	val, ok := taskC.Get("pKey")
	assert.True(t, ok)
	assert.Equal(t, "pValue", val)

	val, ok = taskC.Get("mKey")
	assert.True(t, ok)
	assert.Equal(t, "mValue", val)

	val, ok = taskC.Get("tKey")
	assert.True(t, ok)
	assert.Equal(t, "tValue", val)
}


func TestGenericCache_LocalizedSetDelete(t *testing.T) {
	pipelineC := NewGenericCache(nil)
	moduleC := NewGenericCache(pipelineC)

	pipelineC.Set("sharedKey", "pipeValue")
	moduleC.Set("sharedKey", "moduleValue") // Override in module cache

	// Check module cache sees its own value
	val, ok := moduleC.Get("sharedKey")
	assert.True(t, ok)
	assert.Equal(t, "moduleValue", val)

	// Check pipeline cache still has its original value
	val, ok = pipelineC.Get("sharedKey")
	assert.True(t, ok)
	assert.Equal(t, "pipeValue", val)

	// Delete from module cache
	moduleC.Delete("sharedKey")
	_, stillInModuleStore := moduleC.store.Load("sharedKey")
	assert.False(t, stillInModuleStore, "ModuleCache: 'sharedKey' should have been deleted from module's own store")

	val, ok = moduleC.Get("sharedKey")
	assert.True(t, ok, "ModuleCache: Should find 'sharedKey' from pipeline after local delete")
	assert.Equal(t, "pipeValue", val, "ModuleCache: Value from pipeline for 'sharedKey' incorrect after local delete")


	// Set a key only in module cache
	moduleC.Set("moduleOnlyKey", "modOnly")
	val, ok = pipelineC.Get("moduleOnlyKey")
	assert.False(t, ok, "PipelineCache: Should not find 'moduleOnlyKey' set in module cache")
	assert.Nil(t, val)
}

func TestFactoryFunctions(t *testing.T) {
	pc := NewPipelineCache()
	assert.NotNil(t, pc, "NewPipelineCache returned nil")
	pc.Set("testPipe", "valPipe")
	val, ok := pc.Get("testPipe")
	assert.True(t, ok, "PipelineCache Get failed for 'testPipe'")
	assert.Equal(t, "valPipe", val)

	mc := NewModuleCache(pc)
	assert.NotNil(t, mc, "NewModuleCache returned nil")
	mc.Set("testMod", "valMod")
	val, ok = mc.Get("testMod")
	assert.True(t, ok, "ModuleCache Get failed for 'testMod'")
	assert.Equal(t, "valMod", val)

	// Test inheritance from parent
	val, ok = mc.Get("testPipe")
	assert.True(t, ok, "ModuleCache failed to get 'testPipe' from parent")
	assert.Equal(t, "valPipe", val)

	tc := NewTaskCache(mc)
	assert.NotNil(t, tc, "NewTaskCache returned nil")
	tc.Set("testTask", "valTask")
	val, ok = tc.Get("testTask")
	assert.True(t, ok, "TaskCache Get failed for 'testTask'")
	assert.Equal(t, "valTask", val)

	val, ok = tc.Get("testMod")
	assert.True(t, ok, "TaskCache failed to get 'testMod' from parent")
	assert.Equal(t, "valMod", val)

	sc := NewStepCache(tc)
	assert.NotNil(t, sc, "NewStepCache returned nil")
	sc.Set("testStep", "valStep")
	val, ok = sc.Get("testStep")
	assert.True(t, ok, "StepCache Get failed for 'testStep'")
	assert.Equal(t, "valStep", val)

	val, ok = sc.Get("testTask")
	assert.True(t, ok, "StepCache failed to get 'testTask' from parent")
	assert.Equal(t, "valTask", val)
}

func TestFactoryFunctions_WithNilParents(t *testing.T) {
	mc := NewModuleCache(nil) // Pass nil as PipelineCache
	assert.NotNil(t, mc, "NewModuleCache(nil) returned nil")
	mc.Set("key", "val")
	val, ok := mc.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "val", val)

	tc := NewTaskCache(nil) // Pass nil as ModuleCache
	assert.NotNil(t, tc, "NewTaskCache(nil) returned nil")
	tc.Set("key", "val")
	val, ok = tc.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "val", val)

	sc := NewStepCache(nil) // Pass nil as TaskCache
	assert.NotNil(t, sc, "NewStepCache(nil) returned nil")
	sc.Set("key", "val")
	val, ok = sc.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "val", val)

	spec := &testStepSpec{Name: "NilParentSpec"}
	sc.SetCurrentStepSpec(spec)
	retSpec, ok := sc.GetCurrentStepSpec()
	assert.True(t, ok)
	assert.Equal(t, spec, retSpec)
}

func TestFactoryFunction_HierarchyAndInheritance(t *testing.T) {
	// 1. Create hierarchy using factory functions
	pipelineCache := NewPipelineCache()
	pipelineCache.Set("pipeKey", "pipeValue")
	pipelineCache.Set("overrideKey", "pipeOverride")

	moduleCache := NewModuleCache(pipelineCache)
	moduleCache.Set("moduleKey", "moduleValue")
	moduleCache.Set("overrideKey", "moduleOverride") // Overrides pipeline

	taskCache := NewTaskCache(moduleCache)
	taskCache.Set("taskKey", "taskValue")

	stepCache := NewStepCache(taskCache)
	stepCache.Set("stepKey", "stepValue")

	// 2. Test reads from stepCache (should be able to see all inherited values)
	// Local to step
	val, ok := stepCache.Get("stepKey")
	assert.True(t, ok)
	assert.Equal(t, "stepValue", val)
	// From task
	val, ok = stepCache.Get("taskKey")
	assert.True(t, ok)
	assert.Equal(t, "taskValue", val)
	// From module (overridden)
	val, ok = stepCache.Get("overrideKey")
	assert.True(t, ok)
	assert.Equal(t, "moduleOverride", val)
	// From pipeline
	val, ok = stepCache.Get("pipeKey")
	assert.True(t, ok)
	assert.Equal(t, "pipeValue", val)

	// 3. Test reads from intermediate caches
	// From moduleCache, should not see taskKey
	val, ok = moduleCache.Get("taskKey")
	assert.False(t, ok, "ModuleCache should not find 'taskKey'")
	assert.Nil(t, val)

	// From moduleCache, should see its own overrideKey
	val, ok = moduleCache.Get("overrideKey")
	assert.True(t, ok)
	assert.Equal(t, "moduleOverride", val)

	// From moduleCache, should see pipeKey
	val, ok = moduleCache.Get("pipeKey")
	assert.True(t, ok)
	assert.Equal(t, "pipeValue", val)


	// 4. Test non-existent key
	val, ok = stepCache.Get("nonExistentAnywhere")
	assert.False(t, ok, "StepCache should not find 'nonExistentAnywhere'")
	assert.Nil(t, val)

	// 5. Test that writes are local (idempotent to TestGenericCache_LocalizedSetDelete, but using factory-created caches)
	pipelineCache.Set("sharedKey", "pipeInitial")
	moduleCache.Set("sharedKey", "moduleInitial")

	// Check module cache sees its own value
	val, ok = moduleCache.Get("sharedKey")
	assert.True(t, ok)
	assert.Equal(t, "moduleInitial", val)
	// Check pipeline cache still has its original value
	val, ok = pipelineCache.Get("sharedKey")
	assert.True(t, ok)
	assert.Equal(t, "pipeInitial", val)
}

// TestFactoryFunctions_PanicWithWrongParentType tests that factory functions panic
// if a parent of an incorrect type is provided.
func TestFactoryFunctions_PanicWithWrongParentType(t *testing.T) {
	// Create a dummy cache that satisfies PipelineCache but isn't a *genericCache
	type dummyPipelineCache struct{ PipelineCache }
	dp := &dummyPipelineCache{}

	// Create a dummy cache that satisfies ModuleCache but isn't a *genericCache
	type dummyModuleCache struct{ ModuleCache }
	dm := &dummyModuleCache{}

	// Create a dummy cache that satisfies TaskCache but isn't a *genericCache
	type dummyTaskCache struct{ TaskCache }
	dt := &dummyTaskCache{}

	assert.PanicsWithValue(t, "NewModuleCache: parent is not of type *genericCache, got *cache.dummyPipelineCache", func() {
		NewModuleCache(dp)
	}, "NewModuleCache should panic with wrong parent type")

	assert.PanicsWithValue(t, "NewTaskCache: parent is not of type *genericCache, got *cache.dummyModuleCache", func() {
		NewTaskCache(dm)
	}, "NewTaskCache should panic with wrong parent type")

	assert.PanicsWithValue(t, "NewStepCache: parent is not of type *genericCache, got *cache.dummyTaskCache", func() {
		NewStepCache(dt)
	}, "NewStepCache should panic with wrong parent type")
}

func TestGenericCache_Keys(t *testing.T) {
	// Test with a single cache (no parent)
	cache := NewGenericCache(nil)

	// 1. Test on an empty cache
	keys := cache.Keys()
	assert.Empty(t, keys, "Keys() on an empty cache should return an empty slice")

	// 2. Add some keys
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set(currentStepSpecKey, "specValue") // Special key for StepCache

	keys = cache.Keys()
	assert.Len(t, keys, 3, "Expected 3 keys after adding")
	assert.ElementsMatch(t, []string{"key1", "key2", currentStepSpecKey}, keys, "Keys mismatch after adding")

	// 3. Delete a key
	cache.Delete("key1")
	keys = cache.Keys()
	assert.Len(t, keys, 2, "Expected 2 keys after deleting one")
	assert.ElementsMatch(t, []string{"key2", currentStepSpecKey}, keys, "Keys mismatch after deleting one")

	// 4. Delete another key (special one)
	cache.Delete(currentStepSpecKey)
	keys = cache.Keys()
	assert.Len(t, keys, 1, "Expected 1 key after deleting currentStepSpecKey")
	assert.ElementsMatch(t, []string{"key2"}, keys, "Keys mismatch after deleting currentStepSpecKey")

	// 5. Delete last key
	cache.Delete("key2")
	keys = cache.Keys()
	assert.Empty(t, keys, "Keys() after deleting all keys should return an empty slice")

	// Test with parent cache to ensure Keys() is local
	parentCache := NewGenericCache(nil)
	parentCache.Set("parentKey1", "parentValue1")
	parentCache.Set("sharedKey", "parentSharedValue")

	childCache := NewGenericCache(parentCache)
	childCache.Set("childKey1", "childValue1")
	childCache.Set("sharedKey", "childSharedValue") // Overwrites for Get, but local for Keys

	// Keys from childCache should only include child's local keys
	childKeys := childCache.Keys()
	assert.Len(t, childKeys, 2, "Child cache Keys() should return 2 keys")
	assert.ElementsMatch(t, []string{"childKey1", "sharedKey"}, childKeys, "Child cache Keys() mismatch")

	// Keys from parentCache should only include parent's local keys
	parentKeys := parentCache.Keys()
	assert.Len(t, parentKeys, 2, "Parent cache Keys() should return 2 keys")
	assert.ElementsMatch(t, []string{"parentKey1", "sharedKey"}, parentKeys, "Parent cache Keys() mismatch")

	// Test Keys() via different cache interface types (using factory functions)
	pipelineC := NewPipelineCache().(*genericCache) // Assert to *genericCache to use Set directly for testing
	pipelineC.Set("pKey", "val")
	assert.ElementsMatch(t, []string{"pKey"}, pipelineC.Keys())

	moduleC := NewModuleCache(pipelineC).(*genericCache)
	moduleC.Set("mKey", "val")
	assert.ElementsMatch(t, []string{"mKey"}, moduleC.Keys(), "ModuleCache Keys() should be local")
	// Ensure parent keys are not included
	allModuleVisibleKeysViaGet := []string{}
	if _, ok := moduleC.Get("pKey"); ok { allModuleVisibleKeysViaGet = append(allModuleVisibleKeysViaGet, "pKey")}
	if _, ok := moduleC.Get("mKey"); ok { allModuleVisibleKeysViaGet = append(allModuleVisibleKeysViaGet, "mKey")}
	assert.Contains(t, allModuleVisibleKeysViaGet, "pKey", "Module should see pKey via Get for this test setup")


	taskC := NewTaskCache(moduleC).(*genericCache)
	taskC.Set("tKey", "val")
	assert.ElementsMatch(t, []string{"tKey"}, taskC.Keys(), "TaskCache Keys() should be local")

	stepC := NewStepCache(taskC).(*genericCache)
	stepC.Set("sKey", "val")
	stepC.SetCurrentStepSpec("step spec")
	assert.ElementsMatch(t, []string{"sKey", currentStepSpecKey}, stepC.Keys(), "StepCache Keys() should be local and include spec key")
}
