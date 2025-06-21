package etcd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/resource" // For LocalCertificateHandle path logic if needed
)

// DistributeEtcdCertsStep uploads necessary ETCD certificates to ETCD and Master nodes.
type DistributeEtcdCertsStep struct {
	meta                spec.StepMeta
	ClusterName         string   // Used to construct local cert paths via LocalCertificateHandle logic
	CertBaseDirOnControlNode string // Base directory on control node where certs are located (e.g., .kubexm/${cluster_name}/certs/etcd)
	RemotePKIDir        string   // Target directory on remote nodes (e.g., /etc/etcd/pki or /etc/kubernetes/pki/etcd)
	EtcdNodeRole        string   // Role name for ETCD nodes
	MasterNodeRole      string   // Role name for Master nodes
	Sudo                bool     // Whether to use sudo for mkdir and upload.
}

// NewDistributeEtcdCertsStep creates a new DistributeEtcdCertsStep.
func NewDistributeEtcdCertsStep(instanceName, clusterName, certBaseDir, remotePKIDir, etcdRole, masterRole string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "DistributeEtcdCertificates"
	}
	if remotePKIDir == "" {
		remotePKIDir = "/etc/etcd/pki" // Default for etcd nodes
	}
	if etcdRole == "" {
		etcdRole = "etcd" // Default role name
	}
	if masterRole == "" {
		masterRole = "master" // Default role name
	}

	return &DistributeEtcdCertsStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Distributes ETCD CA, server, peer, and client certificates to relevant nodes.",
		},
		ClusterName:          clusterName,
		CertBaseDirOnControlNode: certBaseDir, // If empty, will be derived
		RemotePKIDir:         remotePKIDir,
		EtcdNodeRole:         etcdRole,
		MasterNodeRole:       masterRole,
		Sudo:                 sudo,
	}
}

func (s *DistributeEtcdCertsStep) Meta() *spec.StepMeta {
	return &s.meta
}

// getLocalCertPath constructs the path on the control node for a given certificate.
func (s *DistributeEtcdCertsStep) getLocalCertPath(ctx runtime.StepContext, certFileName string) string {
	baseDir := s.CertBaseDirOnControlNode
	if baseDir == "" {
		// Derive using similar logic to LocalCertificateHandle.Path()
		// GlobalWorkDir/${cluster_name}/certs/etcd/${CertFileName}
		baseDir = filepath.Join(ctx.GetGlobalWorkDir(), s.ClusterName, "certs", "etcd")
	}
	return filepath.Join(baseDir, certFileName)
}

func (s *DistributeEtcdCertsStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	certsToCheck := s.getRequiredCertsForHost(host)
	if len(certsToCheck) == 0 {
		logger.Info("No ETCD certificates required for this host's role(s). Skipping precheck for this host.")
		return true, nil // Nothing to check for this host based on its role
	}

	for _, certFile := range certsToCheck {
		remoteCertPath := filepath.Join(s.RemotePKIDir, certFile) // Adjust RemotePKIDir if masters use different path
		exists, err := runnerSvc.Exists(ctx.GoContext(), conn, remoteCertPath)
		if err != nil {
			logger.Warn("Failed to check for existing remote certificate, will attempt upload.", "path", remoteCertPath, "error", err)
			return false, nil
		}
		if !exists {
			logger.Info("Remote certificate does not exist.", "path", remoteCertPath)
			return false, nil
		}
		// TODO: Add checksum or content verification if possible and necessary.
		// This would require local certs to be available to this Precheck to compare against.
	}

	logger.Info("All required ETCD certificates appear to exist on remote host.", "pki_dir", s.RemotePKIDir)
	return true, nil
}

func (s *DistributeEtcdCertsStep) getRequiredCertsForHost(host connector.Host) []string {
	isEtcdNode := false
	isMasterNode := false
	for _, role := range host.GetRoles() {
		if role == s.EtcdNodeRole {
			isEtcdNode = true
		}
		if role == s.MasterNodeRole {
			isMasterNode = true
		}
	}

	var certs []string
	if isEtcdNode {
		certs = append(certs, "ca.pem", "server.pem", "server-key.pem", "peer.pem", "peer-key.pem")
	}
	if isMasterNode { // kube-apiserver needs to connect to etcd
		// Avoid duplicates if a node is both etcd and master
		currentCertsMap := make(map[string]bool)
		for _, c := range certs {
			currentCertsMap[c] = true
		}
		if !currentCertsMap["ca.pem"] {
			certs = append(certs, "ca.pem")
		}
		// Assuming apiserver-etcd-client certs are named as such
		certs = append(certs, "apiserver-etcd-client.pem", "apiserver-etcd-client-key.pem")
	}
	return certs
}

func (s *DistributeEtcdCertsStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	certsToDistribute := s.getRequiredCertsForHost(host)
	if len(certsToDistribute) == 0 {
		logger.Info("No ETCD certificates required for this host's role(s). Skipping distribution for this host.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Determine remote PKI directory. For masters, it might be different if they don't run etcd.
	// For simplicity, current step uses one RemotePKIDir. This might need refinement if paths differ significantly.
	// e.g. etcd nodes: /etc/etcd/pki, master nodes for apiserver client certs: /etc/kubernetes/pki/etcd
	// For now, we assume RemotePKIDir is appropriate for the certs being copied to this host.
	// A more complex setup might involve different RemotePKIDirs based on host role.
	effectiveRemotePKIDir := s.RemotePKIDir
	// Example adjustment (can be made more robust):
	// if slices.Contains(host.GetRoles(), s.MasterNodeRole) && !slices.Contains(host.GetRoles(), s.EtcdNodeRole) {
	//    effectiveRemotePKIDir = filepath.Join("/etc/kubernetes/pki", "etcd") // Kubeadm default for external etcd certs
	// }


	logger.Info("Ensuring remote PKI directory exists.", "path", effectiveRemotePKIDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, effectiveRemotePKIDir, "0700", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote PKI directory %s: %w", effectiveRemotePKIDir, err)
	}

	for _, certFileName := range certsToDistribute {
		localCertPath := s.getLocalCertPath(ctx, certFileName)
		remoteCertPath := filepath.Join(effectiveRemotePKIDir, certFileName)

		logger.Info("Uploading certificate.", "local", localCertPath, "remote", remoteCertPath)

		// Check if local cert exists first (on control node)
		// This requires access to control node's filesystem from StepContext, or this check happens earlier.
		// For now, assume localCertPath is valid and accessible by UploadFile.
		// A LocalFileExists check on control node could be added if runner supports it or via os.Stat.

		fileTransferOptions := &connector.FileTransferOptions{
			Permissions: "0600", // Secure permissions for private keys and certs
			Owner:       "etcd", // If etcd user exists and is relevant
			Group:       "etcd", // If etcd group exists
			Sudo:        s.Sudo,
		}
		// Keys should be 0600, public certs (ca.pem, server.pem) can be 0644 or 0640.
		if certFileName == "server-key.pem" || certFileName == "peer-key.pem" || certFileName == "apiserver-etcd-client-key.pem" {
			fileTransferOptions.Permissions = "0600"
		} else {
			fileTransferOptions.Permissions = "0644" // Or 0640 if group ownership is strict
		}


		errUpload := runnerSvc.UploadFile(ctx.GoContext(), localCertPath, remoteCertPath, fileTransferOptions, host)
		if errUpload != nil {
			return fmt.Errorf("failed to upload certificate %s from %s to %s:%s: %w",
				certFileName, localCertPath, host.GetName(), remoteCertPath, errUpload)
		}
	}

	logger.Info("All required ETCD certificates distributed successfully.", "pki_dir", effectiveRemotePKIDir)
	return nil
}

func (s *DistributeEtcdCertsStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	certsToRemove := s.getRequiredCertsForHost(host)
	if len(certsToRemove) == 0 {
		logger.Info("No ETCD certificates were designated for this host's role(s). Skipping rollback cleanup for this host.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil // Best effort
	}

	effectiveRemotePKIDir := s.RemotePKIDir // Use the same logic as Run if paths vary by role

	for _, certFileName := range certsToRemove {
		remoteCertPath := filepath.Join(effectiveRemotePKIDir, certFileName)
		logger.Info("Attempting to remove remote certificate for rollback.", "path", remoteCertPath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, remoteCertPath, s.Sudo); err != nil {
			logger.Warn("Failed to remove remote certificate during rollback (best effort).", "path", remoteCertPath, "error", err)
		}
	}
	// Optionally remove RemotePKIDir if it's empty and was created by this step.
	// This is often risky as other components might use it.
	logger.Info("Rollback attempt for distributing ETCD certificates finished.")
	return nil
}

var _ step.Step = (*DistributeEtcdCertsStep)(nil)
