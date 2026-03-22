package images

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type PushManifestListStep struct {
	step.Base
	Concurrency int
}

type PushManifestListStepBuilder struct {
	step.Builder[PushManifestListStepBuilder, *PushManifestListStep]
}

func NewPushManifestListStepBuilder(ctx runtime.ExecutionContext, instanceName string) *PushManifestListStepBuilder {
	if ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting == nil ||
		ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting.PrivateRegistry == "" {
		return nil
	}

	s := &PushManifestListStep{
		Concurrency: 5,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Create and push multi-architecture manifest lists to the private registry", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 30 * time.Minute

	b := new(PushManifestListStepBuilder).Init(s)
	return b
}

func (b *PushManifestListStepBuilder) WithConcurrency(c int) *PushManifestListStepBuilder {
	if c > 0 {
		b.Step.Concurrency = c
	}
	return b
}

func (s *PushManifestListStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PushManifestListStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if _, err := exec.LookPath("skopeo"); err != nil {
		return false, fmt.Errorf("skopeo command not found in PATH, please install it first")
	}
	if _, ok := ctx.GetTaskCache().Get("manifestList"); !ok {
		return false, fmt.Errorf("manifestList not found in cache, ensure push images step ran successfully")
	}
	return false, nil
}

func (s *PushManifestListStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	val, ok := ctx.GetTaskCache().Get("manifestList")
	if !ok {
		failErr := fmt.Errorf("manifestList not found in cache, cannot proceed")
		result.MarkFailed(failErr, "manifestList not found in cache, cannot proceed")
		return result, failErr
	}
	manifestList, ok := val.(map[string][]manifestEntry)
	if !ok {
		failErr := fmt.Errorf("invalid type for manifestList in cache")
		result.MarkFailed(failErr, "invalid type for manifestList in cache")
		return result, failErr
	}

	if len(manifestList) == 0 {
		logger.Info("No manifest lists to create. Skipping.")
		result.MarkCompleted("No manifest lists to create. Skipping.")
		return result, nil
	}

	type pushJob struct {
		BaseImageName string
		Entries       []manifestEntry
	}

	jobs := make(chan pushJob, len(manifestList))
	errChan := make(chan error, len(manifestList))

	var wg sync.WaitGroup
	for i := 0; i < s.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				err := s.createAndPushManifest(ctx, job.BaseImageName, job.Entries)
				if err != nil {
					logger.Errorf("Worker %d: failed to push manifest for %s: %v", workerID, job.BaseImageName, err)
					errChan <- err
				}
			}
		}(i)
	}

	for baseName, entries := range manifestList {
		jobs <- pushJob{BaseImageName: baseName, Entries: entries}
	}
	close(jobs)
	wg.Wait()
	close(errChan)

	var allErrors []string
	for err := range errChan {
		allErrors = append(allErrors, err.Error())
	}
	if len(allErrors) > 0 {
		failErr := fmt.Errorf("failed to push some manifest lists:\n- %s", strings.Join(allErrors, "\n- "))
		result.MarkFailed(failErr, "failed to push some manifest lists")
		return result, failErr
	}

	logger.Info("All multi-architecture manifest lists have been pushed successfully.")
	result.MarkCompleted("All multi-architecture manifest lists have been pushed successfully")
	return result, nil
}

func (s *PushManifestListStep) createAndPushManifest(ctx runtime.ExecutionContext, baseImageName string, entries []manifestEntry) error {
	logger := ctx.GetLogger()

	destManifestList := "docker://" + baseImageName

	createArgs := []string{"manifest", "create", destManifestList}
	for _, entry := range entries {
		createArgs = append(createArgs, entry.Image)
	}

	logger.Infof("Creating manifest list for %s", baseImageName)
	createCmd := exec.Command("skopeo", createArgs...)
	if output, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create manifest list for %s: %w\nOutput: %s", baseImageName, err, string(output))
	}

	pushArgs := []string{"manifest", "push", "--all"}

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
		pushArgs = append(pushArgs, "--creds", creds)
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
		pushArgs = append(pushArgs, "--tls-verify=false")
	}

	pushArgs = append(pushArgs, destManifestList)

	logger.Infof("Pushing manifest list for %s", baseImageName)
	pushCmd := exec.Command("skopeo", pushArgs...)
	if output, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push manifest list for %s: %w\nOutput: %s", baseImageName, err, string(output))
	}

	return nil
}

func (s *PushManifestListStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for PushManifestListStep is not implemented, as it would require deleting manifest lists from the private registry.")
	return nil
}

var _ step.Step = (*PushManifestListStep)(nil)