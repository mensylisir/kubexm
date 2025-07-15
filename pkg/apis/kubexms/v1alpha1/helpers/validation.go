package helpers

import (
	"encoding/base64"
	"encoding/json"
	"github.com/containerd/containerd/reference/docker"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"gopkg.in/yaml.v3"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
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
	return ip != nil && ip.To4() != nil && !strings.Contains(ipStr, ":")
}

func IsValidIPv6(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	return ip != nil && ip.To4() == nil
}

func IsValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

func IsValidPort(port int) bool {
	return port >= 1 && port <= 65535
}

func IsValidPortString(portStr string) bool {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return IsValidPort(port)
}

func IsValidDomainName(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	if net.ParseIP(domain) != nil {
		return false
	}

	if !validDomainNameRegex.MatchString(domain) {
		return false
	}

	parts := strings.Split(strings.TrimRight(domain, "."), ".")
	if len(parts) == 1 && domain == "localhost" {
		return true
	}

	if len(parts) > 1 {
		tld := parts[len(parts)-1]
		if _, err := strconv.Atoi(tld); err == nil {
			return false // TLD is purely numeric.
		}
	}
	return true
}

func IsValidHostname(hostname string) bool {
	return IsValidDomainName(hostname)
}

func IsValidFQDN(fqdn string) bool {
	return IsValidDomainName(fqdn)
}

func ValidateHostPortString(hp string) bool {
	hp = strings.TrimSpace(hp)
	if hp == "" {
		return false
	}

	host, port, err := net.SplitHostPort(hp)
	if err == nil {
		return (IsValidIP(host) || IsValidDomainName(host)) && IsValidPortString(port)
	}

	if strings.HasPrefix(hp, "[") && strings.HasSuffix(hp, "]") {
		return IsValidIP(hp[1 : len(hp)-1])
	}

	return IsValidIP(hp) || IsValidDomainName(hp)
}

func IsValidURL(rawURL string) bool {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return false
	}
	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return false
	}
	return parsedURL.Scheme != "" && parsedURL.Host != ""
}

func IsValidHTTPURL(rawURL string) bool {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return false
	}
	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return false
	}
	return (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") && parsedURL.Host != ""
}

func IsValidImageReference(ref string) bool {
	_, err := docker.ParseAnyReference(ref)
	return err == nil
}

func IsValidChartVersion(version string) bool {
	if version == "latest" || version == "stable" {
		return true
	}
	return validChartVersionRegex.MatchString(version)
}

func IsValidSemanticVersion(version string) bool {
	return validSemanticVersionRegex.MatchString(version)
}

func IsValidKubernetesVersion(version string) bool {
	return ContainsString(common.SupportedKubernetesVersions, version)
}

func IsValidEtcdVersion(version string) bool {
	return ContainsString(common.SupportedEtcdVersions, version)
}

func IsValidDockerVersion(version string) bool {
	return ContainsString(common.SupportedDockerVersions, version)
}

func IsValidContainerdVersion(version string) bool {
	return ContainsString(common.SupportedContainerdVersions, version)
}

func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}

func IsValidFilePath(path string) bool {
	if path == "" {
		return false
	}
	return !strings.Contains(path, "\x00")
}

func IsValidDirectory(path string) bool {
	return IsValidFilePath(path)
}

func IsValidArchitecture(arch string) bool {
	return ContainsString(common.SupportedArchitectures, arch)
}

func IsValidOperatingSystem(os string) bool {
	return ContainsString(common.SupportedOperatingSystems, os)
}

func IsValidLinuxDistribution(distro string) bool {
	return ContainsString(common.SupportedLinuxDistributions, distro)
}

func IsValidContainerRuntime(runtime string) bool {
	return ContainsString(common.SupportedContainerRuntimes, runtime)
}

func IsValidCNIType(cniType string) bool {
	return ContainsString(common.SupportedCNITypes, cniType)
}

func IsValidInternalLoadBalancerType(lbType string) bool {
	return ContainsString(common.SupportedInternalLoadBalancerTypes, lbType)
}

func IsValidExternalLoadBalancerType(lbType string) bool {
	return ContainsString(common.SupportedExternalLoadBalancerTypes, lbType)
}

func IsValidKubernetesDeploymentType(deploymentType string) bool {
	return ContainsString(common.SupportedKubernetesDeploymentTypes, deploymentType)
}

func IsValidEtcdDeploymentType(deploymentType string) bool {
	return ContainsString(common.SupportedEtcdDeploymentTypes, deploymentType)
}

func IsValidNonEmptyString(s string) bool {
	return strings.TrimSpace(s) != ""
}

func IsValidStringLength(s string, minLen, maxLen int) bool {
	length := len(s)
	return length >= minLen && length <= maxLen
}

func IsValidStringPattern(s, pattern string) bool {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return regex.MatchString(s)
}

func IsValidPositiveInteger(n int) bool {
	return n > 0
}

func IsValidNonNegativeInteger(n int) bool {
	return n >= 0
}

func IsValidRange(n, min, max int) bool {
	return n >= min && n <= max
}

func IsValidPercentage(n int) bool {
	return IsValidRange(n, 0, 100)
}

func IsValidDuration(duration string) bool {
	_, err := time.ParseDuration(duration)
	return err == nil
}

func IsValidTimeFormat(timeStr, format string) bool {
	_, err := time.Parse(format, timeStr)
	return err == nil
}

func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func IsValidMAC(mac string) bool {
	_, err := net.ParseMAC(mac)
	return err == nil
}

func IsValidBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

func IsValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

func IsValidYAML(s string) bool {
	var y interface{}
	return yaml.Unmarshal([]byte(s), &y) == nil
}

func IsValidKubernetesName(name string) bool {
	return len(name) <= 253 && k8sNameRegex.MatchString(name)
}

func IsValidKubernetesLabel(label string) bool {
	if label == "" {
		return true
	}
	return len(label) <= 63 && k8sLabelRegex.MatchString(label)
}

func IsValidKubernetesAnnotation(annotation string) bool {
	return len(annotation) <= 262144 // 256KB limit
}

func IsValidKubernetesNamespace(namespace string) bool {
	return IsValidKubernetesName(namespace)
}

func ValidateHostConfig(host, user, password string, port int) *validation.ValidationErrors {
	verrs := &validation.ValidationErrors{}

	if !IsValidNonEmptyString(host) {
		verrs.Add("host", "cannot be empty")
	} else if !IsValidIP(host) && !IsValidHostname(host) {
		verrs.Add("host", "must be a valid IP address or hostname")
	}

	if !IsValidNonEmptyString(user) {
		verrs.Add("user", "cannot be empty")
	} else if !IsValidUsername(user) {
		verrs.Add("user", "must be a valid username")
	}

	if !IsValidNonEmptyString(password) {
		verrs.Add("password", "cannot be empty")
	}

	if !IsValidPort(port) {
		verrs.Add("port", "must be between 1 and 65535")
	}

	return verrs
}

func ValidateNetworkConfig(podCIDR, serviceCIDR, dnsIP string) *validation.ValidationErrors {
	verrs := &validation.ValidationErrors{}

	if !IsValidCIDR(podCIDR) {
		verrs.Add("podCIDR", "must be a valid CIDR notation")
	}

	if !IsValidCIDR(serviceCIDR) {
		verrs.Add("serviceCIDR", "must be a valid CIDR notation")
	}

	if !IsValidIP(dnsIP) {
		verrs.Add("dnsIP", "must be a valid IP address")
	}

	return verrs
}

func ValidateVersionConfig(kubernetesVersion, etcdVersion, dockerVersion string) *validation.ValidationErrors {
	verrs := &validation.ValidationErrors{}

	if !IsValidKubernetesVersion(kubernetesVersion) {
		verrs.Add("kubernetesVersion", "unsupported Kubernetes version")
	}

	if !IsValidEtcdVersion(etcdVersion) {
		verrs.Add("etcdVersion", "unsupported etcd version")
	}

	if !IsValidDockerVersion(dockerVersion) {
		verrs.Add("dockerVersion", "unsupported Docker version")
	}

	return verrs
}
