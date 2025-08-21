package cache

import "time"

type ModuleCache = Cache

func NewModuleCache(parent PipelineCache) ModuleCache {
	return New(1*time.Hour, 10*time.Minute, parent)
}
