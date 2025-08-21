package cache

import "time"

type PipelineCache = Cache

func NewPipelineCache() PipelineCache {
	return New(24*time.Hour, 1*time.Hour, nil)
}
