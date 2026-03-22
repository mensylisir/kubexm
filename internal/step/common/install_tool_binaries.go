package common

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
	binarybom "github.com/mensylisir/kubexm/internal/util/binaries"
)

type InstallToolBinariesStep struct {
	step.Base
	Tools     []string
	TargetDir string
}

type InstallToolBinariesStepBuilder struct {
	step.Builder[InstallToolBinariesStepBuilder, *InstallToolBinariesStep]
}

func NewInstallToolBinariesStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallToolBinariesStepBuilder {
	s := &InstallToolBinariesStep{
		Tools:     []string{"jq", "yq"},
		TargetDir: "/usr/local/bin",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install required tool binaries (jq/yq)", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(InstallToolBinariesStepBuilder).Init(s)
}

func (b *InstallToolBinariesStepBuilder) WithTools(tools []string) *InstallToolBinariesStepBuilder {
	if len(tools) > 0 {
		b.Step.Tools = tools
	}
	return b
}

func (b *InstallToolBinariesStepBuilder) WithTargetDir(dir string) *InstallToolBinariesStepBuilder {
	if dir != "" {
		b.Step.TargetDir = dir
	}
	return b
}

func (s *InstallToolBinariesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallToolBinariesStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	if len(s.Tools) == 0 {
		return true, nil
	}
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	for _, tool := range s.Tools {
		if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, tool); err != nil {
			return false, nil
		}
	}
	return true, nil
}

func (s *InstallToolBinariesStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	if len(s.Tools) == 0 {
		logger.Info("No tools requested for installation.")
		result.MarkCompleted("No tools requested for installation")
		return result, nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get host connector")
		return result, err
	}

	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.TargetDir, "0755", s.Sudo); err != nil {
		err := fmt.Errorf("failed to ensure target dir %s: %w", s.TargetDir, err)
		result.MarkFailed(err, "failed to create target directory")
		return result, err
	}

	arch := ctx.GetHost().GetArch()
	provider := binarybom.NewBinaryProvider(ctx)

	for _, tool := range s.Tools {
		if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, tool); err == nil {
			logger.Infof("Tool '%s' already available on host.", tool)
			continue
		}

		component := componentForTool(tool)
		if component == "" {
			err := fmt.Errorf("unsupported tool '%s'", tool)
			result.MarkFailed(err, "unsupported tool")
			return result, err
		}

		binaryInfo, err := provider.GetBinary(component, arch)
		if err != nil {
			err := fmt.Errorf("failed to resolve binary for %s: %w", tool, err)
			result.MarkFailed(err, "failed to resolve binary")
			return result, err
		}
		if binaryInfo == nil {
			err := fmt.Errorf("binary info for tool %s is not available; ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle)", tool)
			result.MarkFailed(err, "binary info not available")
			return result, err
		}

		localPath := binaryInfo.FilePath()
		if _, err := os.Stat(localPath); err != nil {
			err := fmt.Errorf("local tool binary '%s' not found, ensure assets were prepared (kubexm download or Preflight PrepareAssets/ExtractBundle)", localPath)
			result.MarkFailed(err, "local binary not found")
			return result, err
		}

		remotePath := filepath.Join(s.TargetDir, tool)
		if err := runnerSvc.Upload(ctx.GoContext(), conn, localPath, remotePath, s.Sudo); err != nil {
			err := fmt.Errorf("failed to upload tool %s to %s: %w", tool, remotePath, err)
			result.MarkFailed(err, "failed to upload tool")
			return result, err
		}
		if err := runnerSvc.Chmod(ctx.GoContext(), conn, remotePath, "0755", s.Sudo); err != nil {
			err := fmt.Errorf("failed to chmod %s: %w", remotePath, err)
			result.MarkFailed(err, "failed to chmod")
			return result, err
		}
		logger.Infof("Installed tool '%s' to %s", tool, remotePath)
	}

	result.MarkCompleted("Tool binaries installed successfully")
	return result, nil
}

func (s *InstallToolBinariesStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

func componentForTool(tool string) string {
	switch tool {
	case "jq":
		return binarybom.ComponentJq
	case "yq":
		return binarybom.ComponentYq
	default:
		return ""
	}
}

var _ step.Step = (*InstallToolBinariesStep)(nil)
