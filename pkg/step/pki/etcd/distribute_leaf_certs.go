package etcd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type fileToDistribute struct {
	localPath  string
	remotePath string
	perms      string
}

type DistributeLeafCertsStep struct {
	step.Base
	files []fileToDistribute
}

type DistributeLeafCertsStepBuilder struct {
	step.Builder[DistributeLeafCertsStepBuilder, *DistributeLeafCertsStep]
}

func NewDistributeLeafCertsStepBuilder(ctx runtime.Context, instanceName string) *DistributeLeafCertsStepBuilder {
	localNewCertsDir := filepath.Join(ctx.GetEtcdCertsDir())

	files := []fileToDistribute{
		{
			localPath:  filepath.Join(localNewCertsDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, ctx.GetHost().GetName())),
			remotePath: filepath.Join(DefaultRemoteEtcdCertsDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, ctx.GetHost().GetName())),
			perms:      "0644",
		},
		{
			localPath:  filepath.Join(localNewCertsDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, ctx.GetHost().GetName())),
			remotePath: filepath.Join(DefaultRemoteEtcdCertsDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, ctx.GetHost().GetName())),
			perms:      "0600",
		},
		{
			localPath:  filepath.Join(localNewCertsDir, fmt.Sprintf(common.EtcdNodeCertFileNamePattern, ctx.GetHost().GetName())),
			remotePath: filepath.Join(DefaultRemoteEtcdCertsDir, fmt.Sprintf(common.EtcdNodeCertFileNamePattern, ctx.GetHost().GetName())),
			perms:      "0644",
		},
		{
			localPath:  filepath.Join(localNewCertsDir, fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, ctx.GetHost().GetName())),
			remotePath: filepath.Join(DefaultRemoteEtcdCertsDir, fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, ctx.GetHost().GetName())),
			perms:      "0600",
		},
		{
			localPath:  filepath.Join(localNewCertsDir, fmt.Sprintf(common.EtcdMemberCertFileNamePattern, ctx.GetHost().GetName())),
			remotePath: filepath.Join(DefaultRemoteEtcdCertsDir, fmt.Sprintf(common.EtcdMemberCertFileNamePattern, ctx.GetHost().GetName())),
			perms:      "0644",
		},
		{
			localPath:  filepath.Join(localNewCertsDir, fmt.Sprintf(common.EtcdMemberKeyFileNamePattern, ctx.GetHost().GetName())),
			remotePath: filepath.Join(DefaultRemoteEtcdCertsDir, fmt.Sprintf(common.EtcdMemberKeyFileNamePattern, ctx.GetHost().GetName())),
			perms:      "0600",
		},
	}

	s := &DistributeLeafCertsStep{
		files: files,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("Distribute new leaf certificates to the etcd node %s", ctx.GetHost().GetName())
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute
	b := new(DistributeLeafCertsStepBuilder).Init(s)
	return b
}

func (s *DistributeLeafCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeLeafCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("checking if leaf certificates exist")
	for _, file := range s.files {
		if !helpers.IsFileExist(file.localPath) {
			logger.Warnf("Local source certificate file '%s' not found. Ensure the preparation step ran successfully", file.localPath)
			return false, fmt.Errorf("local source certificate file '%s' not found. Ensure the generation step ran successfully", file.localPath)
		}
	}
	return false, nil
}

func (s *DistributeLeafCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Distributing new leaf certificates...")
	for _, file := range s.files {
		logger.Debugf("Distributing %s to %s", filepath.Base(file.localPath), file.remotePath)

		content, err := os.ReadFile(file.localPath)
		if err != nil {
			return fmt.Errorf("failed to read local file '%s': %w", file.localPath, err)
		}

		if err := helpers.WriteContentToRemote(ctx, conn, string(content), file.remotePath, file.perms, s.Sudo); err != nil {
			return fmt.Errorf("failed to write file to remote path '%s': %w", file.remotePath, err)
		}
	}

	logger.Info("All new leaf certificates distributed successfully.")
	return nil
}

func (s *DistributeLeafCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*DistributeLeafCertsStep)(nil)
