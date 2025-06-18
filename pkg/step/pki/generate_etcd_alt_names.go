package pki

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
	"k8s.io/client-go/util/cert"
	netutils "k8s.io/utils/net"
	// Assuming a constants package might exist for shared keys
	// "github.com/kubexms/kubexms/pkg/common/constants"
)

// SharedData key for etcd SANs.
const (
	DefaultEtcdAltNamesKey = "etcdAltNames"
)

// HostSpecForAltNames provides necessary host details for generating SANs.
type HostSpecForAltNames struct {
	Name            string `json:"name"`            // Hostname
	InternalAddress string `json:"internalAddress"` // Comma-separated list of internal IP addresses
}

// GenerateEtcdAltNamesStepSpec defines parameters for generating etcd SANs.
type GenerateEtcdAltNamesStepSpec struct {
	ControlPlaneEndpointDomain string                `json:"controlPlaneEndpointDomain,omitempty"` // e.g., "lb.example.com"
	DefaultLBDomain            string                `json:"defaultLBDomain,omitempty"`            // Default load balancer domain if ControlPlaneEndpointDomain is not set
	Hosts                      []HostSpecForAltNames `json:"hosts,omitempty"`                      // List of hosts in the cluster
	OutputAltNamesSharedDataKey string                `json:"outputAltNamesSharedDataKey,omitempty"`  // Key to store the generated *cert.AltNames
}

// GetName returns the name of the step.
func (s *GenerateEtcdAltNamesStepSpec) GetName() string {
	return "Generate Etcd Certificate AltNames"
}

// PopulateDefaults sets default values for the spec.
func (s *GenerateEtcdAltNamesStepSpec) PopulateDefaults() {
	if s.OutputAltNamesSharedDataKey == "" {
		s.OutputAltNamesSharedDataKey = DefaultEtcdAltNamesKey
	}
	if s.DefaultLBDomain == "" {
		// This default is from KubeKey's original script context for internal LB.
		s.DefaultLBDomain = "lb.kubesphere.local"
	}
}

// GenerateEtcdAltNamesStepExecutor implements the logic.
type GenerateEtcdAltNamesStepExecutor struct{}

// Check determines if etcd SANs have already been generated and stored.
func (e *GenerateEtcdAltNamesStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for GenerateEtcdAltNamesStep Check")
	}
	spec, ok := currentFullSpec.(*GenerateEtcdAltNamesStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for GenerateEtcdAltNamesStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults() // Ensure defaults are applied
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())

	val, exists := ctx.Task().Get(spec.OutputAltNamesSharedDataKey)
	if !exists {
		logger.Debugf("Etcd AltNames not found in Task Cache key '%s'. Generation needed.", spec.OutputAltNamesSharedDataKey)
		return false, nil
	}

	_, ok = val.(*cert.AltNames)
	if !ok {
		logger.Errorf("Invalid type in Task Cache for key '%s'. Expected *cert.AltNames, got %T.", spec.OutputAltNamesSharedDataKey, val)
		return false, fmt.Errorf("invalid type in Task Cache for key %s, expected *cert.AltNames", spec.OutputAltNamesSharedDataKey)
	}

	logger.Infof("Etcd AltNames already found in Task Cache key '%s'. Assuming correctly generated.", spec.OutputAltNamesSharedDataKey)
	return true, nil
}

// Execute generates and stores etcd SANs in Task Cache.
func (e *GenerateEtcdAltNamesStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for GenerateEtcdAltNamesStep Execute"))
	}
	spec, ok := currentFullSpec.(*GenerateEtcdAltNamesStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for GenerateEtcdAltNamesStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults()
	logger := ctx.Logger.SugaredLogger().With("step", spec.GetName())
	res := step.NewResult(ctx, startTime, nil) // Use new constructor

	var altName cert.AltNames

	// Default DNS names for etcd
	dnsList := []string{
		"localhost",
		// TODO: Consider if these cluster-internal names are always needed or configurable.
		// These are typical for in-cluster etcd access by apiserver.
		"etcd.kube-system.svc.cluster.local",
		"etcd.kube-system.svc",
		"etcd.kube-system",
		"etcd",
	}

	// Default IP addresses for etcd
	ipList := []net.IP{
		net.IPv4(127, 0, 0, 1), // Loopback IPv4
		net.IPv6loopback,       // Loopback IPv6
	}

	// Add Control Plane Endpoint or default LoadBalancer domain
	if stepSpec.ControlPlaneEndpointDomain != "" {
		dnsList = append(dnsList, stepSpec.ControlPlaneEndpointDomain)
		logger.Debugf("Added ControlPlaneEndpointDomain '%s' to etcd SANs.", stepSpec.ControlPlaneEndpointDomain)
	} else {
		dnsList = append(dnsList, stepSpec.DefaultLBDomain)
		logger.Debugf("ControlPlaneEndpointDomain not set, added DefaultLBDomain '%s' to etcd SANs.", stepSpec.DefaultLBDomain)
	}

	// Add host-specific names and IPs
	if len(stepSpec.Hosts) == 0 {
		logger.Warn("No host specifications provided in spec.Hosts. Etcd SANs will only include defaults and LB domain.")
	}
	for _, host := range stepSpec.Hosts {
		if host.Name != "" {
			if !containsString(dnsList, host.Name) { // Avoid duplicates
				dnsList = append(dnsList, host.Name)
			}
		}
		// Parse first IP from comma-separated InternalAddress
		if host.InternalAddress != "" {
			addrToParse := strings.Split(host.InternalAddress, ",")[0]
			internalIP := netutils.ParseIPSloppy(addrToParse) // Handles both IPv4 and IPv6
			if internalIP != nil {
				if !containsIP(ipList, internalIP) { // Avoid duplicates
					ipList = append(ipList, internalIP)
				}
			} else {
				logger.Warnf("Failed to parse InternalAddress '%s' to IP for host '%s'.", addrToParse, host.Name)
			}
		}
	}

	altName.DNSNames = dnsList
	altName.IPs = ipList

	ctx.Task().Set(spec.OutputAltNamesSharedDataKey, &altName)
	logger.Infof("Generated etcd AltNames. DNS: %v, IPs: %v. Stored in Task Cache key '%s'.", altName.DNSNames, altName.IPs, spec.OutputAltNamesSharedDataKey)

	// res.SetSucceeded() // Status is handled by NewResult based on error
	return res
}

// Helper to check if a string slice contains a string
func containsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// Helper to check if an IP slice contains an IP
func containsIP(slice []net.IP, ip net.IP) bool {
	for _, item := range slice {
		if item.Equal(ip) {
			return true
		}
	}
	return false
}

func init() {
	step.Register(&GenerateEtcdAltNamesStepSpec{}, &GenerateEtcdAltNamesStepExecutor{})
}
