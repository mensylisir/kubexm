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

type SetupEtcdDirsStep struct {
	step.Base
	DataDir    string
	ConfDir    string
	CertsDir   string
	Permission string
}

type SetupEtcdDirsStepBuilder struct {
	step.Builder[SetupEtcdDirsStepBuilder, *SetupEtcdDirsStep]
}

func NewSetupEtcdDirsStepBuilder(ctx runtime.Context, instanceName string) *SetupEtcdDirsStepBuilder {
	s := &SetupEtcdDirsStep{
		DataDir:    common.EtcdDefaultDataDirTarget,
		CertsDir:   common.EtcdDefaultPKIDirTarget,
		ConfDir:    common.EtcdDefaultConfDirTarget,
		Permission: "0700",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Setup directories for etcd", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(SetupEtcdDirsStepBuilder).Init(s)
	return b
}

func (b *SetupEtcdDirsStepBuilder) WithDataDir(path string) *SetupEtcdDirsStepBuilder {
	b.Step.DataDir = path
	return b
}

func (b *SetupEtcdDirsStepBuilder) WithCertsDir(path string) *SetupEtcdDirsStepBuilder {
	b.Step.CertsDir = path
	return b
}

func (b *SetupEtcdDirsStepBuilder) WithConfDir(path string) *SetupEtcdDirsStepBuilder {
	b.Step.ConfDir = path
	return b
}

func (b *SetupEtcdDirsStepBuilder) WithPermission(permission string) *SetupEtcdDirsStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *SetupEtcdDirsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SetupEtcdDirsStep) dirsToSetup() []string {
	return []string{s.DataDir, s.CertsDir, s.ConfDir}
}

func (s *SetupEtcdDirsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	dirs := s.dirsToSetup()
	for _, dir := range dirs {
		exists, err := runner.Exists(ctx.GoContext(), conn, dir)
		if err != nil {
			return false, err
		}
		if !exists {
			logger.Infof("Directory '%s' does not exist. Setup is required.", dir)
			return false, nil
		}
	}

	logger.Info("All required etcd directories already exist. Step is done.")
	return true, nil
}

func (s *SetupEtcdDirsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	dirs := s.dirsToSetup()
	for _, dir := range dirs {
		logger.Infof("Ensuring directory exists: %s with permissions %s", dir, s.Permission)
		if err := runner.Mkdirp(ctx.GoContext(), conn, dir, s.Permission, s.Sudo); err != nil {
			return fmt.Errorf("failed to create directory '%s': %w", dir, err)
		}
		if err := runner.Chown(ctx.GoContext(), conn, dir, "root", "root", true); err != nil {
			logger.Warnf("Failed to set ownership on '%s', this might be okay. Error: %v", dir, err)
		}
	}

	return nil
}

func (s *SetupEtcdDirsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	dirs := s.dirsToSetup()
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		logger.Warnf("Rolling back by removing directory: %s", dir)
		if err := runner.Remove(ctx.GoContext(), conn, dir, s.Sudo, true); err != nil {
			logger.Errorf("Failed to remove '%s' during rollback: %v", dir, err)
		}
	}
	etcdBaseDir := common.EtcdDefaultConfDirTarget
	if s.ConfDir != "" {
		etcdBaseDir = filepath.Dir(s.ConfDir)
		if etcdBaseDir == "." {
			etcdBaseDir = s.ConfDir
		}
	}
	logger.Warnf("Rolling back by removing base directory: %s", etcdBaseDir)
	if err := runner.Remove(ctx.GoContext(), conn, etcdBaseDir, s.Sudo, true); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", etcdBaseDir, err)
	}

	return nil
}

var _ step.Step = (*SetupEtcdDirsStep)(nil)
