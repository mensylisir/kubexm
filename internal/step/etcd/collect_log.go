package etcd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CollectEtcdLogsStep struct {
	step.Base
	ServiceName   string
	RemoteLogDir  string
	remoteLogPath string
	LogSince      string
}

type CollectEtcdLogsStepBuilder struct {
	step.Builder[CollectEtcdLogsStepBuilder, *CollectEtcdLogsStep]
}

func NewCollectEtcdLogsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CollectEtcdLogsStepBuilder {
	s := &CollectEtcdLogsStep{
		ServiceName:  "etcd.service",
		RemoteLogDir: filepath.Join(common.DefaultRemoteWorkDir, "logs"),
		LogSince:     "1 hour ago",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Collect etcd service logs on current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CollectEtcdLogsStepBuilder).Init(s)
	return b
}

func (b *CollectEtcdLogsStepBuilder) WithServiceName(name string) *CollectEtcdLogsStepBuilder {
	b.Step.ServiceName = name
	return b
}

func (b *CollectEtcdLogsStepBuilder) WithRemoteLogDir(path string) *CollectEtcdLogsStepBuilder {
	b.Step.RemoteLogDir = path
	return b
}

func (b *CollectEtcdLogsStepBuilder) WithLogSince(since string) *CollectEtcdLogsStepBuilder {
	b.Step.LogSince = since
	return b
}

func (s *CollectEtcdLogsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CollectEtcdLogsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *CollectEtcdLogsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "Failed to get connector")
		return result, err
	}

	nodeName := ctx.GetHost().GetName()
	logger.Info("Collecting etcd logs...", "node", nodeName, "since", s.LogSince)

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteLogDir, "0755", s.Sudo); err != nil {
		err = fmt.Errorf("failed to create remote log directory %s: %w", s.RemoteLogDir, err)
		result.MarkFailed(err, "Failed to create log directory")
		return result, err
	}

	logFileName := fmt.Sprintf("etcd-%s.log", nodeName)
	remoteLogPath := filepath.Join(s.RemoteLogDir, logFileName)
	collectCmd := fmt.Sprintf("journalctl -u %s --since '%s' --no-pager > %s",
		s.ServiceName,
		s.LogSince,
		remoteLogPath,
	)
	shellCmd := fmt.Sprintf("sh -c \"%s\"", collectCmd)

	logger.Info("Exporting logs to remote file", "path", remoteLogPath)
	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, shellCmd, s.Sudo); err != nil {
		err = fmt.Errorf("failed to collect etcd logs on node %s: %w, stderr: %s", nodeName, err, stderr)
		result.MarkFailed(err, "Failed to collect logs")
		return result, err
	}

	s.remoteLogPath = remoteLogPath

	logger.Info("Successfully collected etcd logs.", "node", nodeName, "path", remoteLogPath)
	result.MarkCompleted("Logs collected successfully")
	return result, nil
}

func (s *CollectEtcdLogsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback")
		return nil
	}
	logger.Warn("Rolling back by removing collected log file", "path", s.remoteLogPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.remoteLogPath, s.Sudo, false); err != nil {
		logger.Error(err, "Failed to remove collected log file during rollback")
	}

	return nil
}

var _ step.Step = (*CollectEtcdLogsStep)(nil)
