package v1alpha1

import (
	"strconv"
	"strings"
	"unicode"
	// "net" // No longer used directly in this file after moving helpers
)

// ContainerRuntimeType defines the type of container runtime.
type ContainerRuntimeType string

const (
	ContainerRuntimeDocker     ContainerRuntimeType = "docker"
	ContainerRuntimeContainerd ContainerRuntimeType = "containerd"
	// Add other runtimes like cri-o, isula if supported by YAML
)

// ContainerRuntimeConfig is a wrapper for specific container runtime configurations.
// Corresponds to `kubernetes.containerRuntime` in YAML.
type ContainerRuntimeConfig struct {
	// Type specifies the container runtime to use (e.g., "docker", "containerd").
	Type ContainerRuntimeType `json:"type,omitempty" yaml:"type,omitempty"`
	// Version of the container runtime.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`

	// Docker holds Docker-specific configurations.
	// Only applicable if Type is "docker".
	Docker *DockerConfig `json:"docker,omitempty" yaml:"docker,omitempty"`
	// Containerd holds Containerd-specific configurations.
	// Only applicable if Type is "containerd".
	Containerd *ContainerdConfig `json:"containerd,omitempty" yaml:"containerd,omitempty"`
}

// SetDefaults_ContainerRuntimeConfig sets default values for ContainerRuntimeConfig.
func SetDefaults_ContainerRuntimeConfig(cfg *ContainerRuntimeConfig) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		cfg.Type = ContainerRuntimeDocker // Default to Docker
	}

	if cfg.Type == ContainerRuntimeDocker {
		if cfg.Docker == nil {
			cfg.Docker = &DockerConfig{}
		}
		SetDefaults_DockerConfig(cfg.Docker)
	}

	if cfg.Type == ContainerRuntimeContainerd {
		if cfg.Containerd == nil {
			cfg.Containerd = &ContainerdConfig{}
		}
		SetDefaults_ContainerdConfig(cfg.Containerd)
	}
}

// Validate_ContainerRuntimeConfig validates ContainerRuntimeConfig.
func Validate_ContainerRuntimeConfig(cfg *ContainerRuntimeConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		verrs.Add("%s: section cannot be nil", pathPrefix)
		return
	}
	validTypes := []ContainerRuntimeType{ContainerRuntimeDocker, ContainerRuntimeContainerd, ""} // Allow empty for default
	isValid := false
	for _, vt := range validTypes {
		if cfg.Type == vt || (cfg.Type == "" && vt == ContainerRuntimeDocker) { // Defaulting "" to Docker
			isValid = true
			break
		}
	}
	if !isValid {
		verrs.Add("%s.type: invalid container runtime type '%s'", pathPrefix, cfg.Type)
	}

	if cfg.Type == ContainerRuntimeDocker {
		if cfg.Docker == nil {
			// Defaulting handles this
		} else {
			Validate_DockerConfig(cfg.Docker, verrs, pathPrefix+".docker")
		}
	} else if cfg.Docker != nil {
		verrs.Add("%s.docker: can only be set if type is 'docker'", pathPrefix)
	}

	if cfg.Type == ContainerRuntimeContainerd {
		if cfg.Containerd == nil {
			// Defaulting handles this
		} else {
			Validate_ContainerdConfig(cfg.Containerd, verrs, pathPrefix+".containerd")
		}
	} else if cfg.Containerd != nil {
		verrs.Add("%s.containerd: can only be set if type is 'containerd'", pathPrefix)
	}

	if cfg.Version != "" {
		if strings.TrimSpace(cfg.Version) == "" {
			verrs.Add("%s.version: cannot be only whitespace if specified", pathPrefix)
		} else if !isValidRuntimeVersion(cfg.Version) {
			verrs.Add("%s.version: '%s' is not a recognized version format", pathPrefix, cfg.Version)
		}
	}
}

func isNumericSegment(s string) bool {
	if s == "" {
		return false // Empty segments are not numeric
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// isValidHostPort has been moved to pkg/util/utils.go as ValidateHostPortString
// func isValidHostPort(hp string) bool {
// 	return isValidRegistryHostPort(hp)
// }

// isValidRegistryHostPort has been moved to pkg/util/utils.go as ValidateHostPortString
// func isValidRegistryHostPort(hp string) bool {
// 	hp = strings.TrimSpace(hp)
// 	if hp == "" {
// 		return false
// 	}
// 	host, port, err := net.SplitHostPort(hp)
// 	if err == nil {
// 		// Successfully split, means it's host:port or [ipv6]:port
// 		if !(isValidIP(host) || isValidDomainName(host)) { // isValidIP and isValidDomainName are now expected from pkg/util
// 			return false
// 		}
// 		return isValidPort(port)
// 	} else {
// 		// net.SplitHostPort failed.
// 		if strings.HasPrefix(hp, "[") && strings.HasSuffix(hp, "]") {
// 			unwrappedHost := hp[1 : len(hp)-1]
// 			return isValidIP(unwrappedHost) // Expected from pkg/util
// 		}
// 		// Simplified: if not split, it's either a valid IP or a valid domain.
// 		// Complex unbracketed IPv6:port cases are tricky and rely on robust isValidIP/isValidDomainName.
// 		return isValidIP(hp) || isValidDomainName(hp) // Expected from pkg/util
// 	}
// }

// isValidIP has been moved to pkg/util/utils.go as IsValidIP
// func isValidIP(ipStr string) bool {
// 	return net.ParseIP(ipStr) != nil
// }

// isValidDomainName has been moved to pkg/util/utils.go as IsValidDomainName
// func isValidDomainName(domain string) bool { ... }


// isValidPort checks if a string is a valid port number (1-65535)
func isValidPort(portStr string) bool {
	if portStr == "" {
		return false
	}
	port, err := parseInt(portStr) // Assuming parseInt is available
	if err != nil {
		return false
	}
	return port >= 1 && port <= 65535
}

// ... (other code) ...

// parseInt uses strconv.Atoi for robust parsing.
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// parseError is no longer needed as strconv.Atoi returns its own error type.

func isAlphanumericHyphenSegment(s string) bool {
	if s == "" {
		return false // Empty segments are not valid extension identifiers
	}
	if strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") {
		return false // Identifiers should not start or end with a hyphen
	}
	if strings.Contains(s, "--") {
		return false // No consecutive hyphens
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' {
			return false
		}
	}
	return true
}


// isValidRuntimeVersion validates a runtime version string.
// It expects formats like "1.2.3", "v1.2.3", "1.2", "v1.2.3-alpha.1", "1.2.3+build.456".
// It does not strictly enforce SemVer for all parts but aims for common patterns.
func isValidRuntimeVersion(version string) bool {
	if strings.TrimSpace(version) == "" {
		return false
	}

	v := strings.TrimPrefix(version, "v")
	if v == "" && strings.HasPrefix(version, "v") { // Original was just "v"
		return false
	}
	if v == "" { // Original was empty or just "v"
		return false
	}

	mainPart := v
	var preReleasePart, buildMetaPart string

	// Split build metadata first
	if strings.Contains(v, "+") {
		parts := strings.SplitN(v, "+", 2)
		mainPart = parts[0]
		if len(parts) > 1 {
			buildMetaPart = parts[1]
		}
	} else { // If no '+', then the whole string (after 'v') is mainPart or mainPart-preRelease
		mainPart = v
	}

	// Split pre-release from mainPart (if no '+' was found or '+' was after '-')
	if strings.Contains(mainPart, "-") {
		parts := strings.SplitN(mainPart, "-", 2)
		mainPart = parts[0] // This is now purely X.Y.Z
		if len(parts) > 1 {
			preReleasePart = parts[1]
		}
	}

	if mainPart == "" { // e.g. version was "v-alpha" or "v+build"
		return false
	}

	// Validate main version part (X.Y.Z)
	segments := strings.Split(mainPart, ".")
	if len(segments) > 3 || len(segments) == 0 { // Allow X, X.Y, X.Y.Z
		return false
	}
	for _, seg := range segments {
		if !isNumericSegment(seg) { // Main version segments must be numeric
			return false
		}
	}

	// Validate pre-release part (e.g., "alpha.1", "rc-1")
	if preReleasePart != "" {
		if buildMetaPart != "" && strings.Contains(preReleasePart, "+") { // Pre-release part should not contain build meta if already split by '+'
			return false
		}
		if strings.HasSuffix(preReleasePart, ".") || strings.HasPrefix(preReleasePart, ".") || strings.Contains(preReleasePart, "..") {
			return false // Disallow leading/trailing/double dots
		}
		extSegments := strings.Split(preReleasePart, ".") // SemVer pre-release identifiers are dot-separated
		if len(extSegments) == 0 && len(preReleasePart) > 0 {
			return false
		}
		for _, extSeg := range extSegments {
			if extSeg == "" { return false } // Should be caught by above checks, but good for safety
			if isNumericSegment(extSeg) {
				if len(extSeg) > 1 && extSeg[0] == '0' { return false } // No leading zeros for numeric identifiers
			} else if !isAlphanumericHyphenSegment(extSeg) {
				return false
			}
		}
	}

	// Validate build metadata part (e.g., "build.123", "001", "alpha-build.test")
	if buildMetaPart != "" {
		if strings.HasSuffix(buildMetaPart, ".") || strings.HasPrefix(buildMetaPart, ".") || strings.Contains(buildMetaPart, "..") {
			return false // Disallow leading/trailing/double dots
		}
		extSegments := strings.Split(buildMetaPart, ".") // SemVer build identifiers are dot-separated
		if len(extSegments) == 0 && len(buildMetaPart) > 0 {
			return false
		}
		for _, extSeg := range extSegments {
			if extSeg == "" { return false } // Should be caught by above checks
			// Build metadata segments can be numeric or alphanumeric with hyphens.
			// Leading zeros are allowed in numeric build metadata.
			if !isAlphanumericHyphenSegment(extSeg) && !isNumericSegment(extSeg) {
				return false
			}
		}
	}

	return true
}
