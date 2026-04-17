package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/mensylisir/kubexm/internal/util"
)

type TransformImagesStep struct {
	step.Base
	ImagesToTransform     []util.Image
	TransformedImageNames []string
}

type TransformImagesStepBuilder struct {
	step.Builder[TransformImagesStepBuilder, *TransformImagesStep]
}

func NewTransformImagesStepBuilder(ctx runtime.ExecutionContext, instanceName string, imagesToTransform []util.Image) *TransformImagesStepBuilder {
	cs := &TransformImagesStep{
		ImagesToTransform: imagesToTransform,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Transforming image names based on registry configuration...", instanceName)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 15 * time.Minute
	return new(TransformImagesStepBuilder).Init(cs)
}

func (s *TransformImagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *TransformImagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *TransformImagesStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger.Info("Starting image name transformation...")

	registryConfig := &struct {
		PrivateRegistry   string
		NamespaceOverride string
		NamespaceRewrite  *v1alpha1.NamespaceRewrite
	}{}
	if cfg := ctx.GetClusterConfig(); cfg != nil && cfg.Spec.Registry != nil && cfg.Spec.Registry.MirroringAndRewriting != nil {
		registryConfig.PrivateRegistry = cfg.Spec.Registry.MirroringAndRewriting.PrivateRegistry
		registryConfig.NamespaceOverride = cfg.Spec.Registry.MirroringAndRewriting.NamespaceOverride
		registryConfig.NamespaceRewrite = cfg.Spec.Registry.MirroringAndRewriting.NamespaceRewrite
	}
	if registryConfig.PrivateRegistry == "" {
		logger.Info("No registry mirroring or rewriting configuration found. Using default image names.")
	}

	var finalImageNames []string
	for _, baseImg := range s.ImagesToTransform {
		if !baseImg.Enable {
			continue
		}

		img := baseImg
		if registryConfig.PrivateRegistry != "" {
			img.RepoAddr = registryConfig.PrivateRegistry
		}
		if registryConfig.NamespaceOverride != "" {
			img.NamespaceOverride = registryConfig.NamespaceOverride
		}
		if registryConfig.NamespaceRewrite != nil {
			img.NamespaceRewrite = registryConfig.NamespaceRewrite
		}

		finalName := img.ImageName()
		finalImageNames = append(finalImageNames, finalName)

		logger.Debugf("Transformed image '%s' -> '%s'", baseImg.Repo, finalName)
	}

	s.TransformedImageNames = finalImageNames
	logger.Infof("Image name transformation complete. %d images processed.", len(s.TransformedImageNames))
	result.MarkCompleted(fmt.Sprintf("Transformed %d images", len(s.TransformedImageNames)))
	return result, nil
}

func (s *TransformImagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for TransformImagesStep is a no-op.")
	return nil
}

var _ step.Step = (*TransformImagesStep)(nil)
