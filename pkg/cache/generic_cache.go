package cache

import (
	"sync"
	"time"
)

type GenericCache struct {
	defaultTTL time.Duration
	store      sync.Map
	parent     Cache
	janitor    *janitor
}

func New(defaultTTL, cleanupInterval time.Duration, parent Cache) Cache {
	c := &GenericCache{
		defaultTTL: defaultTTL,
		parent:     parent,
	}

	if cleanupInterval > 0 {
		c.janitor = runJanitor(c, cleanupInterval)
	}

	return c
}

func (c *GenericCache) Get(key string) (interface{}, bool) {
	val, ok := c.store.Load(key)
	if ok {
		item := val.(item)
		if !item.Expired() {
			return item.Value, true
		}
		c.store.Delete(key)
	}

	if c.parent != nil {
		return c.parent.Get(key)
	}

	return nil, false
}

func (c *GenericCache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, DefaultExpiration)
}

func (c *GenericCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	var expires int64
	if ttl == DefaultExpiration {
		ttl = c.defaultTTL
	}
	if ttl > 0 {
		expires = time.Now().Add(ttl).UnixNano()
	}
	c.store.Store(key, item{
		Value:      value,
		Expiration: expires,
	})
}

func (c *GenericCache) Delete(k string) {
	c.store.Delete(k)
}

func (c *GenericCache) Has(key string) bool {
	_, ok := c.Get(key)
	return ok
}

func (c *GenericCache) Keys() []string {
	var keys []string
	c.store.Range(func(key, value interface{}) bool {
		item := value.(item)
		if !item.Expired() {
			if kStr, ok := key.(string); ok {
				keys = append(keys, kStr)
			}
		}
		return true
	})
	return keys
}

func (c *GenericCache) Count() int {
	count := 0
	c.store.Range(func(key, value interface{}) bool {
		item := value.(item)
		if !item.Expired() {
			count++
		}
		return true
	})
	return count
}

func (c *GenericCache) Flush() {
	c.store = sync.Map{}
}

func (c *GenericCache) GetOrSet(k string, v interface{}) (interface{}, bool) {
	existing, ok := c.store.Load(k)
	if ok {
		item := existing.(item)
		if !item.Expired() {
			return item.Value, true
		}
	}

	var expires int64
	if c.defaultTTL > 0 {
		expires = time.Now().Add(c.defaultTTL).UnixNano()
	}
	newItem := item{Value: v, Expiration: expires}

	actualItem, loaded := c.store.LoadOrStore(k, newItem)
	if loaded {
		return actualItem.(item).Value, true
	}

	return newItem.Value, false
}

func (c *GenericCache) GetString(k string) (string, bool) {
	v, ok := c.Get(k)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (c *GenericCache) GetInt(k string) (int, bool) {
	v, ok := c.Get(k)
	if !ok {
		return 0, false
	}
	i, ok := v.(int)
	return i, ok
}

func (c *GenericCache) GetInt64(k string) (int64, bool) {
	v, ok := c.Get(k)
	if !ok {
		return 0, false
	}
	i, ok := v.(int64)
	return i, ok
}

func (c *GenericCache) GetBool(k string) (bool, bool) {
	v, ok := c.Get(k)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func (c *GenericCache) GetFloat64(k string) (float64, bool) {
	v, ok := c.Get(k)
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

func (c *GenericCache) GetTime(k string) (time.Time, bool) {
	v, ok := c.Get(k)
	if !ok {
		return time.Time{}, false
	}
	t, ok := v.(time.Time)
	return t, ok
}

func (c *GenericCache) GetStringOrDefault(k string, defaultValue string) string {
	if val, ok := c.GetString(k); ok {
		return val
	}
	return defaultValue
}

func (c *GenericCache) GetIntOrDefault(k string, defaultValue int) int {
	if val, ok := c.GetInt(k); ok {
		return val
	}
	return defaultValue
}

func (c *GenericCache) GetBoolOrDefault(k string, defaultValue bool) bool {
	if val, ok := c.GetBool(k); ok {
		return val
	}
	return defaultValue
}

func (c *GenericCache) Range(f func(key string, value interface{}) bool) {
	c.store.Range(func(key, value interface{}) bool {
		kStr, ok := key.(string)
		if !ok {
			return true
		}

		item, ok := value.(item)
		if !ok || item.Expired() {
			return true
		}

		return f(kStr, item.Value)
	})
}

func (c *GenericCache) numberOperation(k string, n int) (int, error) {
	for {
		v, ok := c.store.Load(k)
		if !ok {
			actual, loaded := c.store.LoadOrStore(k, item{Value: n, Expiration: 0})
			if !loaded {
				return n, nil
			}
			v = actual
		}

		item := v.(item)
		currentVal, ok := item.Value.(int)
		if !ok {
			return 0, &typeAssertionError{key: k, expected: "int"}
		}

		newValue := currentVal + n
		newItem := item
		newItem.Value = newValue

		if c.store.CompareAndSwap(k, v, newItem) {
			return newValue, nil
		}
	}
}

func (c *GenericCache) IncrementInt(k string, n int) (int, error) {
	return c.numberOperation(k, n)
}

func (c *GenericCache) DecrementInt(k string, n int) (int, error) {
	return c.numberOperation(k, -n)
}

func (c *GenericCache) deleteExpired() {
	c.store.Range(func(key, value interface{}) bool {
		item := value.(item)
		if item.Expired() {
			c.store.Delete(key)
		}
		return true
	})
}
