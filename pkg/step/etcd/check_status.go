package etcd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EtcdStatusCheckStep struct {
	step.Base
	EtcdctlBinaryPath string
}

type EtcdStatusCheckStepBuilder struct {
	step.Builder[EtcdStatusCheckStepBuilder, *EtcdStatusCheckStep]
}

func NewEtcdStatusCheckStepBuilder(ctx runtime.Context, instanceName string) *EtcdStatusCheckStepBuilder {
	s := &EtcdStatusCheckStep{
		EtcdctlBinaryPath: filepath.Join(common.DefaultBinDir, "etcdctl"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Get detailed status of the etcd cluster", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EtcdStatusCheckStepBuilder).Init(s)
	return b
}

func (b *EtcdStatusCheckStepBuilder) WithEtcdctlBinaryPath(path string) *EtcdStatusCheckStepBuilder {
	b.Step.EtcdctlBinaryPath = path
	return b
}

func (s *EtcdStatusCheckStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EtcdStatusCheckStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *EtcdStatusCheckStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	nodeName := ctx.GetHost().GetName()
	logger.Info("Getting etcd cluster status from node...", "node", nodeName)

	caPath, certPath, keyPath := getEtcdctlCertPaths(nodeName)

	memberListCmd := fmt.Sprintf("ETCDCTL_API=3 %s member list --cacert %s --cert %s --key %s -w table",
		s.EtcdctlBinaryPath,
		caPath,
		certPath,
		keyPath,
	)

	logger.Info("Fetching member list...")
	memberListOutput, stderr, err := runner.OriginRun(ctx.GoContext(), conn, memberListCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to get etcd member list: %w, stderr: %s", err, stderr)
	}

	logger.Info(fmt.Sprintf("Etcd Member List:\n%s", memberListOutput))

	endpointStatusCmd := fmt.Sprintf("ETCDCTL_API=3 %s endpoint status --cluster --cacert %s --cert %s --key %s -w table",
		s.EtcdctlBinaryPath,
		caPath,
		certPath,
		keyPath,
	)

	logger.Info("Fetching endpoint status...")
	endpointStatusOutput, stderr, err := runner.OriginRun(ctx.GoContext(), conn, endpointStatusCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to get etcd endpoint status: %w, stderr: %s", err, stderr)
	}

	logger.Info(fmt.Sprintf("Etcd Endpoint Status:\n%s", endpointStatusOutput))

	return nil
}

func (s *EtcdStatusCheckStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*EtcdStatusCheckStep)(nil)
