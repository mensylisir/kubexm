package runner

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// CacheEntry represents a cached result
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
	Hash      string
}

// CacheKey represents a unique key for caching
type CacheKey struct {
	Host      string
	Operation string
	Args      []string
}

// String returns a string representation of the cache key
func (k CacheKey) String() string {
	h := md5.New()
	h.Write([]byte(k.Host))
	h.Write([]byte(k.Operation))
	for _, arg := range k.Args {
		h.Write([]byte(arg))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ContextCache provides context-aware caching for runner operations
type ContextCache struct {
	mu           sync.RWMutex
	cache        map[string]*CacheEntry
	defaultTTL   time.Duration
	operationTTL map[string]time.Duration
	maxSize      int
	cleanupDone  chan struct{}
	cleanupOnce  sync.Once
}

// NewContextCache creates a new context-aware cache
func NewContextCache(defaultTTL time.Duration, maxSize int) *ContextCache {
	cache := &ContextCache{
		cache:        make(map[string]*CacheEntry),
		defaultTTL:   defaultTTL,
		operationTTL: make(map[string]time.Duration),
		maxSize:      maxSize,
		cleanupDone:  make(chan struct{}),
	}
	
	// Set operation-specific TTLs
	cache.operationTTL["facts"] = 30 * time.Minute        // System facts change rarely
	cache.operationTTL["package_check"] = 5 * time.Minute // Package status changes moderately
	cache.operationTTL["service_check"] = 1 * time.Minute // Service status changes frequently
	cache.operationTTL["file_exists"] = 10 * time.Minute  // File existence changes moderately
	cache.operationTTL["command_check"] = 2 * time.Minute // Command availability changes rarely
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// Get retrieves a value from the cache
func (c *ContextCache) Get(key CacheKey) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	keyStr := key.String()
	entry, exists := c.cache[keyStr]
	if !exists {
		return nil, false
	}
	
	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	
	return entry.Value, true
}

// Set stores a value in the cache
func (c *ContextCache) Set(key CacheKey, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check cache size limit
	if len(c.cache) >= c.maxSize {
		c.evictOldest()
	}
	
	// Determine TTL
	ttl := c.defaultTTL
	if operationTTL, exists := c.operationTTL[key.Operation]; exists {
		ttl = operationTTL
	}
	
	keyStr := key.String()
	c.cache[keyStr] = &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
		Hash:      keyStr,
	}
}

// Invalidate removes entries matching the pattern
func (c *ContextCache) Invalidate(pattern CacheKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// For simplicity, if host or operation matches, invalidate
	// A more sophisticated implementation could use regex patterns
	for key, entry := range c.cache {
		if entry.Hash == pattern.String() {
			delete(c.cache, key)
		}
	}
}

// InvalidateHost removes all entries for a specific host
func (c *ContextCache) InvalidateHost(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	toDelete := make([]string, 0)
	for key := range c.cache {
		// Simple check - in production, you'd want a more robust way
		// to associate cache keys with hosts
		if key[:8] == host[:min(8, len(host))] {
			toDelete = append(toDelete, key)
		}
	}
	
	for _, key := range toDelete {
		delete(c.cache, key)
	}
}

// Clear removes all entries from the cache
func (c *ContextCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.cache = make(map[string]*CacheEntry)
}

// Close shuts down the cache
func (c *ContextCache) Close() {
	c.cleanupOnce.Do(func() {
		close(c.cleanupDone)
	})
}

// cleanup removes expired entries
func (c *ContextCache) cleanup() {
	ticker := time.NewTicker(c.defaultTTL / 4)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.cleanupDone:
			return
		}
	}
}

// cleanupExpired removes expired entries
func (c *ContextCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for key, entry := range c.cache {
		if now.After(entry.ExpiresAt) {
			delete(c.cache, key)
		}
	}
}

// evictOldest removes the oldest entry (simple LRU)
func (c *ContextCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, entry := range c.cache {
		if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.ExpiresAt
		}
	}
	
	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}

// Stats returns cache statistics
func (c *ContextCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	expired := 0
	now := time.Now()
	
	for _, entry := range c.cache {
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}
	
	return map[string]interface{}{
		"total":      len(c.cache),
		"expired":    expired,
		"active":     len(c.cache) - expired,
		"max_size":   c.maxSize,
		"default_ttl": c.defaultTTL.String(),
	}
}

// CachedRunner wraps a runner with caching capabilities
type CachedRunner struct {
	Runner
	cache *ContextCache
}

// NewCachedRunner creates a new cached runner
func NewCachedRunner(runner Runner, defaultTTL time.Duration, maxSize int) *CachedRunner {
	return &CachedRunner{
		Runner: runner,
		cache:  NewContextCache(defaultTTL, maxSize),
	}
}

// GatherFacts with caching
func (r *CachedRunner) GatherFacts(ctx context.Context, conn connector.Connector) (*Facts, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil")
	}
	
	// For caching, we need a host identifier. Since we can't use GetHost(),
	// we'll use a fallback approach - get hostname from basic command
	var hostName string
	stdout, _, err := conn.Exec(ctx, "hostname", &connector.ExecOptions{Timeout: 5 * time.Second})
	if err == nil && len(stdout) > 0 {
		hostName = string(stdout)
	} else {
		// Fallback to a generic identifier for caching
		hostName = "unknown-host"
	}
	key := CacheKey{
		Host:      hostName,
		Operation: "facts",
		Args:      []string{},
	}
	
	// Check cache first
	if cached, found := r.cache.Get(key); found {
		if facts, ok := cached.(*Facts); ok {
			return facts, nil
		}
	}
	
	// Cache miss - gather facts
	facts, err := r.Runner.GatherFacts(ctx, conn)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	r.cache.Set(key, facts)
	
	return facts, nil
}

// Check with caching
func (r *CachedRunner) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	
	// For caching, get hostname using basic command
	var hostName string
	stdout, _, err := conn.Exec(ctx, "hostname", &connector.ExecOptions{Timeout: 5 * time.Second})
	if err == nil && len(stdout) > 0 {
		hostName = string(stdout)
	} else {
		hostName = "unknown-host"
	}
	key := CacheKey{
		Host:      hostName,
		Operation: "command_check",
		Args:      []string{cmd, fmt.Sprintf("sudo:%v", sudo)},
	}
	
	// Check cache first
	if cached, found := r.cache.Get(key); found {
		if result, ok := cached.(bool); ok {
			return result, nil
		}
	}
	
	// Cache miss - run check
	result, err := r.Runner.Check(ctx, conn, cmd, sudo)
	if err != nil {
		return false, err
	}
	
	// Cache the result
	r.cache.Set(key, result)
	
	return result, nil
}

// Exists with caching
func (r *CachedRunner) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	
	// For caching, get hostname using basic command
	var hostName string
	stdout, _, err := conn.Exec(ctx, "hostname", &connector.ExecOptions{Timeout: 5 * time.Second})
	if err == nil && len(stdout) > 0 {
		hostName = string(stdout)
	} else {
		hostName = "unknown-host"
	}
	key := CacheKey{
		Host:      hostName,
		Operation: "file_exists",
		Args:      []string{path},
	}
	
	// Check cache first
	if cached, found := r.cache.Get(key); found {
		if result, ok := cached.(bool); ok {
			return result, nil
		}
	}
	
	// Cache miss - check existence
	result, err := r.Runner.Exists(ctx, conn, path)
	if err != nil {
		return false, err
	}
	
	// Cache the result
	r.cache.Set(key, result)
	
	return result, nil
}

// IsPackageInstalled with caching
func (r *CachedRunner) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *Facts, packageName string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	
	hostName := facts.Hostname
	if hostName == "" {
		hostName = "unknown-host"
	}
	key := CacheKey{
		Host:      hostName,
		Operation: "package_check",
		Args:      []string{packageName},
	}
	
	// Check cache first
	if cached, found := r.cache.Get(key); found {
		if result, ok := cached.(bool); ok {
			return result, nil
		}
	}
	
	// Cache miss - check package installation
	result, err := r.Runner.IsPackageInstalled(ctx, conn, facts, packageName)
	if err != nil {
		return false, err
	}
	
	// Cache the result
	r.cache.Set(key, result)
	
	return result, nil
}

// IsServiceActive with caching
func (r *CachedRunner) IsServiceActive(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	
	hostName := facts.Hostname
	if hostName == "" {
		hostName = "unknown-host"
	}
	key := CacheKey{
		Host:      hostName,
		Operation: "service_check",
		Args:      []string{serviceName},
	}
	
	// Check cache first
	if cached, found := r.cache.Get(key); found {
		if result, ok := cached.(bool); ok {
			return result, nil
		}
	}
	
	// Cache miss - check service status
	result, err := r.Runner.IsServiceActive(ctx, conn, facts, serviceName)
	if err != nil {
		return false, err
	}
	
	// Cache the result
	r.cache.Set(key, result)
	
	return result, nil
}

// InvalidateCache invalidates cache entries for a host
func (r *CachedRunner) InvalidateCache(host string) {
	r.cache.InvalidateHost(host)
}

// GetCacheStats returns cache statistics
func (r *CachedRunner) GetCacheStats() map[string]interface{} {
	return r.cache.Stats()
}

// Close closes the cache
func (r *CachedRunner) Close() {
	r.cache.Close()
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}