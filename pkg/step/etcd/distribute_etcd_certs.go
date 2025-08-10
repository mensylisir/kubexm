package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"os"
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
		LocalCertsDir:  ctx.GetEtcdCertsDir(),
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
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

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

	allDone := true
	for _, fileName := range requiredFiles {
		localPath := filepath.Join(s.LocalCertsDir, fileName)
		remotePath := filepath.Join(s.RemoteCertsDir, fileName)

		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return false, fmt.Errorf("local source certificate '%s' not found, ensure generation steps ran successfully", localPath)
		}

		isDone, err := helpers.CheckRemoteFileIntegrity(ctx, localPath, remotePath, s.Sudo)
		if err != nil {
			return false, fmt.Errorf("failed to check remote file integrity for %s: %w", remotePath, err)
		}

		if !isDone {
			logger.Infof("Remote certificate '%s' is missing or outdated. Distribution is required.", remotePath)
			allDone = false
			break
		}
	}

	if allDone {
		logger.Info("All required etcd certificates already exist and are up-to-date on the remote host. Step is done.", "node", nodeName)
	}

	return allDone, nil
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
		logger.Debugf("Setting permissions for %s to %s", remotePath, permission)
		if err := runner.Chmod(ctx.GoContext(), conn, remotePath, permission, s.Sudo); err != nil {
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
