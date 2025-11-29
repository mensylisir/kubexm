package runtime

import (
	"sync"
	"testing"
)

func TestStateBag(t *testing.T) {
	sb := NewStateBag()

	t.Run("SetAndGet", func(t *testing.T) {
		sb.Set("key1", "value1")
		val, ok := sb.Get("key1")
		if !ok {
			t.Error("Expected key1 to exist")
		}
		if val != "value1" {
			t.Errorf("Expected value1, got %v", val)
		}
	})

	t.Run("TypedGetters", func(t *testing.T) {
		sb.Set("strKey", "hello")
		sb.Set("intKey", 123)
		sb.Set("boolKey", true)

		sVal, ok := sb.GetString("strKey")
		if !ok || sVal != "hello" {
			t.Errorf("GetString failed: %v, %v", sVal, ok)
		}

		iVal, ok := sb.GetInt("intKey")
		if !ok || iVal != 123 {
			t.Errorf("GetInt failed: %v, %v", iVal, ok)
		}

		bVal, ok := sb.GetBool("boolKey")
		if !ok || !bVal {
			t.Errorf("GetBool failed: %v, %v", bVal, ok)
		}

		// Type mismatch
		_, ok = sb.GetInt("strKey")
		if ok {
			t.Error("GetInt should fail for string value")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		sb.Set("delKey", "val")
		sb.Delete("delKey")
		_, ok := sb.Get("delKey")
		if ok {
			t.Error("Expected delKey to be deleted")
		}
	})

	t.Run("Concurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				sb.Set("concurrentKey", val)
				sb.Get("concurrentKey")
			}(i)
		}
		wg.Wait()
	})
}
