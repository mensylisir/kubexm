package cache

import (
	"sync"
)

type parentCache interface {
	Get(key string) (interface{}, bool)
}
type GenericCache struct {
	store  sync.Map
	parent parentCache
}

func NewGenericCache(parent parentCache) *GenericCache {
	return &GenericCache{
		parent: parent,
	}
}

func (c *GenericCache) Get(key string) (interface{}, bool) {
	if val, ok := c.store.Load(key); ok {
		return val, true
	}
	if c.parent != nil {
		return c.parent.Get(key)
	}
	return nil, false
}

func (c *GenericCache) Keys() []string {
	var keys []string
	c.store.Range(func(key interface{}, value interface{}) bool {
		if kStr, ok := key.(string); ok {
			keys = append(keys, kStr)
		}
		return true
	})
	return keys
}

func (c *GenericCache) Set(k string, v interface{}) {
	c.store.Store(k, v)
}

func (c *GenericCache) Delete(k string) {
	c.store.Delete(k)
}

func (c *GenericCache) GetMustString(k string) (string, bool) {
	v, ok := c.Get(k)
	if !ok {
		return "", false
	}
	res, assert := v.(string)
	return res, assert
}

func (c *GenericCache) SetParent(parent *GenericCache) {
	c.parent = parent
}

func (c *GenericCache) GetOrSet(k string, v interface{}) (interface{}, bool) {
	return c.store.LoadOrStore(k, v)
}

func (c *GenericCache) Range(f func(key, value interface{}) bool) {
	c.store.Range(f)
}

func (c *GenericCache) Clean() {
	c.store.Range(func(key, value interface{}) bool {
		c.store.Delete(key)
		return true
	})
}

func (c *GenericCache) GetMustInt(k string) (int, bool) {
	v, ok := c.Get(k)
	res, assert := v.(int)
	if !assert {
		return res, false
	}
	return res, ok
}

func (c *GenericCache) GetMustBool(k string) (bool, bool) {
	v, ok := c.Get(k)
	res, assert := v.(bool)
	if !assert {
		return res, false
	}
	return res, ok
}
