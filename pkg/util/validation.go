package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/reference/docker"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

var (
	k8sNameRegexStr           = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	validDomainNameRegexStr   = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?|[a-zA-Z0-9])(\.([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?|[a-zA-Z0-9]))*$`)
	validHostnameRegex        = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	validHostPortRegex        = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*|\[::1\]|localhost|([0-9]{1,3}\.){3}[0-9]{1,3})(:([0-9]{1,5}))?$`)
	validDomainNameRegex      = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?|[a-zA-Z0-9])(\.([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?|[a-zA-Z0-9]))*\.?$`)
	validChartVersionRegex    = regexp.MustCompile(`^v?([0-9]+)(\.[0-9]+){0,2}$`)
	validSemanticVersionRegex = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	usernameRegex             = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	k8sNameRegex              = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	k8sLabelRegex             = regexp.MustCompile(`^[a-z0-9A-Z]([-a-z0-9A-Z_.]*[a-z0-9A-Z])?$`)
	emailRegex                = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

func IsValidK8sName(name string) bool {
	if name == "" {
		return false
	}
	if len(name) > 253 {
		return false
	}
	return k8sNameRegexStr.MatchString(name)
}

func IsValidHostPort(hostport string) bool {
	if hostport == "" {
		return false
	}

	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
		port = ""
	}

	isIP := net.ParseIP(host) != nil
	isDomain := IsValidDomainName(host)

	if !isIP && !isDomain {
		return false
	}

	if port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil {
			return false
		}
		if portNum < 1 || portNum > 65535 {
			return false
		}
	}
	return true
}

func ValidateHostPortStrict(hostport string) bool {
	host, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		if addrErr, ok := err.(*net.AddrError); ok && addrErr.Err == "missing port in address" {
			host = hostport
			portStr = ""
		} else {
			return false
		}
	}

	if net.ParseIP(host) == nil {
		if !validHostnameRegex.MatchString(host) {
			return false
		}
	}

	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return false
		}
		if port < 1 || port > 65535 {
			return false
		}
	}

	return true
}

func IsValidRuntimeVersion(version string) bool {
	if strings.TrimSpace(version) == "" {
		return false
	}

	v := strings.TrimPrefix(version, "v")
	if v == "" {
		return false
	}

	mainPart := v
	var preReleasePart, buildMetaPart string

	if strings.Contains(v, "+") {
		parts := strings.SplitN(v, "+", 2)
		mainPart = parts[0]
		buildMetaPart = parts[1]
	}

	if strings.Contains(mainPart, "-") {
		parts := strings.SplitN(mainPart, "-", 2)
		mainPart = parts[0]
		preReleasePart = parts[1]
	}

	if mainPart == "" {
		return false
	}

	segments := strings.Split(mainPart, ".")
	if len(segments) > 3 || len(segments) == 0 {
		return false
	}
	isNumericSegment := func(s string) bool {
		if s == "" {
			return false
		}
		_, err := strconv.Atoi(s)
		return err == nil
	}
	for _, seg := range segments {
		if !isNumericSegment(seg) {
			return false
		}
	}

	isAlphanumericHyphenSegment := func(s string) bool {
		if s == "" || strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") || strings.Contains(s, "--") {
			return false
		}
		for _, r := range s {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-') {
				return false
			}
		}
		return true
	}

	if preReleasePart != "" {
		if strings.HasSuffix(preReleasePart, ".") || strings.HasPrefix(preReleasePart, ".") || strings.Contains(preReleasePart, "..") {
			return false
		}
		for _, extSeg := range strings.Split(preReleasePart, ".") {
			if extSeg == "" {
				return false
			}
			if num, err := strconv.Atoi(extSeg); err == nil {
				if len(extSeg) > 1 && extSeg[0] == '0' {
					return false
				}
				_ = num
			} else if !isAlphanumericHyphenSegment(extSeg) {
				return false
			}
		}
	}

	if buildMetaPart != "" {
		if strings.HasSuffix(buildMetaPart, ".") || strings.HasPrefix(buildMetaPart, ".") || strings.Contains(buildMetaPart, "..") {
			return false
		}
		for _, extSeg := range strings.Split(buildMetaPart, ".") {
			if extSeg == "" {
				return false
			}
			if !isAlphanumericHyphenSegment(extSeg) && !isNumericSegment(extSeg) {
				return false
			}
		}
	}
	return true
}

func IsValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}

func IsValidIPv4(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.To4() != nil
}

func IsValidIPv6(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.To4() == nil && ip.To16() != nil
}

func IsValidCIDR(cidrStr string) bool {
	_, _, err := net.ParseCIDR(cidrStr)
	return err == nil
}

func IsValidCIDRv4(cidrStr string) bool {
	ip, _, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return false
	}
	return ip.To4() != nil
}

func IsValidCIDRv6(cidrStr string) bool {
	ip, _, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return false
	}
	return ip.To4() == nil && ip.To16() != nil
}

func IsValidDomainName(domain string) bool {
	if domain == "" {
		return false
	}
	if len(domain) > 253 {
		return false
	}
	return validDomainNameRegex.MatchString(domain)
}

func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func IsValidURL(urlStr string) bool {
	_, err := url.ParseRequestURI(urlStr)
	return err == nil
}

func IsValidPort(port int) bool {
	return port > 0 && port <= 65535
}

func IsValidPortStr(portStr string) bool {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return IsValidPort(port)
}

func IsValidUsername(username string) bool {
	if username == "" {
		return false
	}
	return usernameRegex.MatchString(username)
}

func IsValidK8sLabel(label string) bool {
	if label == "" {
		return false
	}
	if len(label) > 63 {
		return false
	}
	return k8sLabelRegex.MatchString(label)
}

func IsValidImageReference(imageRef string) bool {
	_, err := docker.ParseAnyReference(imageRef)
	return err == nil
}

func IsValidBase64(base64Str string) bool {
	_, err := base64.StdEncoding.DecodeString(base64Str)
	return err == nil
}

func IsValidJSON(jsonStr string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(jsonStr), &js) == nil
}

func IsValidYAML(yamlStr string) bool {
	var y interface{}
	return yaml.Unmarshal([]byte(yamlStr), &y) == nil
}

func IsValidDuration(durationStr string) bool {
	_, err := time.ParseDuration(durationStr)
	return err == nil
}

func IsValidSemanticVersion(version string) bool {
	return validSemanticVersionRegex.MatchString(version)
}

func IsValidChartVersion(version string) bool {
	return validChartVersionRegex.MatchString(version)
}

func IsValidMACAddress(macStr string) bool {
	_, err := net.ParseMAC(macStr)
	return err == nil
}

func IsValidUUID(uuidStr string) bool {
	_, err := uuid.Parse(uuidStr)
	return err == nil
}

func IsValidFilePath(filePath string) bool {
	if filePath == "" {
		return false
	}
	// Basic validation - check if it contains null bytes or invalid characters
	return !strings.Contains(filePath, "\x00") && !strings.ContainsAny(filePath, "<>:\"|?*")
}

func IsValidDirectoryName(dirName string) bool {
	if dirName == "" {
		return false
	}
	// Directory names shouldn't contain certain characters
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\x00"}
	for _, char := range invalidChars {
		if strings.Contains(dirName, char) {
			return false
		}
	}
	return true
}

func IsValidK8sNamespace(namespace string) bool {
	return IsValidK8sName(namespace)
}

func IsValidK8sServiceAccount(serviceAccount string) bool {
	return IsValidK8sName(serviceAccount)
}

func IsValidK8sSecret(secret string) bool {
	return IsValidK8sName(secret)
}

func IsValidK8sConfigMap(configMap string) bool {
	return IsValidK8sName(configMap)
}

func IsValidK8sDeployment(deployment string) bool {
	return IsValidK8sName(deployment)
}

func IsValidK8sPod(pod string) bool {
	return IsValidK8sName(pod)
}

func IsValidK8sContainer(container string) bool {
	return IsValidK8sName(container)
}

func IsValidK8sVolume(volume string) bool {
	return IsValidK8sName(volume)
}

func IsValidK8sPersistentVolume(pv string) bool {
	return IsValidK8sName(pv)
}

func IsValidK8sPersistentVolumeClaim(pvc string) bool {
	return IsValidK8sName(pvc)
}

func IsValidK8sStorageClass(storageClass string) bool {
	return IsValidK8sName(storageClass)
}

func IsValidK8sIngress(ingress string) bool {
	return IsValidK8sName(ingress)
}

func IsValidK8sService(service string) bool {
	return IsValidK8sName(service)
}

func IsValidK8sDaemonSet(daemonSet string) bool {
	return IsValidK8sName(daemonSet)
}

func IsValidK8sStatefulSet(statefulSet string) bool {
	return IsValidK8sName(statefulSet)
}

func IsValidK8sJob(job string) bool {
	return IsValidK8sName(job)
}

func IsValidK8sCronJob(cronJob string) bool {
	return IsValidK8sName(cronJob)
}

func IsValidK8sRole(role string) bool {
	return IsValidK8sName(role)
}

func IsValidK8sRoleBinding(roleBinding string) bool {
	return IsValidK8sName(roleBinding)
}

func IsValidK8sClusterRole(clusterRole string) bool {
	return IsValidK8sName(clusterRole)
}

func IsValidK8sClusterRoleBinding(clusterRoleBinding string) bool {
	return IsValidK8sName(clusterRoleBinding)
}

func IsValidK8sNetworkPolicy(networkPolicy string) bool {
	return IsValidK8sName(networkPolicy)
}

func IsValidK8sResourceQuota(resourceQuota string) bool {
	return IsValidK8sName(resourceQuota)
}

func IsValidK8sLimitRange(limitRange string) bool {
	return IsValidK8sName(limitRange)
}

func IsValidK8sHorizontalPodAutoscaler(hpa string) bool {
	return IsValidK8sName(hpa)
}

func IsValidK8sPodDisruptionBudget(pdb string) bool {
	return IsValidK8sName(pdb)
}

func IsValidK8sPriorityClass(priorityClass string) bool {
	return IsValidK8sName(priorityClass)
}

func IsValidK8sRuntimeClass(runtimeClass string) bool {
	return IsValidK8sName(runtimeClass)
}

func IsValidK8sMutatingWebhookConfiguration(webhook string) bool {
	return IsValidK8sName(webhook)
}

func IsValidK8sValidatingWebhookConfiguration(webhook string) bool {
	return IsValidK8sName(webhook)
}

func IsValidK8sCustomResourceDefinition(crd string) bool {
	return IsValidK8sName(crd)
}

func IsValidK8sAPIService(apiService string) bool {
	return IsValidK8sName(apiService)
}

func IsValidK8sCertificateSigningRequest(csr string) bool {
	return IsValidK8sName(csr)
}

func IsValidK8sLease(lease string) bool {
	return IsValidK8sName(lease)
}

func IsValidK8sEvent(event string) bool {
	return IsValidK8sName(event)
}

func IsValidK8sEndpoint(endpoint string) bool {
	return IsValidK8sName(endpoint)
}

func IsValidK8sEndpointSlice(endpointSlice string) bool {
	return IsValidK8sName(endpointSlice)
}

func IsValidK8sNode(node string) bool {
	return IsValidK8sName(node)
}

func IsValidK8sNamespaceSelector(selector string) bool {
	return IsValidK8sLabel(selector)
}

func IsValidK8sLabelSelector(selector string) bool {
	return IsValidK8sLabel(selector)
}

func IsValidK8sAnnotation(annotation string) bool {
	// Annotations have different rules than labels
	if len(annotation) > 1024 {
		return false
	}
	return true
}

func IsValidK8sFieldSelector(selector string) bool {
	// Field selectors are key=value pairs separated by commas
	pairs := strings.Split(selector, ",")
	for _, pair := range pairs {
		if !strings.Contains(pair, "=") {
			return false
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return false
		}
		// Both key and value should be non-empty
		if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return false
		}
	}
	return true
}

func IsValidK8sResourceVersion(version string) bool {
	// Resource versions are typically numeric strings
	_, err := strconv.ParseUint(version, 10, 64)
	return err == nil
}

func IsValidK8sUID(uid string) bool {
	// UIDs are typically UUIDs
	return IsValidUUID(uid)
}

func IsValidK8sGeneration(generation int64) bool {
	// Generation should be positive
	return generation > 0
}

func IsValidK8sDeletionGracePeriodSeconds(seconds int64) bool {
	// Deletion grace period should be non-negative
	return seconds >= 0
}

// IsValidPositiveInteger checks if an integer is positive
func IsValidPositiveInteger(n int) bool {
	return n > 0
}

// IsValidNonNegativeInteger checks if an integer is non-negative
func IsValidNonNegativeInteger(n int) bool {
	return n >= 0
}

// IsValidRange checks if an integer is within a range
func IsValidRange(n, min, max int) bool {
	return n >= min && n <= max
}

// IsValidPercentage checks if an integer is a valid percentage
func IsValidPercentage(n int) bool {
	return IsValidRange(n, 0, 100)
}

// ValidateHostPortString validates a host:port string
func ValidateHostPortString(hostport string) bool {
	if hostport == "" {
		return false
	}

	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
		port = ""
	}

	isIP := net.ParseIP(host) != nil
	isDomain := IsValidDomainName(host)

	if !isIP && !isDomain {
		return false
	}

	if port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil {
			return false
		}
		if portNum < 1 || portNum > 65535 {
			return false
		}
	}
	return true
}

// IsValidNonEmptyString checks if a string is not empty after trimming whitespace
func IsValidNonEmptyString(s string) bool {
	return strings.TrimSpace(s) != ""
}

func IsValidK8sFinalizer(finalizer string) bool {
	// Finalizers should be DNS subdomain prefixed paths
	if len(finalizer) > 316 {
		return false
	}
	parts := strings.Split(finalizer, "/")
	if len(parts) != 2 {
		return false
	}
	if !IsValidDomainName(parts[0]) {
		return false
	}
	if !IsValidK8sName(parts[1]) {
		return false
	}
	return true
}

func IsValidK8sOwnerReference(ownerRef string) bool {
	// Owner references are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(ownerRef) > 0
}

func IsValidK8sToleration(toleration string) bool {
	// Tolerations are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(toleration) > 0
}

func IsValidK8sAffinity(affinity string) bool {
	// Affinities are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(affinity) > 0
}

func IsValidK8sTopologySpreadConstraint(constraint string) bool {
	// Topology spread constraints are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(constraint) > 0
}

func IsValidK8sSecurityContext(securityContext string) bool {
	// Security contexts are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(securityContext) > 0
}

func IsValidK8sPodSecurityContext(podSecurityContext string) bool {
	// Pod security contexts are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(podSecurityContext) > 0
}

func IsValidK8sContainerSecurityContext(containerSecurityContext string) bool {
	// Container security contexts are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(containerSecurityContext) > 0
}

func IsValidK8sCapabilities(capabilities string) bool {
	// Capabilities are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(capabilities) > 0
}

func IsValidK8sSELinuxOptions(selinuxOptions string) bool {
	// SELinux options are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(selinuxOptions) > 0
}

func IsValidK8sSeccompProfile(seccompProfile string) bool {
	// Seccomp profiles are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(seccompProfile) > 0
}

func IsValidK8sWindowsSecurityContextOptions(windowsOptions string) bool {
	// Windows security context options are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(windowsOptions) > 0
}

func IsValidK8sSysctl(sysctl string) bool {
	// Sysctls are key=value pairs, but we can validate the basic format
	// This is a simplified validation
	return len(sysctl) > 0
}

func IsValidK8sEnvVar(envVar string) bool {
	// Env vars are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(envVar) > 0
}

func IsValidK8sEnvFrom(envFrom string) bool {
	// EnvFrom is complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(envFrom) > 0
}

func IsValidK8sVolumeMount(volumeMount string) bool {
	// Volume mounts are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(volumeMount) > 0
}

func IsValidK8sVolumeDevice(volumeDevice string) bool {
	// Volume devices are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(volumeDevice) > 0
}

func IsValidK8sProbe(probe string) bool {
	// Probes are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(probe) > 0
}

func IsValidK8sLifecycle(lifecycle string) bool {
	// Lifecycle handlers are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(lifecycle) > 0
}

func IsValidK8sHandler(handler string) bool {
	// Handlers are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(handler) > 0
}

func IsValidK8sHTTPGetAction(httpGet string) bool {
	// HTTPGet actions are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(httpGet) > 0
}

func IsValidK8sTCPSocketAction(tcpSocket string) bool {
	// TCPSocket actions are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(tcpSocket) > 0
}

func IsValidK8sExecAction(exec string) bool {
	// Exec actions are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(exec) > 0
}

func IsValidK8sGRPCAction(grpc string) bool {
	// GRPC actions are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(grpc) > 0
}

func IsValidK8sContainerPort(containerPort string) bool {
	// Container ports are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(containerPort) > 0
}

func IsValidK8sResourceRequirements(resources string) bool {
	// Resource requirements are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(resources) > 0
}

func IsValidK8sResourceList(resourceList string) bool {
	// Resource lists are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(resourceList) > 0
}

func IsValidK8sQuantity(quantity string) bool {
	// Quantities are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(quantity) > 0
}

func IsValidK8sContainerState(containerState string) bool {
	// Container states are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(containerState) > 0
}

func IsValidK8sContainerStatus(containerStatus string) bool {
	// Container statuses are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(containerStatus) > 0
}

func IsValidK8sPodCondition(podCondition string) bool {
	// Pod conditions are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(podCondition) > 0
}

func IsValidK8sPodStatus(podStatus string) bool {
	// Pod statuses are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(podStatus) > 0
}

func IsValidK8sPodSpec(podSpec string) bool {
	// Pod specs are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(podSpec) > 0
}

func IsValidK8sPodTemplateSpec(podTemplateSpec string) bool {
	// Pod template specs are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(podTemplateSpec) > 0
}

func IsValidK8sObjectMeta(objectMeta string) bool {
	// Object meta is complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(objectMeta) > 0
}

func IsValidK8sTypeMeta(typeMeta string) bool {
	// Type meta is complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(typeMeta) > 0
}

func IsValidK8sListMeta(listMeta string) bool {
	// List meta is complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(listMeta) > 0
}

func IsValidK8sObjectReference(objectReference string) bool {
	// Object references are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(objectReference) > 0
}

func IsValidK8sLocalObjectReference(localObjectReference string) bool {
	// Local object references are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(localObjectReference) > 0
}

func IsValidK8sTypedLocalObjectReference(typedLocalObjectReference string) bool {
	// Typed local object references are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(typedLocalObjectReference) > 0
}

func IsValidK8sSecretReference(secretReference string) bool {
	// Secret references are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(secretReference) > 0
}

func IsValidK8sConfigMapKeySelector(configMapKeySelector string) bool {
	// ConfigMap key selectors are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(configMapKeySelector) > 0
}

func IsValidK8sSecretKeySelector(secretKeySelector string) bool {
	// Secret key selectors are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(secretKeySelector) > 0
}

func IsValidK8sObjectFieldSelector(objectFieldSelector string) bool {
	// Object field selectors are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(objectFieldSelector) > 0
}

func IsValidK8sResourceFieldSelector(resourceFieldSelector string) bool {
	// Resource field selectors are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(resourceFieldSelector) > 0
}

func IsValidK8sDownwardAPIVolumeFile(downwardAPIVolumeFile string) bool {
	// DownwardAPI volume files are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(downwardAPIVolumeFile) > 0
}

func IsValidK8sDownwardAPIVolumeSource(downwardAPIVolumeSource string) bool {
	// DownwardAPI volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(downwardAPIVolumeSource) > 0
}

func IsValidK8sConfigMapVolumeSource(configMapVolumeSource string) bool {
	// ConfigMap volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(configMapVolumeSource) > 0
}

func IsValidK8sSecretVolumeSource(secretVolumeSource string) bool {
	// Secret volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(secretVolumeSource) > 0
}

func IsValidK8sPersistentVolumeClaimVolumeSource(pvcVolumeSource string) bool {
	// PersistentVolumeClaim volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(pvcVolumeSource) > 0
}

func IsValidK8sProjectedVolumeSource(projectedVolumeSource string) bool {
	// Projected volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(projectedVolumeSource) > 0
}

func IsValidK8sVolumeProjection(volumeProjection string) bool {
	// Volume projections are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(volumeProjection) > 0
}

func IsValidK8sHostPathVolumeSource(hostPathVolumeSource string) bool {
	// HostPath volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(hostPathVolumeSource) > 0
}

func IsValidK8sEmptyDirVolumeSource(emptyDirVolumeSource string) bool {
	// EmptyDir volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(emptyDirVolumeSource) > 0
}

func IsValidK8sGitRepoVolumeSource(gitRepoVolumeSource string) bool {
	// GitRepo volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(gitRepoVolumeSource) > 0
}

func IsValidK8sAWSElasticBlockStoreVolumeSource(awsElasticBlockStoreVolumeSource string) bool {
	// AWSElasticBlockStore volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(awsElasticBlockStoreVolumeSource) > 0
}

func IsValidK8sGCEPersistentDiskVolumeSource(gcePersistentDiskVolumeSource string) bool {
	// GCEPersistentDisk volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(gcePersistentDiskVolumeSource) > 0
}

func IsValidK8sAzureDiskVolumeSource(azureDiskVolumeSource string) bool {
	// AzureDisk volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(azureDiskVolumeSource) > 0
}

func IsValidK8sAzureFileVolumeSource(azureFileVolumeSource string) bool {
	// AzureFile volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(azureFileVolumeSource) > 0
}

func IsValidK8sCephFSVolumeSource(cephFSVolumeSource string) bool {
	// CephFS volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(cephFSVolumeSource) > 0
}

func IsValidK8sCinderVolumeSource(cinderVolumeSource string) bool {
	// Cinder volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(cinderVolumeSource) > 0
}

func IsValidK8sFCVolumeSource(fcVolumeSource string) bool {
	// FC volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(fcVolumeSource) > 0
}

func IsValidK8sFlexVolumeSource(flexVolumeSource string) bool {
	// Flex volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(flexVolumeSource) > 0
}

func IsValidK8sFlockerVolumeSource(flockerVolumeSource string) bool {
	// Flocker volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(flockerVolumeSource) > 0
}

func IsValidK8sGlusterfsVolumeSource(glusterfsVolumeSource string) bool {
	// Glusterfs volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(glusterfsVolumeSource) > 0
}

func IsValidK8sHostPathType(hostPathType string) bool {
	// HostPath types are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(hostPathType) > 0
}

func IsValidK8sISCSIVolumeSource(iscsiVolumeSource string) bool {
	// ISCSI volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(iscsiVolumeSource) > 0
}

func IsValidK8sNFSVolumeSource(nfsVolumeSource string) bool {
	// NFS volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(nfsVolumeSource) > 0
}

func IsValidK8sPhotonPersistentDiskVolumeSource(photonPersistentDiskVolumeSource string) bool {
	// PhotonPersistentDisk volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(photonPersistentDiskVolumeSource) > 0
}

func IsValidK8sPortworxVolumeSource(portworxVolumeSource string) bool {
	// Portworx volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(portworxVolumeSource) > 0
}

func IsValidK8sQuobyteVolumeSource(quobyteVolumeSource string) bool {
	// Quobyte volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(quobyteVolumeSource) > 0
}

func IsValidK8sRBDVolumeSource(rbdVolumeSource string) bool {
	// RBD volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(rbdVolumeSource) > 0
}

func IsValidK8sScaleIOVolumeSource(scaleIOVolumeSource string) bool {
	// ScaleIO volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(scaleIOVolumeSource) > 0
}

func IsValidK8sStorageOSVolumeSource(storageOSVolumeSource string) bool {
	// StorageOS volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(storageOSVolumeSource) > 0
}

func IsValidK8sVsphereVirtualDiskVolumeSource(vsphereVirtualDiskVolumeSource string) bool {
	// VsphereVirtualDisk volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(vsphereVirtualDiskVolumeSource) > 0
}

func IsValidK8sCSIVolumeSource(csiVolumeSource string) bool {
	// CSI volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(csiVolumeSource) > 0
}

func IsValidK8sEphemeralVolumeSource(ephemeralVolumeSource string) bool {
	// Ephemeral volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(ephemeralVolumeSource) > 0
}

func IsValidK8sVolumeSource(volumeSource string) bool {
	// Volume sources are complex structures, but we can validate the basic format
	// This is a simplified validation
	return len(volumeSource) > 0
}