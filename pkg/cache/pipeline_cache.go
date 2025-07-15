package cache

type PipelineCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
	Keys() []string
}

func NewPipelineCache() PipelineCache {
	return NewGenericCache(nil)
}
