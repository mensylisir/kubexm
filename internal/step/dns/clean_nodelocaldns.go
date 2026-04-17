package dns

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CleanNodeLocalDNSStep struct {
	step.Base
}

type CleanNodeLocalDNSStepBuilder struct {
	step.Builder[CleanNodeLocalDNSStepBuilder, *CleanNodeLocalDNSStep]
}

func NewCleanNodeLocalDNSStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanNodeLocalDNSStepBuilder {
	s := &CleanNodeLocalDNSStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Cleanup NodeLocal DNSCache addon", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CleanNodeLocalDNSStepBuilder).Init(s)
	return b
}

func (s *CleanNodeLocalDNSStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanNodeLocalDNSStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	checkCmd := "kubectl get daemonset node-local-dns -n kube-system"
	_, err = runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "notfound") {
			logger.Info("NodeLocal DNSCache daemonset not found. Step is done.")
			return true, nil
		}
		logger.Warn(err, "Failed to check for NodeLocal DNSCache daemonset, assuming cleanup is required.")
		return false, nil
	}

	logger.Info("NodeLocal DNSCache daemonset found. Cleanup is required.")
	return false, nil
}

func (s *CleanNodeLocalDNSStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	remoteManifestPath := filepath.Join(ctx.GetUploadDir(), ctx.GetHost().GetName(), "nodelocaldns.yaml")

	exists, err := runner.Exists(ctx.GoContext(), conn, remoteManifestPath)
	if err != nil {
		logger.Warn(err, "Failed to check for manifest file. Will attempt deletion by label.", "path", remoteManifestPath)
		return s.deleteByLabel(ctx)
	}

	if exists {
		deleteCmd := fmt.Sprintf("kubectl delete -f %s --ignore-not-found=true", remoteManifestPath)
		logger.Info("Cleaning up NodeLocal DNSCache using manifest.", "command", deleteCmd)

		runResult, err := runner.Run(ctx.GoContext(), conn, deleteCmd, s.Sudo)
		if err != nil {
			logger.Warn(err, "kubectl delete -f command failed, will attempt deletion by label as a fallback.", "output", runResult.Stdout)
			return s.deleteByLabel(ctx)
		}

		logger.Info("NodeLocal DNSCache cleanup using manifest finished successfully.")
		result.MarkCompleted("cleanup completed")
		return result, nil
	}

	logger.Warn("Manifest file not found. Attempting deletion by label.", "path", remoteManifestPath)
	return s.deleteByLabel(ctx)
}

func (s *CleanNodeLocalDNSStep) deleteByLabel(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()

	label := "k8s-app=node-local-dns"
	namespace := "kube-system"

	resourcesToDelete := []string{"daemonset", "service", "configmap", "serviceaccount"}

	for _, resource := range resourcesToDelete {
		deleteCmd := fmt.Sprintf("kubectl delete %s -n %s -l %s --ignore-not-found=true", resource, namespace, label)
		logger.Info("Attempting to delete resource by label.", "resource", resource, "command", deleteCmd)
		if _, err := runner.Run(ctx.GoContext(), conn, deleteCmd, s.Sudo); err != nil {
			logger.Warn(err, "Failed to delete resource by label (this may be ok).", "resource", resource)
		}
	}
	result.MarkCompleted("cleanup by label completed")
	return result, nil
}

func (s *CleanNodeLocalDNSStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step has no rollback action.")
	return nil
}

var _ step.Step = (*CleanNodeLocalDNSStep)(nil)
