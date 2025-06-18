package pki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
	// "k8s.io/client-go/util/cert" // For the actual cert.AltNames type if KubexmsAltNames was not used in definitions
	// For this subtask, our stubs (KubexmsCert, KubexmsAltNames, KubexmsKubeConf) are in this 'pki' package.
	// Actual certs.GenerateCerts would be imported from Kubekey's utils.
)

// Constants for SharedData keys
const (
	// DefaultEtcdPKIPathKey, DefaultEtcdAltNamesKey, DefaultEtcdCACertObjectKey, DefaultKubeConfKey
	// are assumed to be defined (e.g. in generate_etcd_ca.go or a constants package).
	// We will reuse them here by convention.

	DefaultHostsKey                  = "clusterHosts" // Key for []HostSpecForPKI from cluster config
	DefaultEtcdGeneratedFilesListKey = "etcdGeneratedNodeCertFiles" // Output: list of generated file basenames
)

// HostSpecForPKI structures host data relevant for PKI generation.
// This might be a common struct, but defined here if specific to PKI steps.
// It was also defined in generate_etcd_alt_names.go; ideally, it's common.
// For this subtask, we assume its definition is compatible or can be locally defined.
/*
type HostSpecForPKI struct {
	Name  string   `json:"name"`  // Hostname
	Roles []string `json:"roles"` // List of roles, e.g., "etcd", "master", "worker"
}
*/
// Using HostSpecForAltNames from generate_etcd_alt_names.go (if it's made common or this file is in same package)
// For now, let's assume HostSpecForAltNames also includes Roles or we adapt.
// Plan uses HostSpecForPKI { Name, Roles }, let's define it here.
type HostSpecForPKI struct {
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	// InternalAddress might be needed if altNames are generated per host specifically here,
	// but plan implies altNames are pre-generated and passed in.
}


// GenerateEtcdNodeCertsStepSpec defines parameters for generating etcd node certificates.
type GenerateEtcdNodeCertsStepSpec struct {
	PKIPathSharedDataKey        string `json:"pkiPathSharedDataKey,omitempty"`
	AltNamesSharedDataKey       string `json:"altNamesSharedDataKey,omitempty"`       // Input: *KubexmsAltNames or *cert.AltNames
	CACertObjectSharedDataKey   string `json:"caCertObjectSharedDataKey,omitempty"` // Input: *KubexmsCert for CA
	KubeConfSharedDataKey       string `json:"kubeConfSharedDataKey,omitempty"`       // Input: *KubexmsKubeConf
	HostsSharedDataKey          string `json:"hostsSharedDataKey,omitempty"`          // Input: []HostSpecForPKI
	OutputGeneratedFilesListKey string `json:"outputGeneratedFilesListKey,omitempty"` // Output: []string of basenames
}

// GetName returns the name of the step.
func (s *GenerateEtcdNodeCertsStepSpec) GetName() string {
	return "Generate Etcd Node Certificates"
}

// PopulateDefaults sets default values for SharedData keys.
func (s *GenerateEtcdNodeCertsStepSpec) PopulateDefaults() {
	if s.PKIPathSharedDataKey == "" {
		s.PKIPathSharedDataKey = DefaultEtcdPKIPathKey
	}
	if s.AltNamesSharedDataKey == "" {
		s.AltNamesSharedDataKey = DefaultEtcdAltNamesKey
	}
	if s.CACertObjectSharedDataKey == "" {
		s.CACertObjectSharedDataKey = DefaultEtcdCACertObjectKey
	}
	if s.KubeConfSharedDataKey == "" {
		s.KubeConfSharedDataKey = DefaultKubeConfKey
	}
	if s.HostsSharedDataKey == "" {
		s.HostsSharedDataKey = DefaultHostsKey
	}
	if s.OutputGeneratedFilesListKey == "" {
		s.OutputGeneratedFilesListKey = DefaultEtcdGeneratedFilesListKey
	}
}

// GenerateEtcdNodeCertsStepExecutor implements the logic.
type GenerateEtcdNodeCertsStepExecutor struct{}

// isHostRole checks if a host has a specific role.
func (e *GenerateEtcdNodeCertsStepExecutor) isHostRole(hostRoles []string, targetRole string) bool {
	for _, role := range hostRoles {
		if strings.EqualFold(role, targetRole) {
			return true
		}
	}
	return false
}

// Check determines if all expected etcd node certificate files already exist.
func (e *GenerateEtcdNodeCertsStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for GenerateEtcdNodeCertsStep Check")
	}
	spec, ok := currentFullSpec.(*GenerateEtcdNodeCertsStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for GenerateEtcdNodeCertsStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())

	pkiPathVal, pkiPathOk := ctx.Task().Get(spec.PKIPathSharedDataKey)
	altNamesVal, altNamesOk := ctx.Task().Get(spec.AltNamesSharedDataKey) // Now expecting *certutil.AltNames
	hostsVal, hostsOk := ctx.Task().Get(spec.HostsSharedDataKey)

	if !pkiPathOk || !altNamesOk || !hostsOk {
		logger.Debug("Missing one or more required inputs (PKIPath, AltNames, Hosts) in Task Cache. Cannot check.")
		return false, nil
	}
	pkiPath, _ := pkiPathVal.(string)
	altNames, altNamesTypeOk := altNamesVal.(*certutil.AltNames) // Expecting actual k8s type
	hosts, hostsTypeOk := hostsVal.([]HostSpecForPKI)

	if pkiPath == "" || !altNamesTypeOk || altNames == nil || !hostsTypeOk || hosts == nil { // Check hostsTypeOk as well
		logger.Debug("Invalid type or empty value for required inputs (PKIPath, AltNames, Hosts). Cannot check.")
		return false, nil
	}

	for _, host := range hosts {
		hostName := host.Name
		if e.isHostRole(host.Roles, "etcd") {
			adminCertDef := KubexmsCertEtcdAdmin(hostName, altNames) // Use renamed function
			if !fileExists(filepath.Join(pkiPath, adminCertDef.BaseName+".pem")) || !fileExists(filepath.Join(pkiPath, adminCertDef.BaseName+"-key.pem")) {
				logger.Debugf("Etcd admin cert/key for host %s not found.", hostName)
				return false, nil
			}
			memberCertDef := KubexmsCertEtcdMember(hostName, altNames) // Use renamed function
			if !fileExists(filepath.Join(pkiPath, memberCertDef.BaseName+".pem")) || !fileExists(filepath.Join(pkiPath, memberCertDef.BaseName+"-key.pem")) {
				logger.Debugf("Etcd member cert/key for host %s not found.", hostName)
				return false, nil
			}
		}
		if e.isHostRole(host.Roles, "master") { // Assuming "master" role implies needing etcd client cert
			clientCertDef := KubexmsCertEtcdClient(hostName, altNames) // Use renamed function
			if !fileExists(filepath.Join(pkiPath, clientCertDef.BaseName+".pem")) || !fileExists(filepath.Join(pkiPath, clientCertDef.BaseName+"-key.pem")) {
				logger.Debugf("Etcd client cert/key for master host %s not found.", hostName)
				return false, nil
			}
		}
	}

	// Additionally, check if the output list of generated files is already in Task Cache.
	if _, exists := ctx.Task().Get(spec.OutputGeneratedFilesListKey); !exists {
		logger.Debugf("Output list of generated files (key: %s) not yet in Task Cache.", spec.OutputGeneratedFilesListKey)
		return false, nil
	}

	logger.Info("All expected etcd node certificates appear to exist, and output list is in Task Cache.")
	return true, nil
}

// Execute generates etcd node certificates.
func (e *GenerateEtcdNodeCertsStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for GenerateEtcdNodeCertsStep Execute"))
	}
	spec, ok := currentFullSpec.(*GenerateEtcdNodeCertsStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for GenerateEtcdNodeCertsStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil) // Local operation

	// Retrieve inputs from Task Cache
	pkiPathVal, pkiPathOk := ctx.Task().Get(spec.PKIPathSharedDataKey)
	altNamesVal, altNamesOk := ctx.Task().Get(spec.AltNamesSharedDataKey)
	caCertDefVal, caCertDefOk := ctx.Task().Get(spec.CACertObjectSharedDataKey)
	kubeConfVal, kubeConfOk := ctx.Task().Get(spec.KubeConfSharedDataKey)
	hostsVal, hostsOk := ctx.Task().Get(spec.HostsSharedDataKey)

	if !pkiPathOk || !altNamesOk || !caCertDefOk || !kubeConfOk || !hostsOk {
		res.Error = fmt.Errorf("missing one or more required inputs (PKIPath, AltNames, CACert, KubexmsKubeConf, Hosts) in Task Cache")
		res.Status = step.StatusFailed; return res
	}

	pkiPath, _ := pkiPathVal.(string)
	altNames, altNamesTypeOk := altNamesVal.(*certutil.AltNames) // Expecting actual k8s type
	caCertDef, caCertDefTypeOk := caCertDefVal.(*KubexmsCert)    // Using our renamed stub type for KubexmsCert
	kubeConf, kubeConfTypeOk := kubeConfVal.(*KubexmsKubeConf)  // Using our renamed stub type for KubexmsKubeConf
	hosts, hostsTypeOk := hostsVal.([]HostSpecForPKI)

	if !altNamesTypeOk || !caCertDefTypeOk || !kubeConfTypeOk || !hostsTypeOk || pkiPath == "" {
		res.Error = fmt.Errorf("invalid type or empty value for one or more required inputs (PKIPath, AltNames, CACert, KubexmsKubeConf, Hosts)")
		res.Status = step.StatusFailed; return res
	}

	generatedFileBaseNames := []string{}

	for _, host := range hosts {
		hostName := host.Name
		logger.Infof("Processing certificates for host: %s with roles: %v", hostName, host.Roles)

		// Generate certs for etcd members
		if e.isHostRole(host.Roles, "etcd") {
			// Etcd Admin/Server Cert (often the same as Member for simplicity, or a specific admin client)
			// Original script generates 'admin.pem' then 'member-*.pem'.
			// Let's follow KubexmsCertEtcdAdmin and KubexmsCertEtcdMember.

			adminCertDef := KubexmsCertEtcdAdmin(hostName, altNames) // Use renamed function
			logger.Infof("Defining etcd admin cert for %s: %s", hostName, adminCertDef.BaseName)
			// SIMULATE: err := certsutil.GenerateCerts(adminCertDef, caCertDef, pkiPath, kubeConf)
			simulateCertGeneration(pkiPath, adminCertDef.BaseName, logger)
			generatedFileBaseNames = append(generatedFileBaseNames, adminCertDef.BaseName+".pem", adminCertDef.BaseName+"-key.pem")

			memberCertDef := KubexmsCertEtcdMember(hostName, altNames) // Use renamed function
			logger.Infof("Defining etcd member peer/server cert for %s: %s", hostName, memberCertDef.BaseName)
			// SIMULATE: err := certsutil.GenerateCerts(memberCertDef, caCertDef, pkiPath, kubeConf)
			simulateCertGeneration(pkiPath, memberCertDef.BaseName, logger)
			generatedFileBaseNames = append(generatedFileBaseNames, memberCertDef.BaseName+".pem", memberCertDef.BaseName+"-key.pem")
		}

		// Generate etcd client certs for master nodes (kube-apiserver to connect to etcd)
		// The original script implies that `etcd-admin.pem` is for kube-apiserver.
		// If `KubexmsCertEtcdAdmin` is already fulfilling that, this might be redundant or for other clients.
		// The script logic was: if host is master AND etcd, generate admin. if host is master (only), generate client.
		// This suggests admin is a specific client for master+etcd nodes, and 'client' is for other masters.
		// For now, let's assume any master needs a client cert. If it's also etcd, it already got admin.
		// If KubexmsCertEtcdAdmin is the main client cert for API server, then this `client` loop might be different.
		// Let's assume the logic: master nodes that are NOT etcd nodes get a generic client cert.
		// Etcd nodes already have `etcd-admin` and `etcd-member` certs.
		// The `KubexmsCertEtcdAdmin` was defined as a client cert with "system:masters".
		// This seems sufficient for API servers on master nodes that are also etcd nodes.
		// What if a master is NOT an etcd node? It still needs to talk to etcd.
		// The original script's `generateEtcdCerts` has:
		//   - `etcd-admin.pem` (for apiserver, using `ControlPlaneEndpoint` and `PrimaryHost` from KubeConf) - seems global, not per-host.
		//   - `etcd-member-%s.pem` (per etcd host)
		// This step's current loop is per-host. The global "etcd-admin" cert might be another step or part of CA step.
		// For this per-host loop, `KubexmsCertEtcdClient` for master nodes seems appropriate if they need their own identity to etcd.

		if e.isHostRole(host.Roles, "master") {
			// If this master node is also an etcd node, it already got an admin cert.
			// If it's a master but NOT an etcd node, it might need a client cert.
			// However, the common pattern is one client cert for all apiservers.
			// Let's stick to the plan: generate client cert if it's a master.
			// This might result in multiple "etcd-client-*" certs.
			clientCertDef := KubexmsCertEtcdClient(hostName, altNames) // Use renamed function; Client cert for this specific master host
			logger.Infof("Defining etcd client cert for master host %s: %s", hostName, clientCertDef.BaseName)
			// SIMULATE: err := certsutil.GenerateCerts(clientCertDef, caCertDef, pkiPath, kubeConf)
			simulateCertGeneration(pkiPath, clientCertDef.BaseName, logger)
			generatedFileBaseNames = append(generatedFileBaseNames, clientCertDef.BaseName+".pem", clientCertDef.BaseName+"-key.pem")
		}
	}

	// Deduplicate generatedFileBaseNames before storing
	uniqueFiles := []string{}
	seen := make(map[string]bool)
	for _, name := range generatedFileBaseNames {
		if !seen[name] {
			uniqueFiles = append(uniqueFiles, name)
			seen[name] = true
		}
	}

	ctx.Task().Set(spec.OutputGeneratedFilesListKey, uniqueFiles)
	logger.Infof("Etcd node certificate generation simulated. List of generated file basenames stored in Task Cache: %v", uniqueFiles)

	done, checkErr := e.Check(ctx) // Pass context
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed; return res
	}
	if !done {
		res.Error = fmt.Errorf("post-execution check indicates etcd node certs generation was not fully successful")
		res.Status = step.StatusFailed; return res
	}

	// res.SetSucceeded() // Status is handled by NewResult
	return res
}

// fileExists is a helper for local file system checks.
func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil || !os.IsNotExist(err)
}

// simulateCertGeneration creates dummy cert/key files for testing.
func simulateCertGeneration(pkiPath, baseName string, logger runtime.Logger) {
	dummyCertPath := filepath.Join(pkiPath, baseName+".pem")
	dummyKeyPath := filepath.Join(pkiPath, baseName+"-key.pem")

	if !fileExists(dummyCertPath) {
		if errWrite := os.WriteFile(dummyCertPath, []byte(fmt.Sprintf("dummy cert for %s", baseName)), 0600); errWrite != nil {
			logger.Warnf("Failed to write dummy cert for %s: %v", baseName, errWrite)
		}
	}
	if !fileExists(dummyKeyPath) {
		if errWrite := os.WriteFile(dummyKeyPath, []byte(fmt.Sprintf("dummy key for %s", baseName)), 0600); errWrite != nil {
			logger.Warnf("Failed to write dummy key for %s: %v", baseName, errWrite)
		}
	}
}


func init() {
	step.Register(&GenerateEtcdNodeCertsStepSpec{}, &GenerateEtcdNodeCertsStepExecutor{})
}
