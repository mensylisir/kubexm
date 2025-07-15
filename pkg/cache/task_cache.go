package cache

type TaskCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
	Keys() []string
}

func NewTaskCache(parent ModuleCache) TaskCache {
	return NewGenericCache(parent)
}
