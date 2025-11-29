package runtime

import (
	"sync"
)

// StateBag provides a thread-safe mechanism to store and retrieve state
// with support for different scopes (Global, Pipeline, Module, Task).
type StateBag interface {
	Set(key string, value interface{})
	Get(key string) (interface{}, bool)
	GetString(key string) (string, bool)
	GetInt(key string) (int, bool)
	GetBool(key string) (bool, bool)
	GetAll() map[string]interface{}
	Delete(key string)
}

type stateBag struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func NewStateBag() StateBag {
	return &stateBag{
		data: make(map[string]interface{}),
	}
}

func (s *stateBag) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

func (s *stateBag) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *stateBag) GetString(key string) (string, bool) {
	val, ok := s.Get(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

func (s *stateBag) GetInt(key string) (int, bool) {
	val, ok := s.Get(key)
	if !ok {
		return 0, false
	}
	// Handle various int types if needed, but for now strict int
	i, ok := val.(int)
	return i, ok
}

func (s *stateBag) GetBool(key string) (bool, bool) {
	val, ok := s.Get(key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

func (s *stateBag) GetAll() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copy := make(map[string]interface{}, len(s.data))
	for k, v := range s.data {
		copy[k] = v
	}
	return copy
}

func (s *stateBag) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}
