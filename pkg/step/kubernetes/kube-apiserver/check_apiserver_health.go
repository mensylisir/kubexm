package kube_apiserver

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CheckAPIServerHealthStep struct {
	step.Base
	HealthCheckURL string
	RetryCount     int
	RetryDelay     time.Duration
	RemoteCertsDir string
}

type CheckAPIServerHealthStepBuilder struct {
	step.Builder[CheckAPIServerHealthStepBuilder, *CheckAPIServerHealthStep]
}

func NewCheckAPIServerHealthStepBuilder(ctx runtime.Context, instanceName string) *CheckAPIServerHealthStepBuilder {
	s := &CheckAPIServerHealthStep{
		RetryCount:     12,
		RetryDelay:     10 * time.Second,
		RemoteCertsDir: common.KubernetesPKIDir,
	}

	s.HealthCheckURL = fmt.Sprintf("https://127.0.0.1:%d/healthz", common.KubeAPIServerDefaultPort)

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Check kube-apiserver health on localhost", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(CheckAPIServerHealthStepBuilder).Init(s)
	return b
}

func (s *CheckAPIServerHealthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckAPIServerHealthStep) checkHealth(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	caCertPath := filepath.Join(s.RemoteCertsDir, common.CACertFileName)

	cmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' --cacert %s %s", caCertPath, s.HealthCheckURL)

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("health check command failed: %w, stderr: %s", err, stderr)
	}

	if stdout == "200" {
		return true, nil
	}

	return false, fmt.Errorf("health check failed with status code: %s", stdout)
}

func (s *CheckAPIServerHealthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	healthy, err := s.checkHealth(ctx)
	if err != nil {
		logger.Infof("Precheck: API server is not yet healthy. Step needs to run. (Error: %v)", err)
		return false, nil
	}

	if healthy {
		logger.Info("Precheck: API server is already healthy. Step is done.")
		return true, nil
	}

	return false, nil
}

func (s *CheckAPIServerHealthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	for i := 0; i < s.RetryCount; i++ {
		logger.Infof("Waiting for kube-apiserver to be healthy... (Attempt %d/%d)", i+1, s.RetryCount)

		healthy, err := s.checkHealth(ctx)
		if healthy {
			logger.Info("kube-apiserver is healthy!")
			return nil
		}

		if err != nil {
			logger.Debugf("Health check attempt failed: %v", err)
		}

		if i < s.RetryCount-1 {
			time.Sleep(s.RetryDelay)
		}
	}

	return fmt.Errorf("kube-apiserver did not become healthy after %d attempts", s.RetryCount)
}

func (s *CheckAPIServerHealthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Check health step has no rollback action.")
	return nil
}

var _ step.Step = (*CheckAPIServerHealthStep)(nil)
