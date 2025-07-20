package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/util"
)

type TransformImagesStep struct {
	step.Base
	ImagesToTransform     []util.Image
	TransformedImageNames []string
}

type TransformImagesStepBuilder struct {
	step.Builder[TransformImagesStepBuilder, *TransformImagesStep]
}

func NewTransformImagesStepBuilder(instanceName string, imagesToTransform []util.Image) *TransformImagesStepBuilder {
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

func (s *TransformImagesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Starting image name transformation...")

	registryConfig := ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting
	if registryConfig == nil {
		logger.Info("No registry mirroring or rewriting configuration found. Using default image names.")
		registryConfig = &v1alpha1.RegistryMirroringAndRewriting{}
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

	return nil
}

func (s *TransformImagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for TransformImagesStep is a no-op.")
	return nil
}

var _ step.Step = (*TransformImagesStep)(nil)
