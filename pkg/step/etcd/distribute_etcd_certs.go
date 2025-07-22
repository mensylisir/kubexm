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

type DistributeEtcdCertsStep struct {
	step.Base
	LocalCertsDir  string
	RemoteCertsDir string
	CaCertFileName string
	PermissionDir  string
	PermissionFile string
}

type DistributeEtcdCertsStepBuilder struct {
	step.Builder[DistributeEtcdCertsStepBuilder, *DistributeEtcdCertsStep]
}

func NewDistributeEtcdCertsStepBuilder(ctx runtime.Context, instanceName string) *DistributeEtcdCertsStepBuilder {
	s := &DistributeEtcdCertsStep{
		LocalCertsDir:  filepath.Join(ctx.GetGlobalWorkDir(), "certs", "etcd"),
		RemoteCertsDir: common.DefaultEtcdPKIDir,
		CaCertFileName: common.EtcdCaPemFileName,
		PermissionDir:  "0755",
		PermissionFile: "0644",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute etcd certificates to all etcd nodes", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(DistributeEtcdCertsStepBuilder).Init(s)
	return b
}

func (b *DistributeEtcdCertsStepBuilder) WithLocalCertsDir(path string) *DistributeEtcdCertsStepBuilder {
	b.Step.LocalCertsDir = path
	return b
}

func (b *DistributeEtcdCertsStepBuilder) WithRemoteCertsDir(path string) *DistributeEtcdCertsStepBuilder {
	b.Step.RemoteCertsDir = path
	return b
}

func (b *DistributeEtcdCertsStepBuilder) WithCaCertFileName(name string) *DistributeEtcdCertsStepBuilder {
	b.Step.CaCertFileName = name
	return b
}

func (b *DistributeEtcdCertsStepBuilder) WithPermissionDir(permission string) *DistributeEtcdCertsStepBuilder {
	b.Step.PermissionDir = permission
	return b
}

func (b *DistributeEtcdCertsStepBuilder) WithPermissionFile(permission string) *DistributeEtcdCertsStepBuilder {
	b.Step.PermissionFile = permission
	return b
}

func (s *DistributeEtcdCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeEtcdCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")
	runner := ctx.GetRunner()

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("failed to get connector for precheck on host %s: %w", ctx.GetHost().GetName(), err)
	}

	nodeName := ctx.GetHost().GetName()
	requiredFiles := []string{
		s.CaCertFileName,
		fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName),
		fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName),
		fmt.Sprintf(common.EtcdNodeCertFileNamePattern, nodeName),
		fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, nodeName),
		fmt.Sprintf(common.EtcdMemberCertFileNamePattern, nodeName),
		fmt.Sprintf(common.EtcdMemberKeyFileNamePattern, nodeName),
	}

	for _, fileName := range requiredFiles {
		remotePath := filepath.Join(s.RemoteCertsDir, fileName)

		exists, err := runner.Exists(ctx.GoContext(), conn, remotePath)
		if err != nil {
			return false, fmt.Errorf("failed to check existence of remote file %s on node %s: %w", remotePath, nodeName, err)
		}

		if !exists {
			logger.Info("Required certificate file not found on remote host. Step needs to run.", "node", nodeName, "file", remotePath)
			return false, nil
		}
	}
	logger.Info("All required etcd certificates already exist on the remote host. Step is done.", "node", nodeName)
	return true, nil
}

func (s *DistributeEtcdCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	filesToCopy := []string{
		s.CaCertFileName,
		fmt.Sprintf(common.EtcdAdminCertFileNamePattern, ctx.GetHost().GetName()),
		fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, ctx.GetHost().GetName()),
		fmt.Sprintf(common.EtcdNodeCertFileNamePattern, ctx.GetHost().GetName()),
		fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, ctx.GetHost().GetName()),
		fmt.Sprintf(common.EtcdMemberCertFileNamePattern, ctx.GetHost().GetName()),
		fmt.Sprintf(common.EtcdMemberKeyFileNamePattern, ctx.GetHost().GetName()),
	}

	for _, fileName := range filesToCopy {
		localPath := filepath.Join(s.LocalCertsDir, fileName)
		remotePath := filepath.Join(s.RemoteCertsDir, fileName)

		logger.Info("Uploading file", "from", localPath, "to", fmt.Sprintf("%s:%s", ctx.GetHost().GetName(), remotePath))
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remotePath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload %s to %s on node %s: %w", fileName, remotePath, ctx.GetHost().GetName(), err)
		}

		permission := s.PermissionFile
		if isPrivateKey(fileName) {
			permission = "0600"
		}
		chmodCmd := fmt.Sprintf("chmod %s %s", permission, remotePath)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, chmodCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to set permission on %s for node %s: %w", remotePath, ctx.GetHost().GetName(), err)
		}
	}

	logger.Info("All etcd certificates have been distributed successfully.")
	return nil
}

func (s *DistributeEtcdCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback, cannot remove files: %v", err)
		return nil
	}

	rmCmd := fmt.Sprintf("rm -rf %s", s.RemoteCertsDir)
	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, rmCmd, s.Sudo); err != nil {
		logger.Error(err, "Failed to remove remote directory during rollback", "node", ctx.GetHost().GetName(), "dir", s.RemoteCertsDir)
	}
	return nil
}

func isPrivateKey(filename string) bool {
	return strings.HasSuffix(filename, "-key.pem") || strings.HasSuffix(filename, ".key")
}

var _ step.Step = (*DistributeEtcdCertsStep)(nil)
