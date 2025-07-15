package cache

type ModuleCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
	Keys() []string
}

func NewModuleCache(parent PipelineCache) ModuleCache {
	return NewGenericCache(parent)
}
