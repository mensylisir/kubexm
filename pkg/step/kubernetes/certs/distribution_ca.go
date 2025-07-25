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

type DistributeKubeCACertsStep struct {
	step.Base
	LocalCertsDir  string
	RemoteCertsDir string
}

type DistributeKubeCACertsStepBuilder struct {
	step.Builder[DistributeKubeCACertsStepBuilder, *DistributeKubeCACertsStep]
}

func NewDistributeKubeCACertsStepBuilder(ctx runtime.Context, instanceName string) *DistributeKubeCACertsStepBuilder {
	s := &DistributeKubeCACertsStep{
		LocalCertsDir:  filepath.Join(ctx.GetGlobalWorkDir(), "certs", "kubernetes"),
		RemoteCertsDir: common.KubernetesPKIDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute Kubernetes CA certificates to the node", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(DistributeKubeCACertsStepBuilder).Init(s)
	return b
}

func (s *DistributeKubeCACertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeKubeCACertsStep) filesToDistribute() []string {
	return []string{
		common.CACertFileName,
		common.FrontProxyCACertFileName,
	}
}

func (s *DistributeKubeCACertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
			return false, fmt.Errorf("failed to check for CA file '%s' on host %s: %w", remotePath, ctx.GetHost().GetName(), err)
		}
		if !exists {
			logger.Infof("Required CA file '%s' not found on remote host. Distribution is required.", remotePath)
			return false, nil
		}
	}

	logger.Info("All required Kubernetes CA certificates already exist on the remote host. Step is done.")
	return true, nil
}

func (s *DistributeKubeCACertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteCertsDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote PKI directory '%s' on host %s: %w", s.RemoteCertsDir, ctx.GetHost().GetName(), err)
	}

	for _, fileName := range s.filesToDistribute() {
		localPath := filepath.Join(s.LocalCertsDir, fileName)
		remotePath := filepath.Join(s.RemoteCertsDir, fileName)

		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return fmt.Errorf("local source CA file '%s' not found, ensure CA generation step ran successfully", localPath)
		}

		logger.Infof("Uploading %s to %s:%s", localPath, ctx.GetHost().GetName(), remotePath)
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remotePath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload CA file '%s' to host %s: %w", fileName, ctx.GetHost().GetName(), err)
		}

		logger.Infof("Setting permissions for %s to 0644", remotePath)
		if err := runner.Chmod(ctx.GoContext(), conn, remotePath, "0644", s.Sudo); err != nil {
			return fmt.Errorf("failed to set permission on CA file '%s' on host %s: %w", remotePath, ctx.GetHost().GetName(), err)
		}
	}

	logger.Info("Kubernetes CA certificates have been distributed successfully.")
	return nil
}

func (s *DistributeKubeCACertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback, cannot remove files: %v", err)
		return nil
	}

	for _, fileName := range s.filesToDistribute() {
		remotePath := filepath.Join(s.RemoteCertsDir, fileName)
		logger.Warnf("Rolling back by removing remote CA file: %s", remotePath)
		if err := runner.Remove(ctx.GoContext(), conn, remotePath, s.Sudo, false); err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				logger.Errorf("Failed to remove remote CA file '%s' during rollback: %v", remotePath, err)
			}
		}
	}

	return nil
}

var _ step.Step = (*DistributeKubeCACertsStep)(nil)
