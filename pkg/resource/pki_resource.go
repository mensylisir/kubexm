package resource

import (
	"fmt"
	"path/filepath"
	// "os" // For os.Stat if checking file existence directly in handle

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/pki" // Assuming cert generation steps will be in pkg/step/pki
	"github.com/mensylisir/kubexm/pkg/task"
)

// PKIResourceHandle manages the generation of a specific certificate or CA.
type PKIResourceHandle struct {
	IsCA            bool     // True if this handle is for generating a CA certificate
	CAName          string   // Logical name of the CA to use (if IsCA is false, and this cert is signed by it)
	CertName        string   // Logical name of the certificate to generate (e.g., "etcd-server", "kube-apiserver")
	CommonName      string   // Common Name (CN) for the certificate
	SANs            []string // Subject Alternative Names (IPs and DNS names)
	Organizations   []string // Organization (O) field
	OutputDirName   string   // Subdirectory name under the main certs dir (e.g., "etcd", "kubernetes")
	ValidityDays    int      // Certificate validity period in days
	// Other relevant fields like KeyUsage, ExtKeyUsage could be added if more control is needed.
}

// NewPKIResourceHandle creates a new handle for a PKI resource (certificate or CA).
func NewPKIResourceHandle(isCA bool, caName, certName, commonName string, sans, orgs []string, outputDirName string, validityDays int) Handle {
	return &PKIResourceHandle{
		IsCA:            isCA,
		CAName:          caName,
		CertName:        certName,
		CommonName:      commonName,
		SANs:            sans,
		Organizations:   orgs,
		OutputDirName:   outputDirName,
		ValidityDays:    validityDays,
	}
}

func (h *PKIResourceHandle) ID() string {
	if h.IsCA {
		return fmt.Sprintf("pki-ca-%s", h.CertName) // CertName is used as the CA's own name here
	}
	return fmt.Sprintf("pki-cert-%s-signedby-%s", h.CertName, h.CAName)
}

// Path returns the local path to the generated certificate file.
// Example: <GlobalWorkDir>/.kubexm/<ClusterName>/certs/<OutputDirName>/<CertName>.crt
func (h *PKIResourceHandle) Path(ctx task.TaskContext) (string, error) {
	certsBaseDir := ctx.GetCertsDir() // Should give <GlobalWorkDir>/.kubexm/<ClusterName>/certs
	if certsBaseDir == "" {
		return "", fmt.Errorf("certs base directory is not configured in context")
	}
	// Specific output directory for this cert type (e.g. "etcd", "kubernetes")
	specificCertsDir := filepath.Join(certsBaseDir, h.OutputDirName)

	// Cert filename, e.g. "etcd-server.crt"
	certFileName := fmt.Sprintf("%s.crt", h.CertName)
	return filepath.Join(specificCertsDir, certFileName), nil
}

func (h *PKIResourceHandle) KeyPath(ctx task.TaskContext) (string, error) {
	certsBaseDir := ctx.GetCertsDir()
	if certsBaseDir == "" {
		return "", fmt.Errorf("certs base directory is not configured in context")
	}
	specificCertsDir := filepath.Join(certsBaseDir, h.OutputDirName)
	keyFileName := fmt.Sprintf("%s.key", h.CertName)
	return filepath.Join(specificCertsDir, keyFileName), nil
}


func (h *PKIResourceHandle) Type() string {
	if h.IsCA {
		return "pki-ca"
	}
	return "pki-certificate"
}

// EnsurePlan generates an ExecutionFragment to create the certificate/CA if it doesn't exist.
// Steps are planned to run on the local control node.
func (h *PKIResourceHandle) EnsurePlan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("resource_id", h.ID())
	logger.Info("Planning resource assurance for PKI resource...")

	certPath, err := h.Path(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to determine cert path for %s: %w", h.ID(), err)
	}
	keyPath, err := h.KeyPath(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to determine key path for %s: %w", h.ID(), err)
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for PKI resource %s: %w", h.ID(), err)
	}

	runner := ctx.GetRunner()
	localConn, err := ctx.GetConnectorForHost(controlNode)
	if err != nil {
		return nil, fmt.Errorf("failed to get connector for control node: %w", err)
	}

	// Precheck: Check if both certificate and key files exist.
	// A more robust check would validate the certificate's CN, SANs, expiry, and if it's signed by the correct CA.
	certExists, _ := runner.Exists(ctx.GoContext(), localConn, certPath)
	keyExists, _ := runner.Exists(ctx.GoContext(), localConn, keyPath)

	if certExists && keyExists {
		// TODO: Add actual certificate validation here (CN, SANs, Expiry, Issuer for signed certs)
		logger.Info("Certificate and key files already exist. Assuming valid.", "cert", certPath, "key", keyPath)
		return &task.ExecutionFragment{Nodes: make(map[plan.NodeID]*plan.ExecutionNode)}, nil
	}
	logger.Info("Certificate or key missing. Planning generation.", "cert_exists", certExists, "key_exists", keyExists)


	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var genStep step.Step // To hold either GenerateCACertStep or GenerateSignedCertStep
	var nodeName string

	certsBaseDir := ctx.GetCertsDir()
	specificCertsDir := filepath.Join(certsBaseDir, h.OutputDirName)


	if h.IsCA {
		genStep = pki.NewGenerateCACertStep(
			h.CertName, // Used as instance name for the step
			h.CommonName,
			h.Organizations,
			h.ValidityDays,
			specificCertsDir, // Output directory for this CA's cert and key
		)
		nodeName = fmt.Sprintf("Generate CA Certificate %s", h.CertName)
	} else {
		// Path to the CA certificate and key that will sign this new certificate
		caCertPath := filepath.Join(certsBaseDir, h.CAName, fmt.Sprintf("%s.crt", h.CAName)) // Assuming CA cert is named after CAName
		caKeyPath := filepath.Join(certsBaseDir, h.CAName, fmt.Sprintf("%s.key", h.CAName))   // Assuming CA key is named after CAName

		// Before planning to generate a signed cert, we must ensure its CA exists.
		// This creates a dependency. The EnsurePlan for the CA itself should be called by the Task/Module.
		// Here, we assume the CA's EnsurePlan has already been processed or will be a dependency in the graph.

		genStep = pki.NewGenerateSignedCertStep(
			h.CertName, // Instance name
			h.CommonName,
			h.Organizations,
			h.SANs,
			h.ValidityDays,
			caCertPath,
			caKeyPath,
			specificCertsDir, // Output directory for this cert
			h.CertName,       // Base filename for the cert and key
		)
		nodeName = fmt.Sprintf("Generate Signed Certificate %s (signed by %s)", h.CertName, h.CAName)
	}

	nodeID := plan.NodeID(fmt.Sprintf("generate-%s", h.ID()))
	nodes[nodeID] = &plan.ExecutionNode{
		Name:     nodeName,
		Step:     genStep,
		Hosts:    []connector.Host{controlNode}, // PKI generation happens on the control node
		StepName: genStep.Meta().Name,
		// Dependencies: If !IsCA, this node might depend on the CA's generation node.
		// This inter-handle dependency needs to be managed at the Task/Module level when assembling the graph.
	}

	logger.Info("PKI resource assurance plan created.", "cert_path", certPath)
	return &task.ExecutionFragment{
		Nodes:      nodes,
		EntryNodes: []plan.NodeID{nodeID},
		ExitNodes:  []plan.NodeID{nodeID},
	}, nil
}

var _ Handle = (*PKIResourceHandle)(nil)
