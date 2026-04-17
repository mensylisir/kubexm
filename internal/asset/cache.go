package asset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AssetCache tracks which assets are locally available and their metadata.
type AssetCache struct {
	mu      sync.RWMutex
	baseDir string
	// assets maps "category/name/version/arch" to cache entry
	assets map[string]*CacheEntry
}

// CacheEntry represents a single cached asset entry.
type CacheEntry struct {
	Path        string    `json:"path"`
	SizeBytes   int64     `json:"sizeBytes"`
	SHA256      string    `json:"sha256,omitempty"`
	CachedAt    time.Time `json:"cachedAt"`
	ClusterName string    `json:"clusterName,omitempty"`
}

// NewAssetCache creates a new asset cache with the given base directory.
// The cache directory stores a manifest.json and per-asset metadata files.
func NewAssetCache(baseDir string) (*AssetCache, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("asset cache base directory cannot be empty")
	}
	c := &AssetCache{
		baseDir: baseDir,
		assets: make(map[string]*CacheEntry),
	}
	if err := c.loadManifest(); err != nil {
		// If manifest doesn't exist, start with empty cache
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load asset cache manifest: %w", err)
		}
	}
	return c, nil
}

// cacheKey generates a deterministic cache key for an asset.
func cacheKey(category, name, version, arch string) string {
	if arch != "" {
		return filepath.Join(category, name, version, arch)
	}
	return filepath.Join(category, name, version)
}

// Check returns the cache entry if the asset is cached locally, or nil if not found.
func (c *AssetCache) Check(category, name, version, arch string) *CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := cacheKey(category, name, version, arch)
	entry, exists := c.assets[key]
	if !exists {
		return nil
	}
	// Verify the file actually exists on disk
	if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
		return nil
	}
	return entry
}

// Exists returns true if the asset is cached and the file exists on disk.
func (c *AssetCache) Exists(category, name, version, arch string) bool {
	return c.Check(category, name, version, arch) != nil
}

// Put records an asset as cached.
func (c *AssetCache) Put(category, name, version, arch string, entry *CacheEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(category, name, version, arch)
	c.assets[key] = entry
	return c.saveManifest()
}

// PutLocal records an asset from the local filesystem.
func (c *AssetCache) PutLocal(category, name, version, arch, path string, sizeBytes int64, sha256 string) error {
	entry := &CacheEntry{
		Path:      path,
		SizeBytes: sizeBytes,
		SHA256:    sha256,
		CachedAt:  time.Now(),
	}
	return c.Put(category, name, version, arch, entry)
}

// PutForCluster records an asset as cached for a specific cluster.
func (c *AssetCache) PutForCluster(category, name, version, arch, path, clusterName string, sizeBytes int64, sha256 string) error {
	entry := &CacheEntry{
		Path:        path,
		SizeBytes:   sizeBytes,
		SHA256:      sha256,
		CachedAt:    time.Now(),
		ClusterName: clusterName,
	}
	return c.Put(category, name, version, arch, entry)
}

// Remove removes an asset from the cache.
func (c *AssetCache) Remove(category, name, version, arch string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := cacheKey(category, name, version, arch)
	delete(c.assets, key)
	return c.saveManifest()
}

// List returns all cache entries for a given category, or all entries if category is empty.
func (c *AssetCache) List(category string) []*CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var entries []*CacheEntry
	for key, entry := range c.assets {
		if category == "" || len(key) > len(category) && key[:len(category)+1] == category+string(filepath.Separator) {
			entries = append(entries, entry)
		}
	}
	return entries
}

// Stats returns cache statistics.
func (c *AssetCache) Stats() (totalAssets, totalSize int64, categories map[string]int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	categories = make(map[string]int)
	totalSize = 0
	for key, entry := range c.assets {
		// Extract category from key
		parts := filepath.SplitList(key)
		if len(parts) > 0 {
			categories[parts[0]]++
		}
		totalAssets++
		totalSize += entry.SizeBytes
	}
	return
}

// cacheManifest is the on-disk format for the cache.
type cacheManifest struct {
	Version   int                     `json:"version"`
	UpdatedAt time.Time               `json:"updatedAt"`
	Entries   map[string]*CacheEntry `json:"entries"`
}

func (c *AssetCache) manifestPath() string {
	return filepath.Join(c.baseDir, "manifest.json")
}

func (c *AssetCache) loadManifest() error {
	path := c.manifestPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m cacheManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("failed to unmarshal cache manifest: %w", err)
	}
	c.assets = m.Entries
	return nil
}

func (c *AssetCache) saveManifest() error {
	if err := os.MkdirAll(c.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	m := cacheManifest{
		Version:   1,
		UpdatedAt: time.Now(),
		Entries:   c.assets,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache manifest: %w", err)
	}
	tmpPath := c.manifestPath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache manifest: %w", err)
	}
	if err := os.Rename(tmpPath, c.manifestPath()); err != nil {
		return fmt.Errorf("failed to rename cache manifest: %w", err)
	}
	return nil
}
