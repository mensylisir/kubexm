package kubexm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/types"
)

const DefaultKubxmPKIDir = common.KubernetesPKIDir

type KubxmFetchPKIStep struct {
	step.Base
	remotePKIDir  string
	localCertsDir string
	filesToFetch  map[string]string
}

type KubxmFetchPKIStepBuilder struct {
	step.Builder[KubxmFetchPKIStepBuilder, *KubxmFetchPKIStep]
}

func NewKubxmFetchPKIStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubxmFetchPKIStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()

	s := &KubxmFetchPKIStep{
		remotePKIDir:  DefaultKubxmPKIDir,
		localCertsDir: localCertsDir,
		filesToFetch: map[string]string{
			"ca.crt":             common.CACertFileName,
			"ca.key":             common.CAKeyFileName,
			"front-proxy-ca.crt": common.FrontProxyCACertFileName,
			"front-proxy-ca.key": common.FrontProxyCAKeyFileName,
			"sa.pub":             common.ServiceAccountPublicKeyFileName,
			"sa.key":             common.ServiceAccountPrivateKeyFileName,
		},
	}
	if ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm) {
		s.filesToFetch["etcd/ca.crt"] = common.CACertFileName
		s.filesToFetch["etcd/ca.key"] = common.CAKeyFileName
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Fetch essential PKI files from the primary master node to the local workspace"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubxmFetchPKIStepBuilder).Init(s)
	return b
}

func (s *KubxmFetchPKIStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubxmFetchPKIStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if Kubernetes PKI files need to be fetched...")

	for index, _ := range s.filesToFetch {
		localFullPath := filepath.Join(s.localCertsDir, index)
		if !helpers.IsFileExist(localFullPath) {
			logger.Infof("Local PKI file '%s' is missing. Fetch is required.", localFullPath)
			return false, nil
		}
	}

	logger.Info("All required Kubernetes PKI files already exist locally. Step is done.")
	return true, nil
}

func (s *KubxmFetchPKIStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get host connector")
		return result, err
	}

	isStackedEtcd := ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm)

	for remoteRelativePath, _ := range s.filesToFetch {
		remoteFullPath := filepath.Join(s.remotePKIDir, remoteRelativePath)
		localFullPath := filepath.Join(s.localCertsDir, remoteRelativePath)

		localParentDir := filepath.Dir(localFullPath)
		if err := os.MkdirAll(localParentDir, 0755); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to create local directory '%s'", localParentDir))
			return result, err
		}

		if !helpers.IsFileExist(localFullPath) {
			logger.Infof("Fetching remote:%s to local:%s", remoteFullPath, localFullPath)

			exists, err := runner.Exists(ctx.GoContext(), conn, remoteFullPath)
			if err != nil {
				result.MarkFailed(err, fmt.Sprintf("failed to check for remote file '%s'", remoteFullPath))
				return result, err
			}
			if !exists {
				isCritical := false
				switch remoteRelativePath {
				case "ca.crt", "ca.key":
					isCritical = true
				case "etcd/ca.crt", "etcd/ca.key":
					if isStackedEtcd {
						isCritical = true
					}
				}

				if isCritical {
					result.MarkFailed(fmt.Errorf("critical file not found"), fmt.Sprintf("critical remote file '%s' not found on host %s", remoteFullPath, ctx.GetHost().GetName()))
					return result, err
				}
				logger.Warnf("Optional remote file '%s' not found, skipping.", remoteFullPath)
				continue
			}

			if err := runner.Fetch(ctx.GoContext(), conn, remoteFullPath, localFullPath, s.Sudo); err != nil {
				result.MarkFailed(err, fmt.Sprintf("failed to fetch file '%s'", remoteFullPath))
				return result, err
			}
			logger.Debugf("Successfully fetched %s.", remoteRelativePath)
		}
	}

	logger.Info("All required PKI files have been fetched successfully.")
	result.MarkCompleted("PKI files fetched successfully")
	return result, nil
}

func (s *KubxmFetchPKIStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by deleting fetched PKI files from local workspace...")

	for index, _ := range s.filesToFetch {
		localFullPath := filepath.Join(s.localCertsDir, index)
		if err := os.Remove(localFullPath); err != nil && !os.IsNotExist(err) {
			logger.Errorf("Failed to remove fetched file '%s' during rollback: %v", localFullPath, err)
		}
	}

	return nil
}

var _ step.Step = (*KubxmFetchPKIStep)(nil)
