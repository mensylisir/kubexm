package pki

import (
	"crypto/x509"
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// GenerateKubernetesPkiTask orchestrates the generation of Kubernetes PKI.
type GenerateKubernetesPkiTask struct {
	task.BaseTask
}

// NewGenerateKubernetesPkiTask creates a new task for generating Kubernetes PKI.
func NewGenerateKubernetesPkiTask() task.Task {
	return &GenerateKubernetesPkiTask{
		BaseTask: task.NewBaseTask(
			"GenerateKubernetesPki",
			"Generates all necessary Kubernetes PKI (CA, component certificates).",
			[]string{common.ControlNodeRole},
			nil,
			false,
		),
	}
}

func (t *GenerateKubernetesPkiTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for PKI generation: %w", err)
	}

	k8sCertDir := ctx.GetKubernetesCertsDir()

	// Step 1: Generate Kubernetes CA
	caStep := commonstep.NewGenerateCAStepBuilder(ctx, "GenerateKubernetesCA").
		WithLocalCertsDir(k8sCertDir).
		WithCertFileName(common.KubeCaPemFileName).
		WithKeyFileName(common.KubeCaKeyFileName).
		Build()

	caNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  caStep.Meta().Name,
		Step:  caStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Define certificates to generate
	certsToGenerate := []struct {
		BaseName     string
		CommonName   string
		Organization []string
		SANs         []string // Static SANs
		Usages       []x509.ExtKeyUsage
	}{
		{
			BaseName:   "kube-apiserver",
			CommonName: "kube-apiserver",
			SANs:       []string{"kubernetes", "kubernetes.default", "kubernetes.default.svc", "kubernetes.default.svc.cluster.local", "127.0.0.1"},
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		},
		{
			BaseName:     "kube-controller-manager",
			CommonName:   "system:kube-controller-manager",
			Organization: []string{"system:kube-controller-manager"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		{
			BaseName:     "kube-scheduler",
			CommonName:   "system:kube-scheduler",
			Organization: []string{"system:kube-scheduler"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		{
			BaseName:     "admin",
			CommonName:   "kubernetes-admin",
			Organization: []string{"system:masters"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		{
			BaseName:   "front-proxy-ca",
			CommonName: "front-proxy-ca",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		{
			BaseName:   "front-proxy-client",
			CommonName: "front-proxy-client",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
	}

	var allCertNodeIDs []plan.NodeID
	for _, cert := range certsToGenerate {
		// Add dynamic SANs for apiserver
		if cert.BaseName == "kube-apiserver" {
			cert.SANs = append(cert.SANs, ctx.GetClusterConfig().Spec.ControlPlaneEndpoint.Domain)
			for _, master := range ctx.GetHostsByRoleUnsafe(common.RoleMaster) {
				cert.SANs = append(cert.SANs, master.GetName(), master.GetAddress())
			}
		}

		certStep := commonstep.NewGenerateCertsStepBuilder(ctx, fmt.Sprintf("GenerateCert-%s", cert.BaseName)).
			WithLocalCertsDir(k8sCertDir).
			WithCaCertFileName(common.KubeCaPemFileName).
			WithCaKeyFileName(common.KubeCaKeyFileName).
			WithCert(fmt.Sprintf("%s.pem", cert.BaseName)).
			WithCertKey(fmt.Sprintf("%s-key.pem", cert.BaseName)).
			WithCommonName(cert.CommonName).
			WithOrganization(cert.Organization).
			WithHosts(cert.SANs).
			WithUsages(cert.Usages).
			Build()

		certNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:         certStep.Meta().Name,
			Step:         certStep,
			Hosts:        []connector.Host{controlPlaneHost},
			Dependencies: []plan.NodeID{caNodeID},
		})
		allCertNodeIDs = append(allCertNodeIDs, certNodeID)
	}

	fragment.EntryNodes = []plan.NodeID{caNodeID}
	fragment.ExitNodes = allCertNodeIDs

	logger.Info("Kubernetes PKI generation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*GenerateKubernetesPkiTask)(nil)
