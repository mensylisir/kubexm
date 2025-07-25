package etcd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DefragmentEtcdStep struct {
	step.Base
	EtcdctlBinaryPath string
}

type DefragmentEtcdStepBuilder struct {
	step.Builder[DefragmentEtcdStepBuilder, *DefragmentEtcdStep]
}

func NewDefragmentEtcdStepBuilder(ctx runtime.Context, instanceName string) *DefragmentEtcdStepBuilder {
	s := &DefragmentEtcdStep{
		EtcdctlBinaryPath: filepath.Join(common.DefaultBinDir, "etcdctl"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Defragment etcd data on current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(DefragmentEtcdStepBuilder).Init(s)
	return b
}

func (b *DefragmentEtcdStepBuilder) WithEtcdctlBinaryPath(path string) *DefragmentEtcdStepBuilder {
	b.Step.EtcdctlBinaryPath = path
	return b
}

func (s *DefragmentEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DefragmentEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *DefragmentEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	nodeName := ctx.GetHost().GetName()
	logger.Info("Starting defragmentation on etcd member...", "node", nodeName)
	caPath, certPath, keyPath := getEtcdctlCertPaths(nodeName)
	endpoint := "https://127.0.0.1:2379"

	defragCmd := fmt.Sprintf("ETCDCTL_API=3 %s defrag --endpoints=%s --cacert=%s --cert=%s --key=%s",
		s.EtcdctlBinaryPath,
		endpoint,
		caPath,
		certPath,
		keyPath,
	)

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, defragCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to defragment etcd member %s: %w, stderr: %s", nodeName, err, stderr)
	}

	if !strings.Contains(stdout, "Finished defragmenting") {
		return fmt.Errorf("defragmentation command ran without error, but success message was not found. Output: %s", stdout)
	}

	logger.Info("Successfully defragmented etcd member.", "node", nodeName, "output", stdout)
	return nil
}

func (s *DefragmentEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Warn("Rollback for a defragment step is not applicable.")
	return nil
}

var _ step.Step = (*DefragmentEtcdStep)(nil)
