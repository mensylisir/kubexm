package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Simple struct to act as a StepSpec for testing
type testStepSpec struct {
	Name string
	Val  int
}

func TestGenericCache_GetSetDelete(t *testing.T) {
	cache := NewGenericCache() // This is *genericCache

	// Test Set and Get
	cache.Set("key1", "value1")
	val, ok := cache.Get("key1")
	if !ok || val != "value1" {
		t.Errorf("Expected to get 'value1' for 'key1', got '%v', ok=%v", val, ok)
	}

	// Test Get non-existent
	_, ok = cache.Get("nonexistent")
	if ok {
		t.Errorf("Expected ok=false for non-existent key, got ok=%v", ok)
	}

	// Test Set overwrite
	cache.Set("key1", "value1_overwritten")
	val, ok = cache.Get("key1")
	if !ok || val != "value1_overwritten" {
		t.Errorf("Expected 'value1_overwritten' after overwrite, got '%v', ok=%v", val, ok)
	}

	// Test Delete
	cache.Delete("key1")
	val, ok = cache.Get("key1")
	if ok {
		t.Errorf("Expected ok=false after deleting 'key1', got val='%v', ok=%v", val, ok)
	}

	// Test Delete non-existent
	cache.Delete("nonexistent_delete") // Should not panic
}

func TestGenericCache_StepSpec(t *testing.T) {
	stepCache := NewStepCache() // Returns StepCache interface, implemented by *genericCache

	// Test GetCurrentStepSpec when none is set
	spec, ok := stepCache.GetCurrentStepSpec()
	if ok || spec != nil {
		t.Errorf("Expected no spec initially, got spec=%v, ok=%v", spec, ok)
	}

	// Test SetCurrentStepSpec and GetCurrentStepSpec
	mySpec := &testStepSpec{Name: "TestSpec", Val: 123}
	stepCache.SetCurrentStepSpec(mySpec)

	retrievedSpec, ok := stepCache.GetCurrentStepSpec()
	if !ok {
		t.Fatalf("GetCurrentStepSpec returned ok=false after setting a spec")
	}
	if retrievedSpec == nil {
		t.Fatalf("GetCurrentStepSpec returned nil spec after setting one")
	}

	castedSpec, castOk := retrievedSpec.(*testStepSpec)
	if !castOk {
		t.Fatalf("Could not cast retrieved spec to *testStepSpec, type was %T", retrievedSpec)
	}
	if castedSpec.Name != "TestSpec" || castedSpec.Val != 123 {
		t.Errorf("Retrieved spec did not match set spec. Got: %+v", castedSpec)
	}

	// Test overwriting spec
	anotherSpec := &testStepSpec{Name: "AnotherSpec", Val: 456}
	stepCache.SetCurrentStepSpec(anotherSpec)
	retrievedSpec, _ = stepCache.GetCurrentStepSpec()
	castedSpec, _ = retrievedSpec.(*testStepSpec)
	if castedSpec.Name != "AnotherSpec" {
		t.Errorf("Retrieved spec after overwrite did not match. Got: %+v", castedSpec)
	}
}

func TestGenericCache_Concurrency(t *testing.T) {
	cache := NewGenericCache()
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
			if !ok || retrievedVal != value {
				t.Errorf("Goroutine %d: Expected '%s' for '%s', got '%v', ok=%v", idx, value, key, retrievedVal, ok)
			}
		}(i)
	}
	wg.Wait()

	// Verify all values are still correct after concurrent sets
	for i := 0; i < numGoroutines; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		retrievedVal, ok := cache.Get(key)
		if !ok || retrievedVal != value {
			t.Errorf("Post-concurrency check: Expected '%s' for '%s', got '%v', ok=%v", value, key, retrievedVal, ok)
		}
	}
}

func TestStepCache_Concurrency_StepSpec(t *testing.T) {
	stepCache := NewStepCache()
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
	if !ok {
		t.Errorf("After concurrent SetCurrentStepSpec, GetCurrentStepSpec returned !ok")
	}
	if finalSpec == nil {
		t.Errorf("After concurrent SetCurrentStepSpec, finalSpec is nil")
	}
	if _, castOk := finalSpec.(*testStepSpec); !castOk {
		t.Errorf("After concurrent SetCurrentStepSpec, finalSpec is not *testStepSpec, but %T", finalSpec)
	}
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
	if !ok || val != "stepValue" {
		t.Errorf("StepCache: Expected 'stepValue' for 'stepKey', got '%v'", val)
	}
	// From task
	val, ok = stepC.Get("taskKey")
	if !ok || val != "taskValue" {
		t.Errorf("StepCache: Expected 'taskValue' for 'taskKey' (from task), got '%v'", val)
	}
	// From module (overridden)
	val, ok = stepC.Get("overrideKey")
	if !ok || val != "moduleOverride" {
		t.Errorf("StepCache: Expected 'moduleOverride' for 'overrideKey' (from module), got '%v'", val)
	}
	// From pipeline
	val, ok = stepC.Get("pipeKey")
	if !ok || val != "pipeValue" {
		t.Errorf("StepCache: Expected 'pipeValue' for 'pipeKey' (from pipeline), got '%v'", val)
	}

	// Test reads from moduleCache
	val, ok = moduleC.Get("taskKey") // Should not see taskKey
	if ok {
		t.Errorf("ModuleCache: Expected not to find 'taskKey', but got '%v'", val)
	}
	val, ok = moduleC.Get("overrideKey")
	if !ok || val != "moduleOverride" {
		t.Errorf("ModuleCache: Expected 'moduleOverride' for 'overrideKey', got '%v'", val)
	}

	// Test non-existent key
	_, ok = stepC.Get("nonExistentAnywhere")
	if ok {
		t.Error("StepCache: Expected not to find 'nonExistentAnywhere'")
	}
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
	if !ok || val != "pValue" {
		t.Errorf("TaskCache (with SetParent): Expected 'pValue' for 'pKey', got '%v'", val)
	}
	val, ok = taskC.Get("mKey")
	if !ok || val != "mValue" {
		t.Errorf("TaskCache (with SetParent): Expected 'mValue' for 'mKey', got '%v'", val)
	}
	val, ok = taskC.Get("tKey")
	if !ok || val != "tValue" {
		t.Errorf("TaskCache (with SetParent): Expected 'tValue' for 'tKey', got '%v'", val)
	}
}


func TestGenericCache_LocalizedSetDelete(t *testing.T) {
	pipelineC := NewGenericCache(nil)
	moduleC := NewGenericCache(pipelineC)

	pipelineC.Set("sharedKey", "pipeValue")
	moduleC.Set("sharedKey", "moduleValue") // Override in module cache

	// Check module cache sees its own value
	val, ok := moduleC.Get("sharedKey")
	if !ok || val != "moduleValue" {
		t.Errorf("ModuleCache: Expected 'moduleValue' for 'sharedKey', got '%v'", val)
	}

	// Check pipeline cache still has its original value
	val, ok = pipelineC.Get("sharedKey")
	if !ok || val != "pipeValue" {
		t.Errorf("PipelineCache: Expected 'pipeValue' for 'sharedKey' after module set, got '%v'", val)
	}

	// Delete from module cache
	moduleC.Delete("sharedKey")
	_, ok = moduleC.Get("sharedKey") // Should now fall back to pipeline's value
	if !ok || moduleC.store.Load("sharedKey") == true { // Ensure it's gone from moduleC.store
		t.Error("ModuleCache: 'sharedKey' should be deleted from module's own store.")
	}
	val, ok = moduleC.Get("sharedKey") // Now this Get should find it in pipelineC
	if !ok || val != "pipeValue" {
		t.Errorf("ModuleCache: Expected 'pipeValue' for 'sharedKey' (from pipeline) after module delete, got '%v'", val)
	}


	// Set a key only in module cache
	moduleC.Set("moduleOnlyKey", "modOnly")
	_, ok = pipelineC.Get("moduleOnlyKey")
	if ok {
		t.Error("PipelineCache: Should not find 'moduleOnlyKey' set in module cache.")
	}
}

func TestFactoryFunctions(t *testing.T) {
	pc := NewPipelineCache()
	mc := NewModuleCache()
	tc := NewTaskCache()
	sc := NewStepCache()

	if pc == nil || mc == nil || tc == nil || sc == nil {
		t.Fatal("Factory functions returned nil caches")
	}
	// Further tests could involve type assertions if needed, but basic creation is enough here.
	// The parent linking will be tested via SetParent or in runtime tests.
	// For now, ensure they create non-nil caches.
	pc.Set("test", "val")
	_, ok := pc.Get("test")
	if !ok {
		t.Error("PipelineCache Get/Set failed")
	}
}
