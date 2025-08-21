package cache

import "time"

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *janitor) Run(c *GenericCache) {
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor(c *GenericCache) {
	if c.janitor != nil {
		c.janitor.stop <- true
	}
}

func runJanitor(c *GenericCache, ci time.Duration) *janitor {
	j := &janitor{
		Interval: ci,
		stop:     make(chan bool),
	}
	go j.Run(c)
	return j
}
