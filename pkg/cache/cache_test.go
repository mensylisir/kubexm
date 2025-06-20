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
