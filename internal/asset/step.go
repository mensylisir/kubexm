package asset

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// AssetManagerStep is a unified step that downloads all required assets using the centralized AssetManager.
type AssetManagerStep struct {
	step.Base
	Concurrency int
	CacheDir    string
}

type AssetManagerStepBuilder struct {
	step.Builder[AssetManagerStepBuilder, *AssetManagerStep]
}

// NewAssetManagerStepBuilder creates a new asset manager step builder.
func NewAssetManagerStepBuilder(ctx runtime.ExecutionContext, instanceName string) *AssetManagerStepBuilder {
	s := &AssetManagerStep{
		Concurrency: 5,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "[AssetManager] Download and manage all cluster assets (binaries, images, helm charts)"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 60 * time.Minute

	return new(AssetManagerStepBuilder).Init(s)
}

func (b *AssetManagerStepBuilder) WithConcurrency(c int) *AssetManagerStepBuilder {
	if c > 0 {
		b.Step.Concurrency = c
	}
	return b
}

// WithCacheDir sets the directory for the asset cache.
func (b *AssetManagerStepBuilder) WithCacheDir(dir string) *AssetManagerStepBuilder {
	b.Step.CacheDir = dir
	return b
}

func (s *AssetManagerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *AssetManagerStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil // Always run to ensure assets are up to date
}

func (s *AssetManagerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	log := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	// Determine cache directory
	cacheDir := s.CacheDir
	if cacheDir == "" {
		cacheDir = ctx.GetGlobalWorkDir()
	}

	// Create the centralized asset manager
	manager, err := NewManager(ctx, cacheDir)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to create asset manager: %v", err))
		return result, err
	}

	// Get the asset manifest
	manifest, err := manager.GetManifest()
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to get asset manifest: %v", err))
		return result, err
	}

	totalBin, totalImg, totalHelm := manifest.Stats()
	log.Info("Asset manifest built.",
		"binaries", totalBin,
		"images", totalImg,
		"helm_charts", totalHelm)

	// Report cache stats
	if totalBin, totalSize, categories := manager.GetCacheStats(); totalBin > 0 {
		log.Info("Asset cache stats.",
			"cached_assets", totalBin,
			"total_size_bytes", totalSize,
			"categories", categories)
	}

	// Download all binaries
	if len(manifest.Binaries) > 0 {
		log.Info("Downloading binaries...", "count", len(manifest.Binaries))
		results := manager.DownloadAll(manifest, s.Concurrency)

		var failures []string
		var cached, downloaded int
		for res := range results {
			if res.Success {
				if res.Message == "cached and verified" || res.Message == "cached (no checksum to verify)" {
					cached++
				} else {
					downloaded++
				}
			} else {
				failures = append(failures, fmt.Sprintf("%s: %s", res.Asset.Name, res.Message))
			}
		}

		log.Info("Binary download complete.", "cached", cached, "downloaded", downloaded, "failed", len(failures))
		if len(failures) > 0 {
			result.MarkFailed(fmt.Errorf("some binaries failed to download"), fmt.Sprintf("%d failures", len(failures)))
			return result, fmt.Errorf("binary download failures: %v", failures)
		}
	}

	result.MarkCompleted(fmt.Sprintf("Assets ready: %d binaries, %d images, %d helm charts",
		len(manifest.Binaries), len(manifest.Images), len(manifest.Helm)))
	return result, nil
}

func (s *AssetManagerStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil // Downloaded assets are not rolled back
}

var _ step.Step = (*AssetManagerStep)(nil)
