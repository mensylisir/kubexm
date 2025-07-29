package harbor

import (
	"crypto/sha256"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type DistributeHarborCACertStep struct {
	step.Base
	LocalCACertPath string
	HarborDomain    string
}

type DistributeHarborCACertStepBuilder struct {
	step.Builder[DistributeHarborCACertStepBuilder, *DistributeHarborCACertStep]
}

func NewDistributeHarborCACertStepBuilder(ctx runtime.Context, instanceName string) *DistributeHarborCACertStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	cfg := ctx.GetClusterConfig().Spec
	if cfg.Registry == nil || cfg.Registry.MirroringAndRewriting == nil || cfg.Registry.MirroringAndRewriting.PrivateRegistry == "" {
		return nil
	}

	domain := cfg.Registry.MirroringAndRewriting.PrivateRegistry
	if u, err := url.Parse("scheme://" + domain); err == nil {
		domain = u.Host
	}

	s := &DistributeHarborCACertStep{
		LocalCACertPath: filepath.Join(ctx.GetCertsDir(), "harbor", "ca.crt"),
		HarborDomain:    domain,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute Harbor CA certificate to all nodes", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(DistributeHarborCACertStepBuilder).Init(s)
	return b
}

func (s *DistributeHarborCACertStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DistributeHarborCACertStep) getRemoteCertPaths(ctx runtime.ExecutionContext) []string {
	return []string{
		filepath.Join("/etc/containerd/certs.d", s.HarborDomain, "ca.crt"),
		filepath.Join("/etc/docker/certs.d", s.HarborDomain, "ca.crt"),
	}
}

func (s *DistributeHarborCACertStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	localContent, err := os.ReadFile(s.LocalCACertPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("local Harbor CA certificate '%s' not found, ensure generate step ran successfully", s.LocalCACertPath)
		}
		return false, fmt.Errorf("failed to read local CA certificate: %w", err)
	}
	localChecksum := fmt.Sprintf("%x", sha256.Sum256(localContent))

	for _, remotePath := range s.getRemoteCertPaths(ctx) {
		exists, err := runner.Exists(ctx.GoContext(), conn, remotePath)
		if err != nil {
			continue
		}

		if exists {
			remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remotePath)
			if err != nil {
				logger.Warnf("Remote CA cert '%s' exists but failed to read, will overwrite.", remotePath)
				return false, nil
			}

			remoteChecksum := fmt.Sprintf("%x", sha256.Sum256(remoteContent))
			if localChecksum == remoteChecksum {
				logger.Infof("Harbor CA certificate on remote host '%s' is up to date.", remotePath)
				return true, nil
			}
		}
	}

	logger.Info("Harbor CA certificate not found or outdated on remote host. Distribution is required.")
	return false, nil
}

func (s *DistributeHarborCACertStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	localContent, err := os.ReadFile(s.LocalCACertPath)
	if err != nil {
		return fmt.Errorf("failed to read local Harbor CA certificate '%s': %w", s.LocalCACertPath, err)
	}

	runtimeType := ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type

	var targetPaths []string
	if runtimeType == common.RuntimeTypeContainerd {
		targetPaths = []string{filepath.Join("/etc/containerd/certs.d", s.HarborDomain)}
	} else if runtimeType == common.RuntimeTypeDocker {
		targetPaths = []string{filepath.Join("/etc/docker/certs.d", s.HarborDomain)}
	} else {
		targetPaths = []string{
			filepath.Join("/etc/containerd/certs.d", s.HarborDomain),
			filepath.Join("/etc/docker/certs.d", s.HarborDomain),
		}
	}

	for _, targetDir := range targetPaths {
		logger.Infof("Ensuring remote directory exists: %s", targetDir)
		if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to create remote certs directory '%s': %w", targetDir, err)
		}

		remoteCertPath := filepath.Join(targetDir, "ca.crt")
		logger.Infof("Uploading Harbor CA certificate to %s", remoteCertPath)
		if err := runner.WriteFile(ctx.GoContext(), conn, localContent, remoteCertPath, "0644", s.Sudo); err != nil {
			return fmt.Errorf("failed to write remote CA certificate to '%s': %w", remoteCertPath, err)
		}
	}

	logger.Info("Successfully distributed Harbor CA certificate to the node.")
	return nil
}

func (s *DistributeHarborCACertStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	for _, remotePath := range s.getRemoteCertPaths(ctx) {
		certDir := filepath.Dir(remotePath)
		logger.Warnf("Rolling back by removing directory: %s", certDir)
		if err := runner.Remove(ctx.GoContext(), conn, certDir, s.Sudo, true); err != nil {
			logger.Errorf("Failed to remove directory '%s' during rollback: %v", certDir, err)
		}
	}

	return nil
}

var _ step.Step = (*DistributeHarborCACertStep)(nil)
