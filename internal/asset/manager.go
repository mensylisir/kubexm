package asset

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	binaryBOM "github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	helmBOM "github.com/mensylisir/kubexm/internal/util/helm"
	imagesBOM "github.com/mensylisir/kubexm/internal/util/images"
)

// Manager is the central asset management interface that unifies binaries, images, and helm charts.
type Manager struct {
	ctx        runtime.ExecutionContext
	binaryProv *binaryBOM.BinaryProvider
	imageProv  *imagesBOM.ImageProvider
	helmProv   *helmBOM.HelmProvider
	cache      *AssetCache
	httpClient *http.Client
}

// NewManager creates a new centralized asset manager.
// The cacheDir is used for the asset cache. If empty, caching is disabled.
func NewManager(ctx runtime.ExecutionContext, cacheDir string) (*Manager, error) {
	m := &Manager{
		ctx:        ctx,
		binaryProv: binaryBOM.NewBinaryProvider(ctx),
		imageProv:  imagesBOM.NewImageProvider(ctx),
		helmProv:   helmBOM.NewHelmProvider(ctx),
		httpClient: &http.Client{Timeout: 30 * time.Minute},
	}
	if cacheDir != "" {
		cache, err := NewAssetCache(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create asset cache: %w", err)
		}
		m.cache = cache
	}
	return m, nil
}

// GetManifest returns all assets needed for the current cluster configuration.
func (m *Manager) GetManifest() (*AssetManifest, error) {
	cfg := m.ctx.GetClusterConfig()
	if cfg == nil {
		return nil, fmt.Errorf("cluster config is not available in context")
	}

	manifest := &AssetManifest{}

	// Collect binaries for all required architectures
	archs := m.getRequiredArchs()
	for _, arch := range archs {
		binaries, err := m.binaryProv.GetBinaries(arch)
		if err != nil {
			return nil, fmt.Errorf("failed to get binaries for arch %s: %w", arch, err)
		}
		for _, b := range binaries {
			asset := m.binaryToAsset(b)
			manifest.Binaries = append(manifest.Binaries, asset)
		}
	}

	// Collect images
	images := m.imageProv.GetImages()
	for _, img := range images {
		asset := m.imageToAsset(img)
		manifest.Images = append(manifest.Images, asset)
	}

	// Collect helm charts
	charts := m.helmProv.GetCharts()
	for _, chart := range charts {
		asset := m.helmToAsset(chart)
		manifest.Helm = append(manifest.Helm, asset)
	}

	return manifest, nil
}

// GetBinaryAsset returns a single binary asset by name and architecture.
func (m *Manager) GetBinaryAsset(name, arch string) (*BinaryAsset, error) {
	b, err := m.binaryProv.GetBinary(name, arch)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}
	return m.binaryToAsset(b), nil
}

// GetImageAsset returns a single image asset by name.
func (m *Manager) GetImageAsset(name string) *ImageAsset {
	img := m.imageProv.GetImage(name)
	if img == nil {
		return nil
	}
	return m.imageToAsset(img)
}

// GetHelmAsset returns a single helm chart asset by name.
func (m *Manager) GetHelmAsset(name string) *HelmAsset {
	chart := m.helmProv.GetChart(name)
	if chart == nil {
		return nil
	}
	return m.helmToAsset(chart)
}

// DownloadAll downloads all assets in the manifest concurrently.
func (m *Manager) DownloadAll(manifest *AssetManifest, concurrency int) <-chan *DownloadResult {
	if concurrency <= 0 {
		concurrency = 5
	}

	type workItem struct {
		asset  *Asset
		binary *BinaryAsset
	}

	var work []workItem
	for _, b := range manifest.Binaries {
		work = append(work, workItem{asset: &b.Asset, binary: b})
	}

	results := make(chan *DownloadResult, len(work))
	jobs := make(chan workItem, len(work))
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				start := time.Now()
				result := m.downloadBinary(item.binary)
				result.Duration = time.Since(start)
				results <- result
			}
		}()
	}

	go func() {
		for _, w := range work {
			jobs <- w
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	return results
}

// IsCached returns true if the asset is available in the local cache.
func (m *Manager) IsCached(category Category, name, version, arch string) bool {
	if m.cache == nil {
		return false
	}
	return m.cache.Exists(string(category), name, version, arch)
}

// GetCachedPath returns the cached path for an asset, or empty string if not cached.
func (m *Manager) GetCachedPath(category Category, name, version, arch string) string {
	if m.cache == nil {
		return ""
	}
	entry := m.cache.Check(string(category), name, version, arch)
	if entry == nil {
		return ""
	}
	return entry.Path
}

// GetCacheStats returns statistics about the asset cache.
func (m *Manager) GetCacheStats() (totalAssets, totalSize int64, categories map[string]int) {
	if m.cache == nil {
		return 0, 0, map[string]int{}
	}
	return m.cache.Stats()
}

// --- Internal helpers ---

func (m *Manager) getRequiredArchs() []string {
	archSet := make(map[string]bool)
	allHosts := m.ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return []string{"amd64"}
	}
	for _, host := range allHosts {
		archSet[host.GetArch()] = true
	}
	var archs []string
	for arch := range archSet {
		archs = append(archs, arch)
	}
	return archs
}

func (m *Manager) downloadBinary(asset *BinaryAsset) *DownloadResult {
	result := &DownloadResult{Asset: &asset.Asset}

	// Check cache
	if m.cache != nil {
		entry := m.cache.Check(string(CategoryBinary), asset.Name, asset.Version, asset.Arch)
		if entry != nil {
			// Verify cached file exists
			if _, err := os.Stat(entry.Path); err == nil {
				// Verify checksum if available
				if asset.SHA256 != "" {
					valid, _ := m.verifyChecksum(entry.Path, asset.SHA256)
					if valid {
						result.Success = true
						result.Message = "cached and verified"
						return result
					}
					// Checksum mismatch - re-download
				} else {
					result.Success = true
					result.Message = "cached (no checksum to verify)"
					return result
				}
			}
		}
	}

	// Download
	if err := os.MkdirAll(filepath.Dir(asset.LocalPath), 0755); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to create directory: %v", err)
		return result
	}

	resp, err := m.httpClient.Get(asset.URL)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("HTTP GET failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Success = false
		result.Message = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, asset.URL)
		return result
	}

	tmpPath := asset.LocalPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to create temp file: %v", err)
		return result
	}

	written, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		result.Success = false
		result.Message = fmt.Sprintf("failed to write file: %v", err)
		return result
	}

	if err := os.Rename(tmpPath, asset.LocalPath); err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to rename temp file: %v", err)
		return result
	}

	// Verify checksum
	if asset.SHA256 != "" {
		valid, err := m.verifyChecksum(asset.LocalPath, asset.SHA256)
		if err != nil {
			os.Remove(asset.LocalPath)
			result.Success = false
			result.Message = fmt.Sprintf("checksum error: %v", err)
			return result
		}
		if !valid {
			os.Remove(asset.LocalPath)
			result.Success = false
			result.Message = "checksum mismatch"
			return result
		}
	}

	// Update cache
	if m.cache != nil {
		clusterName := ""
		if m.ctx.GetClusterConfig() != nil {
			clusterName = m.ctx.GetClusterConfig().Name
		}
		checksum := asset.SHA256
		if checksum == "" {
			checksum, _ = m.computeChecksum(asset.LocalPath)
		}
		m.cache.PutForCluster(string(CategoryBinary), asset.Name, asset.Version, asset.Arch,
			asset.LocalPath, clusterName, written, checksum)
	}

	result.Success = true
	result.Message = "downloaded successfully"
	return result
}

func (m *Manager) verifyChecksum(path, expected string) (bool, error) {
	if expected == "" {
		return true, nil
	}
	actual, err := m.computeChecksum(path)
	if err != nil {
		return false, err
	}
	return actual == expected, nil
}

func (m *Manager) computeChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (m *Manager) binaryToAsset(b *binaryBOM.Binary) *BinaryAsset {
	asset := &BinaryAsset{
		Asset: Asset{
			Name:        b.ComponentName,
			Category:    CategoryBinary,
			Version:     b.Version,
			Arch:        b.Arch,
			URL:         b.URL(),
			SHA256:      b.Checksum(),
			LocalPath:   b.FilePath(),
			IsArchive:  b.IsArchive(),
			Description: fmt.Sprintf("Binary: %s %s (%s)", b.ComponentName, b.Version, b.Arch),
		},
		FileName: b.FileName(),
	}

	// Check if cached
	if m.cache != nil {
		if entry := m.cache.Check(string(CategoryBinary), b.ComponentName, b.Version, b.Arch); entry != nil {
			if _, err := os.Stat(entry.Path); err == nil {
				asset.IsCached = true
				asset.LocalPath = entry.Path
				asset.SizeBytes = entry.SizeBytes
				if entry.SHA256 != "" {
					asset.SHA256 = entry.SHA256
				}
			}
		}
	}

	return asset
}

func (m *Manager) imageToAsset(img *imagesBOM.Image) *ImageAsset {
	asset := &ImageAsset{
		Asset: Asset{
			Name:        img.Name(),
			Category:    CategoryImage,
			Version:     img.Tag(),
			Description: fmt.Sprintf("Image: %s", img.OriginalFullName()),
		},
		SourceRegistry: img.OriginalRepoAddr,
		TargetRegistry: img.RegistryAddr(),
		Namespace:     img.Namespace(),
		OriginalName:  img.OriginalFullName(),
		TargetName:    img.FullName(),
	}

	if m.cache != nil {
		if entry := m.cache.Check(string(CategoryImage), img.Name(), img.Tag(), ""); entry != nil {
			asset.IsCached = true
			asset.LocalPath = entry.Path
			asset.SizeBytes = entry.SizeBytes
		}
	}

	return asset
}

func (m *Manager) helmToAsset(chart *helmBOM.HelmChart) *HelmAsset {
	asset := &HelmAsset{
		Asset: Asset{
			Name:        chart.ComponentName,
			Category:    CategoryHelm,
			Version:     chart.Version,
			Description: fmt.Sprintf("Helm Chart: %s %s", chart.ComponentName, chart.Version),
		},
		ChartName: chart.ComponentName,
		RepoURL:   chart.RepoURL(),
	}

	if m.cache != nil {
		if entry := m.cache.Check(string(CategoryHelm), chart.ComponentName, chart.Version, ""); entry != nil {
			asset.IsCached = true
			asset.LocalPath = entry.Path
			asset.SizeBytes = entry.SizeBytes
		}
	}

	return asset
}

// --- Utility: parse version templates ---

func renderURL(tmpl string, version, arch, osName string) string {
	result := strings.ReplaceAll(tmpl, "{{.Version}}", version)
	result = strings.ReplaceAll(result, "{{.Arch}}", arch)
	result = strings.ReplaceAll(result, "{{.OS}}", osName)
	// Handle VersionNoV
	result = strings.ReplaceAll(result, "{{.VersionNoV}}", strings.TrimPrefix(version, "v"))
	// Handle ArchAlias
	switch arch {
	case "amd64":
		result = strings.ReplaceAll(result, "{{.ArchAlias}}", "x86_64")
	case "arm64":
		result = strings.ReplaceAll(result, "{{.ArchAlias}}", "aarch64")
	default:
		result = strings.ReplaceAll(result, "{{.ArchAlias}}", arch)
	}
	return result
}
