package etcd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type WaitClusterHealthyStep struct {
	step.Base
	etcdNodes      []connector.Host
	remoteCertsDir string
	checkTimeout   time.Duration
	checkInterval  time.Duration
}

func NewWaitClusterHealthyStep(ctx runtime.Context, instanceName string) *WaitClusterHealthyStep {
	s := &WaitClusterHealthyStep{
		remoteCertsDir: DefaultRemoteEtcdCertsDir,
		checkTimeout:   2 * time.Minute,
		checkInterval:  5 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Wait for the etcd cluster to become healthy"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute
	return s
}

func (s *WaitClusterHealthyStep) WithCheckTimeout(timeout time.Duration) *WaitClusterHealthyStep {
	s.checkTimeout = timeout
	return s
}

func (s *WaitClusterHealthyStep) WithCheckInterval(interval time.Duration) *WaitClusterHealthyStep {
	s.checkInterval = interval
	return s
}

func (s *WaitClusterHealthyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *WaitClusterHealthyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Checking if etcd is running")
	s.etcdNodes = ctx.GetHostsByRole(common.RoleEtcd)
	if len(s.etcdNodes) == 0 {
		return false, fmt.Errorf("no etcd nodes found in context to check health")
	}
	return false, nil
}

func (s *WaitClusterHealthyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	var endpoints []string
	for _, node := range s.etcdNodes {
		endpoints = append(endpoints, fmt.Sprintf("https://%s:2379", node.GetAddress()))
	}
	endpointsStr := strings.Join(endpoints, ",")
	nodeName := ctx.GetHost().GetName()
	cmd := fmt.Sprintf("etcdctl endpoint health --cluster --endpoints=%s --cacert=%s --cert=%s --key=%s",
		endpointsStr,
		filepath.Join(s.remoteCertsDir, common.EtcdCaPemFileName),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName)),
		filepath.Join(s.remoteCertsDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName)),
	)

	logger.Infof("Waiting up to %v for etcd cluster to become healthy...", s.checkTimeout)

	timeout := time.After(s.checkTimeout)
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out after %v waiting for etcd cluster to be healthy", s.checkTimeout)
		case <-ticker.C:
			logger.Info("Checking etcd cluster health...")
			stdout, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
			if err != nil {
				logger.Warnf("Health check command failed: %v. Retrying in %v...", err, s.checkInterval)
				continue
			}

			output := string(stdout)
			logger.Debugf("Health check output:\n%s", output)

			lines := strings.Split(strings.TrimSpace(output), "\n")
			healthyEndpoints := 0
			for _, line := range lines {
				if strings.Contains(line, "is healthy") {
					healthyEndpoints++
				}
			}

			if healthyEndpoints == len(s.etcdNodes) {
				logger.Info("Etcd cluster is healthy.")
				return nil
			}

			logger.Infof("Cluster not fully healthy yet (%d/%d endpoints healthy). Retrying in %v...", healthyEndpoints, len(s.etcdNodes), s.checkInterval)
		}
	}
}

func (s *WaitClusterHealthyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rolling back etcd cluster health check step")
	return nil
}

var _ step.Step = (*WaitClusterHealthyStep)(nil)
