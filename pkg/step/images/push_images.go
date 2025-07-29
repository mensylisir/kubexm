package images

import (
	"encoding/json"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

type manifestEntry struct {
	Image    string      `json:"image"`
	Platform v1.Platform `json:"platform"`
}

type PushImagesStep struct {
	step.Base
	ImagesDir   string
	Concurrency int
}

type PushImagesStepBuilder struct {
	step.Builder[PushImagesStepBuilder, *PushImagesStep]
}

func NewPushImagesStepBuilder(ctx runtime.Context, instanceName string) *PushImagesStepBuilder {
	if ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting == nil ||
		ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting.PrivateRegistry == "" {
		return nil
	}

	s := &PushImagesStep{
		ImagesDir:   filepath.Join(ctx.GetClusterArtifactsDir(), "images"),
		Concurrency: 5,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Push all saved images to the private registry", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 60 * time.Minute

	b := new(PushImagesStepBuilder).Init(s)
	return b
}

func (b *PushImagesStepBuilder) WithConcurrency(c int) *PushImagesStepBuilder {
	if c > 0 {
		b.Step.Concurrency = c
	}
	return b
}

func (s *PushImagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PushImagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if _, err := exec.LookPath("skopeo"); err != nil {
		return false, fmt.Errorf("skopeo command not found in PATH, please install it first")
	}

	ociIndexPath := filepath.Join(s.ImagesDir, "index.json")
	if _, err := os.Stat(ociIndexPath); os.IsNotExist(err) {
		return false, fmt.Errorf("local OCI image cache at '%s' not found, ensure save step ran successfully", s.ImagesDir)
	}
	return false, nil
}

func (s *PushImagesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	indexJsonPath := filepath.Join(s.ImagesDir, "index.json")
	indexJsonContent, err := os.ReadFile(indexJsonPath)
	if err != nil {
		return fmt.Errorf("failed to read local index.json: %w", err)
	}
	var ociIndex v1.Index
	if err := json.Unmarshal(indexJsonContent, &ociIndex); err != nil {
		return fmt.Errorf("failed to unmarshal OCI index: %w", err)
	}

	imageProvider := images.NewImageProvider(ctx)
	jobs := make(chan v1.Descriptor, len(ociIndex.Manifests))
	errChan := make(chan error, len(ociIndex.Manifests))

	manifestList := make(map[string][]manifestEntry)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < s.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				originalRefWithArch := job.Annotations["org.opencontainers.image.ref.name"]

				parts := strings.Split(originalRefWithArch, "-")
				arch := parts[len(parts)-1]
				originalRef := strings.TrimSuffix(originalRefWithArch, "-"+arch)

				refParts := strings.Split(originalRef, ":")
				imageNameWithRepo := refParts[0]

				componentName := images.GetComponentNameByRef(imageNameWithRepo)
				if componentName == "" {
					errChan <- fmt.Errorf("could not find component name for image ref: %s", imageNameWithRepo)
					continue
				}

				imgObj := imageProvider.GetImage(componentName)
				if imgObj == nil {
					errChan <- fmt.Errorf("could not get image object for component: %s", componentName)
					continue
				}

				err := s.copyToRegistry(ctx, originalRefWithArch, imgObj, arch)
				if err != nil {
					errChan <- err
					continue
				}

				mu.Lock()
				baseImageName := imgObj.FullName()
				if _, ok := manifestList[baseImageName]; !ok {
					manifestList[baseImageName] = []manifestEntry{}
				}
				manifestList[baseImageName] = append(manifestList[baseImageName], manifestEntry{
					Image: fmt.Sprintf("%s-%s", baseImageName, arch),
					Platform: v1.Platform{
						OS:           "linux",
						Architecture: arch,
					},
				})
				mu.Unlock()
			}
		}(i)
	}

	for _, manifest := range ociIndex.Manifests {
		jobs <- manifest
	}
	close(jobs)
	wg.Wait()
	close(errChan)

	var allErrors []string
	for err := range errChan {
		allErrors = append(allErrors, err.Error())
	}
	if len(allErrors) > 0 {
		return fmt.Errorf("failed to push some images:\n- %s", strings.Join(allErrors, "\n- "))
	}

	ctx.GetTaskCache().Set("manifestList", manifestList)
	logger.Info("All saved images have been pushed to the private registry successfully.")
	return nil
}

func (s *PushImagesStep) copyToRegistry(ctx runtime.ExecutionContext, ociRef string, img *images.Image, arch string) error {
	srcName := fmt.Sprintf("oci:%s:%s", s.ImagesDir, ociRef)

	destNameWithArch := fmt.Sprintf("%s-%s", img.FullName(), arch)
	destName := "docker://" + destNameWithArch

	args := []string{
		"copy",
		"--all",
	}

	privateRegistryHost := ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting.PrivateRegistry
	if u, err := url.Parse("scheme://" + privateRegistryHost); err == nil {
		privateRegistryHost = u.Host
	}

	var creds string
	if auths := ctx.GetClusterConfig().Spec.Registry.Auths; auths != nil {
		if auth, ok := auths[privateRegistryHost]; ok {
			if auth.Username != "" && auth.Password != "" {
				creds = fmt.Sprintf("%s:%s", auth.Username, auth.Password)
			}
		}
	}
	if creds != "" {
		args = append(args, "--dest-creds", creds)
	}

	skipVerify := false
	if auths := ctx.GetClusterConfig().Spec.Registry.Auths; auths != nil {
		if auth, ok := auths[privateRegistryHost]; ok {
			if (auth.PlainHTTP != nil && *auth.PlainHTTP) || (auth.SkipTLSVerify != nil && *auth.SkipTLSVerify) {
				skipVerify = true
			}
		}
	}
	if skipVerify {
		args = append(args, "--dest-tls-verify=false")
	}

	args = append(args, srcName, destName)

	cmd := exec.Command("skopeo", args...)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push %s to %s: %w\nOutput: %s", srcName, destName, err, string(output))
	}

	return nil
}

func (s *PushImagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for PushImagesStep is not implemented, as it would require deleting images from the private registry.")
	return nil
}

var _ step.Step = (*PushImagesStep)(nil)
