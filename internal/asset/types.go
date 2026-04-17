package asset

import "time"

// Category represents the type of asset.
type Category string

const (
	CategoryBinary Category = "binary"
	CategoryImage  Category = "image"
	CategoryHelm   Category = "helm"
	CategoryScript Category = "script"
)

// Asset represents a unified asset descriptor that can be any type of downloadable resource.
type Asset struct {
	// Name is the unique identifier for this asset (e.g., "kubelet", "calico/node", "ingress-nginx").
	Name string
	// Category identifies the type of asset.
	Category Category
	// Version is the semantic version of the asset.
	Version string
	// Arch is the target architecture (e.g., "amd64", "arm64"). Empty for non-arch-specific assets.
	Arch string
	// URL is the source download URL.
	URL string
	// SHA256 is the expected SHA-256 checksum. Empty if no verification is available.
	SHA256 string
	// LocalPath is the absolute path where the asset is stored locally.
	LocalPath string
	// SizeBytes is the expected file size in bytes (optional, for progress tracking).
	SizeBytes int64
	// IsArchive indicates whether this asset is a compressed archive that needs extraction.
	IsArchive bool
	// IsCached indicates whether this asset is already available locally.
	IsCached bool
	// CachedAt is when the asset was first cached.
	CachedAt time.Time
	// Description is a human-readable description of this asset.
	Description string
}

// BinaryAsset is a binary-specific asset with additional metadata.
type BinaryAsset struct {
	Asset
	// ComponentDir is the component-specific subdirectory.
	ComponentDir string
	// FileName is the actual filename on disk.
	FileName string
}

// ImageAsset is an image-specific asset with registry information.
type ImageAsset struct {
	Asset
	// SourceRegistry is the original source registry (e.g., "quay.io").
	SourceRegistry string
	// TargetRegistry is the target private registry (e.g., "harbor.example.com").
	TargetRegistry string
	// Namespace is the image namespace within the registry.
	Namespace string
	// OriginalName is the full original image name with registry and tag.
	OriginalName string
	// TargetName is the full target image name with registry and tag.
	TargetName string
}

// HelmAsset is a helm chart asset.
type HelmAsset struct {
	Asset
	// ChartName is the Helm chart name.
	ChartName string
	// RepoURL is the Helm repository URL.
	RepoURL string
}

// DownloadResult describes the outcome of a download operation.
type DownloadResult struct {
	Asset    *Asset
	Success  bool
	Message  string
	Duration time.Duration
}

// VerificationResult describes the outcome of a verification operation.
type VerificationResult struct {
	Asset   *Asset
	Valid   bool
	Message string
}

// AssetManifest is a complete manifest of all assets needed for a cluster.
type AssetManifest struct {
	Binaries []*BinaryAsset
	Images   []*ImageAsset
	Helm     []*HelmAsset
}

// Stats returns summary statistics about the manifest.
func (m *AssetManifest) Stats() (totalBin, totalImg, totalHelm int) {
	if m != nil {
		return len(m.Binaries), len(m.Images), len(m.Helm)
	}
	return 0, 0, 0
}
