package validation

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
)

// ValidationErrors is a collection of validation errors.
type ValidationErrors struct {
	errors []string
}

// Add records a new validation error with formatted message.
func (v *ValidationErrors) Add(format string, args ...interface{}) {
	v.errors = append(v.errors, fmt.Sprintf(format, args...))
}

// AddError records a new validation error.
func (v *ValidationErrors) AddError(path, message string) {
	v.errors = append(v.errors, fmt.Sprintf("%s: %s", path, message))
}

// Error returns the formatted error string.
func (v *ValidationErrors) Error() string {
	if len(v.errors) == 0 {
		return ""
	}
	return strings.Join(v.errors, "\n")
}

// HasErrors returns true if there are any validation errors.
func (v *ValidationErrors) HasErrors() bool {
	return len(v.errors) > 0
}

// IsEmpty returns true if there are no validation errors.
func (v *ValidationErrors) IsEmpty() bool {
	return len(v.errors) == 0
}

// Count returns the number of validation errors.
func (v *ValidationErrors) Count() int {
	return len(v.errors)
}

// Clear removes all validation errors.
func (v *ValidationErrors) Clear() {
	v.errors = nil
}

// GetErrors returns all validation errors as a slice.
func (v *ValidationErrors) GetErrors() []string {
	return v.errors
}

// Network validation functions

// IsValidURL checks if a string is a valid URL.
func IsValidURL(u string) bool {
	_, err := url.ParseRequestURI(u)
	return err == nil
}

// IsValidIP checks if the string is a valid IP address.
func IsValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}

// IsValidIPv4 checks if the string is a valid IPv4 address.
func IsValidIPv4(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	return ip != nil && ip.To4() != nil
}

// IsValidIPv6 checks if the string is a valid IPv6 address.
func IsValidIPv6(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	return ip != nil && ip.To4() == nil
}

// IsValidCIDR checks if the string is a valid CIDR notation.
func IsValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

// IsValidPort checks if the port number is valid (1-65535).
func IsValidPort(port int) bool {
	return port >= 1 && port <= 65535
}

// IsValidPortString checks if the port string is valid.
func IsValidPortString(portStr string) bool {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return IsValidPort(port)
}

// IsValidDomainName checks if a string is a valid domain name.
// It uses a regex based on RFC 1035 and RFC 1123.
func IsValidDomainName(domain string) bool {
	const domainValidationRegexString = `^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)$`
	domainRegex := regexp.MustCompile(domainValidationRegexString)
	return domainRegex.MatchString(domain)
}

// IsValidHostname checks if a string is a valid hostname.
func IsValidHostname(hostname string) bool {
	if len(hostname) > 253 {
		return false
	}
	return IsValidDomainName(hostname)
}

// IsValidFQDN checks if a string is a valid fully qualified domain name.
func IsValidFQDN(fqdn string) bool {
	if !strings.HasSuffix(fqdn, ".") {
		fqdn += "."
	}
	return IsValidDomainName(strings.TrimSuffix(fqdn, "."))
}

// Version validation functions

// validChartVersionRegex is the compiled regular expression for chart versions.
var validChartVersionRegex = regexp.MustCompile(`^v?([0-9]+)(\.[0-9]+){0,2}$`)

// IsValidChartVersion checks if the version string matches common chart version patterns.
func IsValidChartVersion(version string) bool {
	if version == "latest" || version == "stable" {
		return true
	}
	return validChartVersionRegex.MatchString(version)
}

// validSemanticVersionRegex is the compiled regular expression for semantic versions.
var validSemanticVersionRegex = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// IsValidSemanticVersion checks if the version string follows semantic versioning.
func IsValidSemanticVersion(version string) bool {
	return validSemanticVersionRegex.MatchString(version)
}

// IsValidKubernetesVersion checks if the version string is a valid Kubernetes version.
func IsValidKubernetesVersion(version string) bool {
	return ContainsString(common.SupportedKubernetesVersions, version)
}

// IsValidEtcdVersion checks if the version string is a valid etcd version.
func IsValidEtcdVersion(version string) bool {
	return ContainsString(common.SupportedEtcdVersions, version)
}

// IsValidDockerVersion checks if the version string is a valid Docker version.
func IsValidDockerVersion(version string) bool {
	return ContainsString(common.SupportedDockerVersions, version)
}

// IsValidContainerdVersion checks if the version string is a valid containerd version.
func IsValidContainerdVersion(version string) bool {
	return ContainsString(common.SupportedContainerdVersions, version)
}

// System validation functions

// IsValidUsername checks if a string is a valid Unix username.
func IsValidUsername(username string) bool {
	const usernameRegexString = `^[a-z_][a-z0-9_-]{0,31}$`
	usernameRegex := regexp.MustCompile(usernameRegexString)
	return usernameRegex.MatchString(username)
}

// IsValidFilePath checks if a string is a valid file path.
func IsValidFilePath(path string) bool {
	if path == "" {
		return false
	}
	// Check for invalid characters
	invalidChars := []string{"\x00", "\x01", "\x02", "\x03", "\x04", "\x05", "\x06", "\x07", "\x08", "\x09", "\x0a", "\x0b", "\x0c", "\x0d", "\x0e", "\x0f"}
	for _, char := range invalidChars {
		if strings.Contains(path, char) {
			return false
		}
	}
	return true
}

// IsValidDirectory checks if a string is a valid directory path.
func IsValidDirectory(path string) bool {
	return IsValidFilePath(path)
}

// IsValidArchitecture checks if the architecture string is supported.
func IsValidArchitecture(arch string) bool {
	return ContainsString(common.SupportedArchitectures, arch)
}

// IsValidOperatingSystem checks if the operating system string is supported.
func IsValidOperatingSystem(os string) bool {
	return ContainsString(common.SupportedOperatingSystems, os)
}

// IsValidLinuxDistribution checks if the Linux distribution string is supported.
func IsValidLinuxDistribution(distro string) bool {
	return ContainsString(common.SupportedLinuxDistributions, distro)
}

// IsValidContainerRuntime checks if the container runtime string is supported.
func IsValidContainerRuntime(runtime string) bool {
	return ContainsString(common.SupportedContainerRuntimes, runtime)
}

// IsValidCNIType checks if the CNI type string is supported.
func IsValidCNIType(cniType string) bool {
	return ContainsString(common.SupportedCNITypes, cniType)
}

// IsValidLoadBalancerType checks if the load balancer type string is supported.
func IsValidInternalLoadBalancerType(lbType string) bool {
	return ContainsString(common.SupportedInternalLoadBalancerTypes, lbType)
}

// IsValidExternalLoadBalancerType checks if the external load balancer type string is supported.
func IsValidExternalLoadBalancerType(lbType string) bool {
	return ContainsString(common.SupportedExternalLoadBalancerTypes, lbType)
}

// IsValidKubernetesDeploymentType checks if the Kubernetes deployment type string is supported.
func IsValidKubernetesDeploymentType(deploymentType string) bool {
	return ContainsString(common.SupportedKubernetesDeploymentTypes, deploymentType)
}

// IsValidEtcdDeploymentType checks if the etcd deployment type string is supported.
func IsValidEtcdDeploymentType(deploymentType string) bool {
	return ContainsString(common.SupportedEtcdDeploymentTypes, deploymentType)
}

// String validation functions

// IsValidNonEmptyString checks if a string is not empty and not just whitespace.
func IsValidNonEmptyString(s string) bool {
	return strings.TrimSpace(s) != ""
}

// IsValidStringLength checks if a string length is within the specified range.
func IsValidStringLength(s string, minLen, maxLen int) bool {
	length := len(s)
	return length >= minLen && length <= maxLen
}

// IsValidStringPattern checks if a string matches the specified regex pattern.
func IsValidStringPattern(s, pattern string) bool {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return regex.MatchString(s)
}

// Number validation functions

// IsValidPositiveInteger checks if a number is a positive integer.
func IsValidPositiveInteger(n int) bool {
	return n > 0
}

// IsValidNonNegativeInteger checks if a number is a non-negative integer.
func IsValidNonNegativeInteger(n int) bool {
	return n >= 0
}

// IsValidRange checks if a number is within the specified range (inclusive).
func IsValidRange(n, min, max int) bool {
	return n >= min && n <= max
}

// IsValidPercentage checks if a number is a valid percentage (0-100).
func IsValidPercentage(n int) bool {
	return IsValidRange(n, 0, 100)
}

// Time validation functions

// IsValidDuration checks if a duration string is valid.
func IsValidDuration(duration string) bool {
	_, err := time.ParseDuration(duration)
	return err == nil
}

// IsValidTimeFormat checks if a time string matches the specified format.
func IsValidTimeFormat(timeStr, format string) bool {
	_, err := time.Parse(format, timeStr)
	return err == nil
}

// Helper functions

// ContainsString checks if a string slice contains a specific string.
func ContainsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ContainsInt checks if an integer slice contains a specific integer.
func ContainsInt(slice []int, item int) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

// IsValidEmail checks if a string is a valid email address.
func IsValidEmail(email string) bool {
	const emailRegexString = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	emailRegex := regexp.MustCompile(emailRegexString)
	return emailRegex.MatchString(email)
}

// IsValidMAC checks if a string is a valid MAC address.
func IsValidMAC(mac string) bool {
	_, err := net.ParseMAC(mac)
	return err == nil
}

// IsValidBase64 checks if a string is valid base64 encoded.
func IsValidBase64(s string) bool {
	const base64RegexString = `^[A-Za-z0-9+/]*={0,2}$`
	base64Regex := regexp.MustCompile(base64RegexString)
	return base64Regex.MatchString(s) && len(s)%4 == 0
}

// IsValidJSON checks if a string is valid JSON (basic check).
func IsValidJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

// IsValidYAML checks if a string looks like valid YAML (basic check).
func IsValidYAML(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.HasPrefix(s, "{") && !strings.HasPrefix(s, "[")
}

// Kubernetes-specific validation functions

// IsValidKubernetesName checks if a string is a valid Kubernetes resource name.
func IsValidKubernetesName(name string) bool {
	const k8sNameRegexString = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	k8sNameRegex := regexp.MustCompile(k8sNameRegexString)
	return k8sNameRegex.MatchString(name) && len(name) <= 253
}

// IsValidKubernetesLabel checks if a string is a valid Kubernetes label.
func IsValidKubernetesLabel(label string) bool {
	const k8sLabelRegexString = `^[a-z0-9A-Z]([-a-z0-9A-Z_.]*[a-z0-9A-Z])?$`
	k8sLabelRegex := regexp.MustCompile(k8sLabelRegexString)
	return k8sLabelRegex.MatchString(label) && len(label) <= 63
}

// IsValidKubernetesAnnotation checks if a string is a valid Kubernetes annotation.
func IsValidKubernetesAnnotation(annotation string) bool {
	// Kubernetes annotations can contain any UTF-8 character
	return len(annotation) <= 262144 // 256KB limit
}

// IsValidKubernetesNamespace checks if a string is a valid Kubernetes namespace name.
func IsValidKubernetesNamespace(namespace string) bool {
	return IsValidKubernetesName(namespace)
}

// Complex validation functions

// ValidateHostConfig validates a host configuration.
func ValidateHostConfig(host, user, password string, port int) *ValidationErrors {
	verrs := &ValidationErrors{}
	
	if !IsValidNonEmptyString(host) {
		verrs.AddError("host", "cannot be empty")
	} else if !IsValidIP(host) && !IsValidHostname(host) {
		verrs.AddError("host", "must be a valid IP address or hostname")
	}
	
	if !IsValidNonEmptyString(user) {
		verrs.AddError("user", "cannot be empty")
	} else if !IsValidUsername(user) {
		verrs.AddError("user", "must be a valid username")
	}
	
	if !IsValidNonEmptyString(password) {
		verrs.AddError("password", "cannot be empty")
	}
	
	if !IsValidPort(port) {
		verrs.AddError("port", "must be between 1 and 65535")
	}
	
	return verrs
}

// ValidateNetworkConfig validates a network configuration.
func ValidateNetworkConfig(podCIDR, serviceCIDR, dnsIP string) *ValidationErrors {
	verrs := &ValidationErrors{}
	
	if !IsValidCIDR(podCIDR) {
		verrs.AddError("podCIDR", "must be a valid CIDR notation")
	}
	
	if !IsValidCIDR(serviceCIDR) {
		verrs.AddError("serviceCIDR", "must be a valid CIDR notation")
	}
	
	if !IsValidIP(dnsIP) {
		verrs.AddError("dnsIP", "must be a valid IP address")
	}
	
	return verrs
}

// ValidateVersionConfig validates version configurations.
func ValidateVersionConfig(kubernetesVersion, etcdVersion, dockerVersion string) *ValidationErrors {
	verrs := &ValidationErrors{}
	
	if !IsValidKubernetesVersion(kubernetesVersion) {
		verrs.AddError("kubernetesVersion", "unsupported Kubernetes version")
	}
	
	if !IsValidEtcdVersion(etcdVersion) {
		verrs.AddError("etcdVersion", "unsupported etcd version")
	}
	
	if !IsValidDockerVersion(dockerVersion) {
		verrs.AddError("dockerVersion", "unsupported Docker version")
	}
	
	return verrs
}
