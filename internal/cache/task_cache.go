package cache

import "time"

type TaskCache = Cache

func NewTaskCache(parent ModuleCache) TaskCache {
	return New(30*time.Minute, 5*time.Minute, parent)
}
