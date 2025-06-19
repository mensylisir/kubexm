package pki

import (
	"crypto/x509"
	"fmt"
	"net"
	"strings"

	certutil "k8s.io/client-go/util/cert"
	// netutils "k8s.io/utils/net" // Not directly used in this file's definitions, but GenerateEtcdAltNamesStep uses it.
)

// === KubexmsKubeConf Stub ===

// KubexmsKubeConf is a stub for the original common.KubeConf or config.Cluster object.
// It holds configuration relevant to PKI generation and other operations.
type KubexmsKubeConf struct {
	AppFSBaseDir             string                // Base path for app's persistent data, e.g., <executable_dir>/.kubexm
	ClusterName              string                // Name of the cluster
	PKIDirectory             string                // Base directory for this cluster's PKI (e.g., AppFSBaseDir/pki/clusterName)
	ControlPlaneEndpointDomain string                // FQDN for the control plane load balancer/VIP
	DefaultLBDomain            string                // Fallback if ControlPlaneEndpointDomain is empty (e.g., "lb.kubexms.local")
	HostsForAltNames           []HostSpecForAltNames // Simplified host info for AltNames (used by GenerateEtcdAltNamesStep)

	// Fields for External Etcd
	ExternalEtcd struct {
		CAFile   string
		CertFile string
		KeyFile  string
	}
	// Add any other fields that actual certs.GenerateCA or certs.GenerateCerts might conceptually need.
}

// === Certificate Stub Types ===

// KubexmsCertUtilConfig is a stub for certutil.Config, using actual certutil.AltNames.
type KubexmsCertUtilConfig struct {
	CommonName   string
	Organization []string
	AltNames     certutil.AltNames // Uses the actual k8s type
	Usages       []x509.ExtKeyUsage
}

// KubexmsCertConfig is a stub for the original certs.CertConfig.
type KubexmsCertConfig struct {
	Config KubexmsCertUtilConfig
}

// KubexmsCert is a stub for the original certs.KubekeyCert.
type KubexmsCert struct {
	Name     string            // Internal name for lookup/reference (e.g., "etcd-ca", "etcd-admin")
	LongName string            // A more descriptive name for logging/documentation
	BaseName string            // Used for filename generation (e.g., "ca-etcd" -> ca-etcd.pem, ca-etcd-key.pem)
	CAName   string            // Name of the CA KubexmsCert definition that should sign this (if not a CA itself)
	Config   KubexmsCertConfig // Certificate configuration details
	IsCA     bool              // True if this definition is for a CA certificate
	Cert     *x509.Certificate // Stores the parsed *x509.Certificate after generation/loading
	Key      interface{}       // Stores the crypto.PrivateKey after generation/loading
}

// === Host Information Stubs ===

// HostSpecForAltNames provides simplified host info for generating AltNames.
// Used by GenerateEtcdAltNamesStepSpec.
type HostSpecForAltNames struct {
	Name            string // Hostname or FQDN
	InternalAddress string // Comma-separated IP addresses
}

// HostSpecForPKI provides simplified host info for determining certificate roles.
// Used by GenerateEtcdNodeCertsStepSpec.
type HostSpecForPKI struct {
	Name  string   // Hostname
	Roles []string // List of roles, e.g., "etcd", "master"
}


// === Certificate Definition Functions ===

// DefineEtcdCACertForKubexms returns the definition for the etcd CA certificate.
func DefineEtcdCACertForKubexms() *KubexmsCert {
	return &KubexmsCert{
		Name:     "etcd-ca",
		LongName: "Kubexms self-signed CA to provision identities for etcd",
		BaseName: "ca-etcd", // Results in ca-etcd.pem, ca-etcd-key.pem
		IsCA:     true,      // This is a CA certificate
		Config: KubexmsCertConfig{
			Config: KubexmsCertUtilConfig{
				CommonName: "etcd-ca@kubexms", // CN for the CA itself
				// Organization for CAs is often the overarching org name.
				Organization: []string{"kubexms"},
				// Usages for a CA certificate typically include KeyCertSign and CRLSign.
				// These are usually set by the CA generation logic itself based on IsCA=true.
			},
		},
	}
}

// DefineEtcdAdminCertForKubexms returns a definition for an etcd client certificate
// typically used by components like kube-apiserver to securely communicate with etcd.
// It might also be used for administrative CLI access via etcdctl.
// The 'hostname' parameter here might be for a specific node (e.g. a master node)
// or a generic name if it's a shared admin client cert.
// For this stub, we'll assume it can be specific if hostname is unique, or generic like "etcd-admin".
func DefineEtcdAdminCertForKubexms(hostname string, altNames *certutil.AltNames) *KubexmsCert {
	cn := "etcd-admin@kubexms" // Generic admin CN
	base := "admin-etcd"
	if hostname != "" && hostname != "localhost" { // Allow per-host admin certs if needed
		cn = fmt.Sprintf("etcd-admin-%s@kubexms", hostname)
		base = fmt.Sprintf("admin-etcd-%s", hostname)
	}

	var an certutil.AltNames
	if altNames != nil {
		an = *altNames
	}

	return &KubexmsCert{
		Name:     "etcd-admin", // Use a consistent internal name for this type of cert
		LongName: fmt.Sprintf("Kubexms certificate for etcd administration client (%s)", hostname),
		BaseName: base,
		CAName:   "etcd-ca", // Signed by the "etcd-ca" (referencing KubexmsCert.Name)
		Config: KubexmsCertConfig{
			Config: KubexmsCertUtilConfig{
				CommonName:   cn,
				Organization: []string{"system:masters", "kubexms"}, // system:masters has high privileges
				Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
				AltNames:     an, // Include relevant SANs if this client needs to be identified by them
			},
		},
	}
}

// DefineEtcdMemberCertForKubexms returns the definition for an etcd member certificate.
// This certificate is used for both server authentication (listening for client/peer requests)
// and client authentication (when this member talks to other peers).
// 'hostname' should be the specific FQDN or resolvable name of the etcd member.
func DefineEtcdMemberCertForKubexms(hostname string, altNames *certutil.AltNames) *KubexmsCert {
	if hostname == "" {
		// Hostname is critical for member certs.
		// Fallback or error handling might be needed if it's empty.
		hostname = "unknown-etcd-member"
	}
	var an certutil.AltNames
	if altNames != nil {
		an = *altNames
	}

	return &KubexmsCert{
		Name:     fmt.Sprintf("etcd-member-%s", hostname), // Unique internal name per member
		LongName: fmt.Sprintf("Kubexms certificate for etcd member %s", hostname),
		BaseName: fmt.Sprintf("member-etcd-%s", hostname), // Filename base
		CAName:   "etcd-ca",
		Config: KubexmsCertConfig{
			Config: KubexmsCertUtilConfig{
				CommonName:   fmt.Sprintf("%s@kubexms-etcd-peer", hostname), // Specific CN for the member
				Organization: []string{"system:etcd-peers", "kubexms"},
				Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
				AltNames:     an, // Must include all IPs and DNS names this member is reachable at by clients and peers
			},
		},
	}
}

// DefineEtcdClientCertForKubexms returns a definition for a generic etcd client certificate.
// This could be used by various components that need to query etcd but are not administrators or peers.
// 'hostname' here might represent the client component's identity or the node it runs on.
func DefineEtcdClientCertForKubexms(hostname string, altNames *certutil.AltNames) *KubexmsCert {
	cn := "etcd-client@kubexms" // Generic client CN
	base := "client-etcd"
	if hostname != "" && hostname != "localhost" { // Allow per-host/per-client specification
		cn = fmt.Sprintf("etcd-client-%s@kubexms", hostname)
		base = fmt.Sprintf("client-etcd-%s", hostname)
	}

	var an certutil.AltNames
	if altNames != nil {
		an = *altNames
	}

	return &KubexmsCert{
		Name:     "etcd-client", // Use a consistent internal name for this type of generic client cert
		LongName: fmt.Sprintf("Kubexms certificate for generic etcd client (%s)", hostname),
		BaseName: base,
		CAName:   "etcd-ca",
		Config: KubexmsCertConfig{
			Config: KubexmsCertUtilConfig{
				CommonName:   cn,
				Organization: []string{"system:etcd-clients", "kubexms"}, // A less privileged group
				Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
				AltNames:     an, // Usually not strictly needed for pure client certs unless identifying by SAN
			},
		},
	}
}

// Note: The actual certs.GenerateCA and certs.GenerateCerts functions from Kubekey
// would take these *KubexmsCert definitions (as their *certs.KubekeyCert equivalent),
// the PKI path, and the *KubexmsKubeConf (as *common.KubeConf) to generate
// and write the certificate and key files. For this stubbed environment,
// the steps that call these will simulate file creation.
