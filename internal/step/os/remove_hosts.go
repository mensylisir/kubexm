package os

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/pkg/errors"
)

const KubeXMBlockRegex = `(?s)# KubeXM hosts BEGIN.*# KubeXM hosts END\s*`

type RemoveEtcHostsStep struct {
	step.Base
	removedKubeXMBlock string
}

type RemoveEtcHostsStepBuilder struct {
	step.Builder[RemoveEtcHostsStepBuilder, *RemoveEtcHostsStep]
}

func NewRemoveEtcHostsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RemoveEtcHostsStepBuilder {
	s := &RemoveEtcHostsStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Remove KubeXM entries from /etc/hosts", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(RemoveEtcHostsStepBuilder).Init(s)
	return b
}

func (s *RemoveEtcHostsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveEtcHostsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	currentContentBytes, err := runner.ReadFile(ctx.GoContext(), conn, "/etc/hosts")
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "No such file or directory") {
			logger.Info("/etc/hosts not found, nothing to remove.")
			return true, nil
		}
		return false, errors.Wrap(err, "failed to read /etc/hosts on remote host")
	}
	currentContent := string(currentContentBytes)

	re := regexp.MustCompile(KubeXMBlockRegex)
	if !re.MatchString(currentContent) {
		logger.Info("KubeXM hosts block not found in /etc/hosts. Nothing to do.")
		return true, nil
	}

	logger.Info("KubeXM hosts block found, needs to be removed.")
	return false, nil
}

func (s *RemoveEtcHostsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())

	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "step failed"); return result, err
	}

	currentContentBytes, err := runner.ReadFile(ctx.GoContext(), conn, "/etc/hosts")
	if err != nil && !os.IsNotExist(err) && !strings.Contains(err.Error(), "No such file or directory") {
		result.MarkFailed(err, "failed to read /etc/hosts before removal"); return result, err
	}
	currentContent := string(currentContentBytes)

	re := regexp.MustCompile(KubeXMBlockRegex)
	s.removedKubeXMBlock = re.FindString(currentContent)

	if s.removedKubeXMBlock == "" {
		logger.Info("KubeXM hosts block not found. No changes made.")
	result.MarkCompleted("step completed successfully"); return result, nil
	}

	finalContent := re.ReplaceAllString(currentContent, "")
	finalContent = strings.TrimSpace(finalContent) + "\n"

	logger.Info("Removing KubeXM block from /etc/hosts...")
	err = helpers.WriteContentToRemote(ctx, conn, finalContent, "/etc/hosts", "0644", s.Sudo)
	if err != nil {
		result.MarkFailed(err, "failed to write cleaned content to /etc/hosts"); return result, err
	}

	logger.Infof("/etc/hosts cleaned up successfully.")
	result.MarkCompleted("step completed successfully"); return result, nil
}

func (s *RemoveEtcHostsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if s.removedKubeXMBlock == "" {
		logger.Info("Nothing to roll back as no hosts block was removed in the run step.")
		return nil
	}

	logger.Info("Attempting to roll back hosts removal by re-adding the KubeXM block...")

	currentContentBytes, err := runner.ReadFile(ctx.GoContext(), conn, "/etc/hosts")
	if err != nil && !os.IsNotExist(err) && !strings.Contains(err.Error(), "No such file or directory") {
		return err
	}
	currentContent := string(currentContentBytes)

	finalContent := strings.TrimSpace(currentContent) + "\n" + strings.TrimSpace(s.removedKubeXMBlock) + "\n"

	err = helpers.WriteContentToRemote(ctx, conn, finalContent, "/etc/hosts", "0644", s.Sudo)
	if err != nil {
		return errors.Wrapf(err, "failed to write rolled back content to /etc/hosts")
	}

	logger.Infof("/etc/hosts has been restored.")
	return nil
}
