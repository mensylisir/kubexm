package cache

import "time"

type StepCache = Cache

func NewStepCache(parent TaskCache) StepCache {
	return New(5*time.Minute, 1*time.Minute, parent)
}
