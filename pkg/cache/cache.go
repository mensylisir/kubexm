package cache

import "time"

const (
	NoExpiration      time.Duration = -1
	DefaultExpiration time.Duration = 0
)

type item struct {
	Value      interface{}
	Expiration int64
}

func (i item) Expired() bool {
	if i.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > i.Expiration
}

type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	SetWithTTL(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Has(key string) bool
	Keys() []string
	Count() int
	Flush()
	GetOrSet(k string, v interface{}) (interface{}, bool)
	GetString(k string) (string, bool)
	GetInt(k string) (int, bool)
	GetInt64(k string) (int64, bool)
	GetBool(k string) (bool, bool)
	GetFloat64(k string) (float64, bool)
	GetTime(k string) (time.Time, bool)
	GetStringOrDefault(k string, defaultValue string) string
	GetIntOrDefault(k string, defaultValue int) int
	GetBoolOrDefault(k string, defaultValue bool) bool
	IncrementInt(k string, n int) (int, error)
	DecrementInt(k string, n int) (int, error)
	Range(f func(key string, value interface{}) bool)
}
