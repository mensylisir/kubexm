package certs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DistributeKubeCertsStep struct {
	step.Base
	LocalCertsDir  string
	RemoteCertsDir string
	PermissionDir  string
	PermissionFile string
}

type DistributeKubeCertsStepBuilder struct {
	step.Builder[DistributeKubeCertsStepBuilder, *DistributeKubeCertsStep]
}

func NewDistributeKubeCertsStepBuilder(ctx runtime.Context, instanceName string) *DistributeKubeCertsStepBuilder {
	s := &DistributeKubeCertsStep{
		LocalCertsDir:  ctx.GetKubernetesCertsDir(),
		RemoteCertsDir: common.KubernetesPKIDir,
		PermissionDir:  "0755",
		PermissionFile: "0644",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute Kubernetes certificates to the node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(DistributeKubeCertsStepBuilder).Init(s)
	return b
}

func (s *DistributeKubeCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeKubeCertsStep) filesToDistribute() []string {
	return []string{
		common.CACertFileName,
		common.CAKeyFileName,
		common.FrontProxyCACertFileName,
		common.FrontProxyCAKeyFileName,
		common.ServiceAccountPublicKeyFileName,
		common.ServiceAccountPrivateKeyFileName,
		common.APIServerCertFileName,
		common.APIServerKeyFileName,
		common.APIServerKubeletClientCertFileName,
		common.APIServerKubeletClientKeyFileName,
		common.FrontProxyClientCertFileName,
		common.FrontProxyClientKeyFileName,
		common.AdminCertFileName,
		common.AdminKeyFileName,
		common.ControllerManagerCertFileName,
		common.ControllerManagerKeyFileName,
		common.SchedulerCertFileName,
		common.SchedulerKeyFileName,
	}
}

func (s *DistributeKubeCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	for _, fileName := range s.filesToDistribute() {
		remotePath := filepath.Join(s.RemoteCertsDir, fileName)
		exists, err := runner.Exists(ctx.GoContext(), conn, remotePath)
		if err != nil {
			return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", remotePath, ctx.GetHost().GetName(), err)
		}
		if !exists {
			logger.Infof("Required certificate file '%s' not found on remote host. Distribution is required.", remotePath)
			return false, nil
		}
	}

	logger.Info("All required Kubernetes certificates already exist on the remote host. Step is done.")
	return true, nil
}

func (s *DistributeKubeCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteCertsDir, s.PermissionDir, s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote PKI directory '%s' on host %s: %w", s.RemoteCertsDir, ctx.GetHost().GetName(), err)
	}

	for _, fileName := range s.filesToDistribute() {
		localPath := filepath.Join(s.LocalCertsDir, fileName)
		remotePath := filepath.Join(s.RemoteCertsDir, fileName)

		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return fmt.Errorf("local source certificate file '%s' not found, ensure previous generation steps ran successfully", localPath)
		}

		logger.Infof("Uploading %s to %s:%s", localPath, ctx.GetHost().GetName(), remotePath)
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remotePath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s' to host %s: %w", fileName, ctx.GetHost().GetName(), err)
		}

		permission := s.PermissionFile
		if isPrivateKey(fileName) {
			permission = "0600"
		}

		logger.Infof("Setting permissions for %s to %s", remotePath, permission)
		if err := runner.Chmod(ctx.GoContext(), conn, remotePath, permission, s.Sudo); err != nil {
			return fmt.Errorf("failed to set permission '%s' on file '%s' on host %s: %w", permission, remotePath, ctx.GetHost().GetName(), err)
		}
	}

	logger.Info("All Kubernetes certificates have been distributed successfully to the current host.")
	return nil
}

func (s *DistributeKubeCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback, cannot remove files: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing remote directory: %s", s.RemoteCertsDir)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteCertsDir, s.Sudo, true); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Errorf("Failed to remove remote directory '%s' during rollback: %v", s.RemoteCertsDir, err)
		}
	}
	return nil
}

func isPrivateKey(filename string) bool {
	return strings.HasSuffix(filename, ".key")
}

var _ step.Step = (*DistributeKubeCertsStep)(nil)
