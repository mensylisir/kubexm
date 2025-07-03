package v1alpha1

import (
	"net"
	// "regexp" // No longer needed for isValidRuntimeVersion
	"strconv"
	"strings"
	"unicode"
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

// isValidRegistryHostPort checks if the string is a valid hostname, IP, or host:port / ip:port combination.
// It also handles IPv6 addresses correctly, including those with ports.
func isValidRegistryHostPort(hp string) bool {
	hp = strings.TrimSpace(hp)
	if hp == "" {
		return false
	}

	host, port, err := net.SplitHostPort(hp)
	if err == nil {
		// Successfully split, means it's host:port or [ipv6]:port
		// Validate host part (can be domain, IPv4, or unwrapped IPv6 from brackets)
		// net.SplitHostPort already unwraps IPv6 from brackets.
		if ! (isValidIP(host) || isValidDomainName(host)) {
			return false
		}
		// Validate port part
		return isValidPort(port)
	} else {
		// net.SplitHostPort failed. This means input string is not in host:port form.
		// So, the entire string must be a valid IP address or a valid domain name.
		// It also handles bracketed IPv6 without port, e.g. "[::1]", if isValidIP handles it post-unwrapping.
		if strings.HasPrefix(hp, "[") && strings.HasSuffix(hp, "]") {
			unwrappedHost := hp[1 : len(hp)-1]
			return isValidIP(unwrappedHost)
		}
		// For "::1:8080" (unbracketed IPv6 that looks like it has a port)
		// net.ParseIP might be lenient. We want to ensure this is false if it's not a simple IP/domain.
		// If it has multiple colons (is IPv6-like) and the part after the last colon is a valid port,
		// and it's not bracketed, we consider it invalid for this function's purpose.
		if strings.Count(hp, ":") > 1 && !strings.HasPrefix(hp, "[") {
			lastColon := strings.LastIndex(hp, ":")
			// Check if the segment after the last colon looks like a port.
			// This is a heuristic. If net.ParseIP considers "ipv6part:portpart" as a single valid IP,
			// then this specific check is to override that for our host/port context.
			if lastColon > 0 && lastColon < len(hp)-1 { // Ensure colon is not at start/end
				if isValidPort(hp[lastColon+1:]) {
					// It looks like an unbracketed IPv6 with a port.
					// We rely on the fact that net.SplitHostPort would have parsed this if it were
					// a simple hostname:port or ipv4:port. Since SplitHostPort failed, and it looks like
					// unbracketed ipv6:port, we deem it invalid.
					return false
				}
			}
		}

		return isValidIP(hp) || isValidDomainName(hp)
	}
}

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

func isValidRuntimeVersion(version string) bool {
	if len(version) == 0 {
		return false
	}

	v := strings.TrimPrefix(version, "v")
	if len(v) == 0 && strings.HasPrefix(version, "v") { // only "v"
		return false
	}
	if len(v) == 0 { // empty string after removing optional "v"
		return false
	}


	mainPart := v
	var extensionPart string

	if strings.Contains(v, "-") {
		parts := strings.SplitN(v, "-", 2)
		mainPart = parts[0]
		if len(parts) > 1 {
			extensionPart = parts[1]
		}
	} else if strings.Contains(v, "+") { // some versions might use + for build metadata like semver
		parts := strings.SplitN(v, "+", 2)
		mainPart = parts[0]
		if len(parts) > 1 {
			extensionPart = parts[1] // treat build metadata similar to pre-release for this validation
		}
	}


	if mainPart == "" && extensionPart == "" { // e.g. if original was just "-"
		return false
	}

	// Validate main version part (X.Y.Z...)
	segments := strings.Split(mainPart, ".")
	if len(segments) == 0 || (len(segments)==1 && segments[0]=="") { // empty or just "."
		return false
	}

	for i, seg := range segments {
		if seg == "" { return false } // e.g. "1..2" or "1.2."

		// Check for "1.2.3a" type invalidity only on the last segment of the main part
		if i == len(segments)-1 {
			hasTrailingLetter := false
			numPrefix := ""
			for j, r := range seg {
				if unicode.IsDigit(r) {
					if hasTrailingLetter { return false } // Digit after letter in same segment: "1a2"
					numPrefix += string(r)
				} else if unicode.IsLetter(r) {
					if numPrefix == "" && j > 0 { return false } // e.g. ".a" if seg was ".a" (not possible with split)
					hasTrailingLetter = true
				} else {
					return false // Invalid char in numeric segment
				}
			}
			if hasTrailingLetter && numPrefix == "" { return false } // Segment is all letters e.g. "alpha"
			if hasTrailingLetter { return false } //This makes "1.2.3a" invalid
			if !isNumericSegment(seg) { return false } // Fallback, should be caught by above
		} else { // For earlier segments, they must be purely numeric
			if !isNumericSegment(seg) {
				return false
			}
		}
	}

	// Validate extension part (e.g. "alpha.1", "build.123", "rc1-custom")
	if extensionPart != "" {
		extSegments := strings.FieldsFunc(extensionPart, func(r rune) bool { return r == '.' || r == '-' })
		if len(extSegments) == 0 && len(extensionPart) > 0 { // e.g. if extensionPart was only "." or "-"
			return false
		}
		for _, extSeg := range extSegments {
			if extSeg == "" { return false} // e.g. "alpha..1" or "rc1--custom"
			// Extension segments can be numeric or alphanumeric with hyphens
			// isAlphanumericHyphenSegment already checks for leading/trailing hyphens.
			if !isAlphanumericHyphenSegment(extSeg) && !isNumericSegment(extSeg) {
				return false
			}
		}
	}
	return true
}
