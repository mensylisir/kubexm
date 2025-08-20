package kubeadm

import (
	"context"
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeadmWaitClusterHealthyStep struct {
	step.Base
	checkTimeout  time.Duration
	checkInterval time.Duration
}

type KubeadmWaitClusterHealthyStepBuilder struct {
	step.Builder[KubeadmWaitClusterHealthyStepBuilder, *KubeadmWaitClusterHealthyStep]
}

func NewKubeadmWaitClusterHealthyStepBuilder(ctx runtime.Context, instanceName string) *KubeadmWaitClusterHealthyStepBuilder {
	s := &KubeadmWaitClusterHealthyStep{
		checkTimeout:  5 * time.Minute,
		checkInterval: 15 * time.Second,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Wait for the Kubernetes cluster to become healthy and stable"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = s.checkTimeout + 1*time.Minute

	b := new(KubeadmWaitClusterHealthyStepBuilder).Init(s)
	return b
}

func (b *KubeadmWaitClusterHealthyStepBuilder) WithCheckTimeout(timeout time.Duration) *KubeadmWaitClusterHealthyStepBuilder {
	b.Step.checkTimeout = timeout
	return b
}

func (b *KubeadmWaitClusterHealthyStepBuilder) WithCheckInterval(interval time.Duration) *KubeadmWaitClusterHealthyStepBuilder {
	b.Step.checkInterval = interval
	return b
}

func (s *KubeadmWaitClusterHealthyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmWaitClusterHealthyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for waiting on cluster health...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v kubectl && [ -f /etc/kubernetes/admin.conf ]", s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: 'kubectl' or '/etc/kubernetes/admin.conf' not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed.")
	return false, nil
}

func (s *KubeadmWaitClusterHealthyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	logger.Infof("Waiting up to %v for Kubernetes cluster to become healthy...", s.checkTimeout)

	timeout := time.After(s.checkTimeout)
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out after %v waiting for Kubernetes cluster to be healthy. Last known error: %w", s.checkTimeout, lastErr)
		case <-ticker.C:
			logger.Info("Checking Kubernetes cluster health...")

			err := s.checkClusterHealthWithClientGo(ctx)

			if err == nil {
				logger.Info("Kubernetes cluster is healthy.")
				return nil
			}

			lastErr = err
			logger.Warnf("Cluster not yet healthy: %v. Retrying in %v...", err, s.checkInterval)
		}
	}
}

func (s *KubeadmWaitClusterHealthyStep) checkClusterHealthWithClientGo(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	kubeconfigData, err := runner.ReadFile(ctx.GoContext(), conn, "/etc/kubernetes/admin.conf")
	if err != nil {
		return fmt.Errorf("failed to read remote admin.conf: %w", err)
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return fmt.Errorf("failed to build rest config from kubeconfig: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	allNodes := ctx.GetHostsByRole(common.RoleMaster)
	allNodes = append(allNodes, ctx.GetHostsByRole(common.RoleWorker)...)
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}
	if len(nodes.Items) != len(allNodes) {
		return fmt.Errorf("expected %d nodes, but found %d", len(allNodes), len(nodes.Items))
	}
	for _, node := range nodes.Items {
		isReady := false
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}
		if !isReady {
			return fmt.Errorf("node '%s' is not in Ready state", node.Name)
		}
	}

	pods, err := clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods in kube-system: %w", err)
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			if pod.DeletionTimestamp != nil {
				continue
			}
			return fmt.Errorf("pod '%s' in kube-system is in phase '%s'", pod.Name, pod.Status.Phase)
		}
	}

	return nil
}

func (s *KubeadmWaitClusterHealthyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a wait/verification-only step.")
	return nil
}

var _ step.Step = (*KubeadmWaitClusterHealthyStep)(nil)
