package cache

type StepCache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
	Delete(key string)
	Keys() []string
}

func NewStepCache(parent TaskCache) StepCache {
	return NewGenericCache(parent)
}
