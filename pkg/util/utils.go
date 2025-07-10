package util

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// ShellEscape provides basic shell escaping for a string.
// WARNING: This is a simplified version and may not cover all edge cases.
// For production use, a more robust library or approach might be needed if paths can be arbitrary.
func ShellEscape(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// StrPtr returns a pointer to the string value s.
func StrPtr(s string) *string {
	return &s
}

// BoolPtr returns a pointer to the bool value b.
func BoolPtr(b bool) *bool {
	return &b
}

// IntPtr returns a pointer to the int value i.
func IntPtr(i int) *int {
	return &i
}

// Int64Ptr returns a pointer to the int64 value i.
func Int64Ptr(i int64) *int64 {
	return &i
}

// Float64Ptr returns a pointer to the float64 value f.
func Float64Ptr(f float64) *float64 {
	return &f
}

// UintPtr returns a pointer to the uint value u.
func UintPtr(u uint) *uint {
	return &u
}

// IsValidRuntimeVersion validates a runtime version string.
// It expects formats like "1.2.3", "v1.2.3", "1.2", "v1.2.3-alpha.1", "1.2.3+build.456".
// It does not strictly enforce SemVer for all parts but aims for common patterns.
func IsValidRuntimeVersion(version string) bool {
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
	isNumericSegment := func(s string) bool {
		if s == "" { return false }
		for _, r := range s {
			if r < '0' || r > '9' { return false }
		}
		return true
	}
	for _, seg := range segments {
		if !isNumericSegment(seg) { // Main version segments must be numeric
			return false
		}
	}

	isAlphanumericHyphenSegment := func(s string) bool {
		if s == "" { return false }
		if strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") { return false }
		if strings.Contains(s, "--") { return false }
		for _, r := range s {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-') {
				return false
			}
		}
		return true
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
			if extSeg == "" { return false }
			if isNumericSegment(extSeg) {
				if len(extSeg) > 1 && extSeg[0] == '0' { return false }
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
			if extSeg == "" { return false }
			if !isAlphanumericHyphenSegment(extSeg) && !isNumericSegment(extSeg) {
				return false
			}
		}
	}
	return true
}

// IsValidCIDR checks if the given string is a valid CIDR notation.
func IsValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

// --- Validation Helpers ---

// IsValidIP checks if the given string is a valid IP address (IPv4 or IPv6).
func IsValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}

// IsValidDomainName checks if a string is a plausible domain name.
// This validation aims to be reasonably strict but may not cover all edge cases
// of internationalized domain names or very new TLDs.
func IsValidDomainName(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	// Domain must not be an IP address
	if net.ParseIP(domain) != nil {
		return false
	}

	// Regex for basic domain name structure (LDH: letters, digits, hyphen for labels)
	// Each label: starts and ends with alphanumeric, contains alphanumeric or hyphen, 1-63 chars.
	// Allows for a trailing dot indicating FQDN root.
	// Parts cannot start or end with a hyphen.
	fqdnRegex := `^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?|[a-zA-Z0-9])(\.([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?|[a-zA-Z0-9]))*\.?$`
	if matched, _ := regexp.MatchString(fqdnRegex, domain); !matched {
		return false
	}

	// Check for numeric-only TLD if it's not a single label domain (like "localhost")
	// and ensure parts do not start/end with hyphens (partially covered by regex but good for explicit check)
	parts := strings.Split(strings.TrimRight(domain, "."), ".")
	if len(parts) == 1 && domain == "localhost" { // "localhost" is a common valid case
		return true
	}
	// New check for single numeric label that is not localhost
	if len(parts) == 1 && domain != "localhost" {
		isNumericOnly := true
		if domain == "" { // Should have been caught by earlier check, but defensive
			isNumericOnly = false
		}
		for _, char := range domain { // Check the original domain string for this single part
			if char < '0' || char > '9' {
				isNumericOnly = false
				break
			}
		}
		if isNumericOnly {
			return false // Purely numeric single label (not localhost) is invalid
		}
	}

	if len(parts) > 1 {
		tld := parts[len(parts)-1]
		if tld == "" && len(parts) > 1 && strings.HasSuffix(domain, ".") { // trailing dot case, tld becomes empty
			// for "example.com.", parts are ["example", "com"], tld is "com"
			// for "example.com..", regex would fail.
			// for "com.", parts is ["com"], tld is "com"
			// This case is mostly to prevent "myhost.123" from passing if "123" is the TLD.
		} else if tld == "" { // Should be caught by regex if domain is not just "."
			return false
		}

		isNumericTld := true
		for _, char := range tld {
			if char < '0' || char > '9' {
				isNumericTld = false
				break
			}
		}
		if isNumericTld {
			return false // Numeric TLDs are generally invalid
		}
	}

	// Final check on each part for length and hyphen rules (largely covered by regex but belt-and-suspenders)
	for _, part := range parts {
		if len(part) == 0 { // Caused by ".." or leading/trailing "." not handled by TrimRight for FQDN root
			return false
		}
		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
		if len(part) > 63 {
			return false
		}
	}

	return true
}

// IsValidPort checks if a string represents a valid port number (1-65535).
func IsValidPort(portStr string) bool {
	if portStr == "" {
		return false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return port >= 1 && port <= 65535
}

// ValidateHostPortString checks if the string is a valid "host", "host:port", "[ipv6]:port", or "ipv6".
// It uses IsValidIP, IsValidDomainName, and IsValidPort internally.
func ValidateHostPortString(hp string) bool {
	hp = strings.TrimSpace(hp)
	if hp == "" {
		return false
	}

	host, port, err := net.SplitHostPort(hp)
	if err == nil {
		// Successfully split, means it's host:port or [ipv6]:port
		// net.SplitHostPort already unwraps IPv6 from brackets.
		if !(IsValidIP(host) || IsValidDomainName(host)) {
			return false
		}
		return IsValidPort(port)
	} else {
		// net.SplitHostPort failed. This means input string is not in host:port form.
		// So, the entire string must be a valid IP address or a valid domain name.

		// Handle bracketed IPv6 without port, e.g., "[::1]"
		if strings.HasPrefix(hp, "[") && strings.HasSuffix(hp, "]") {
			unwrappedHost := hp[1 : len(hp)-1]
			return IsValidIP(unwrappedHost) // Check if the unwrapped part is a valid IP
		}

		// For unbracketed strings, check if it's a valid IP or domain name.
		// An unbracketed IPv6 with a port-like suffix (e.g. "::1:8080") will fail IsValidIP
		// and should also fail IsValidDomainName.
		return IsValidIP(hp) || IsValidDomainName(hp)
	}
}

// --- End Validation Helpers ---

// ContainsString checks if a string is present in a slice of strings.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// Uint32Ptr returns a pointer to the uint32 value u.
func Uint32Ptr(u uint32) *uint32 {
	return &u
}

// Uint64Ptr returns a pointer to the uint64 value u.
func Uint64Ptr(u uint64) *uint64 {
	return &u
}

// Int32Ptr returns a pointer to the int32 value i.
func Int32Ptr(i int32) *int32 {
	return &i
}

// NetworksOverlap checks if two IP networks overlap.
// TODO: Consider enhancing this for more precise overlap detection,
// e.g., by comparing start and end IPs of each range.
// The current method (n1.Contains(n2.IP) || n2.Contains(n1.IP))
// correctly identifies if one network's starting IP is within the other,
// which covers most common overlap scenarios between a network and a subnetwork,
// or identical networks. It might not catch all complex partial overlaps
// between arbitrary, distinct CIDRs that don't fully contain each other's starting IPs
// but still share some address space. However, for typical pod/service CIDR validation,
// this level of check is often sufficient.
func NetworksOverlap(n1, n2 *net.IPNet) bool {
	if n1 == nil || n2 == nil {
		return false // Cannot overlap if one is nil
	}
	// Check if one network contains the other's network address.
	// Also check if the network masks are valid because IPNet an represent a single IP address (/32 for IPv4 or /128 for IPv6)
	// or a network. The IP field is the network address.
	return (n1.Contains(n2.IP) && n2.Mask != nil) || (n2.Contains(n1.IP) && n1.Mask != nil)
}

// EnsureExtraArgs ensures that a list of default arguments are present in the current arguments,
// unless an argument with the same prefix (e.g., "--audit-log-path=") already exists.
// defaultArgs should be a map where keys are the full default argument strings (e.g., "--profiling=false").
func EnsureExtraArgs(currentArgs []string, defaultArgs map[string]string) []string {
	if currentArgs == nil {
		currentArgs = []string{}
	}

	existingArgPrefixes := make(map[string]bool)
	for _, arg := range currentArgs {
		parts := strings.SplitN(arg, "=", 2)
		existingArgPrefixes[parts[0]] = true
	}

	finalArgs := make([]string, len(currentArgs))
	copy(finalArgs, currentArgs)

	for defaultArgKey, defaultArgValue := range defaultArgs { // defaultArgKey is the prefix, defaultArgValue is the full string like "--prefix=value"
		prefix := defaultArgKey
		if _, exists := existingArgPrefixes[prefix]; !exists {
			finalArgs = append(finalArgs, defaultArgValue)
		}
	}
	return finalArgs
}

// ExpandHostRange expands a hostname pattern with a range into a list of hostnames.
// Examples:
//   "node[1:3]" -> ["node1", "node2", "node3"]
//   "node[01:03]" -> ["node01", "node02", "node03"]
//   "node1" -> ["node1"]
// Returns an error if the pattern is invalid.
func ExpandHostRange(pattern string) ([]string, error) {
	re := regexp.MustCompile(`^(.*)\[([0-9]+):([0-9]+)\](.*)$`)
	matches := re.FindStringSubmatch(pattern)

	if len(matches) == 0 {
		// No range pattern, return the pattern itself as a single host
		if strings.TrimSpace(pattern) == "" {
			return nil, errors.New("host pattern cannot be empty")
		}
		return []string{pattern}, nil
	}

	prefix := matches[1]
	startStr := matches[2]
	endStr := matches[3]
	suffix := matches[4]

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start range in pattern '%s': %w", pattern, err)
	}
	end, err := strconv.Atoi(endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end range in pattern '%s': %w", pattern, err)
	}

	if start > end {
		return nil, fmt.Errorf("start range cannot be greater than end range in pattern '%s'", pattern)
	}

	var hostnames []string
	formatStr := "%s%0" + fmt.Sprintf("%dd", len(startStr)) + "%s"
	if len(startStr) == 1 || (len(startStr) > 1 && startStr[0] != '0') { // No leading zero or not intended for padding
		formatStr = "%s%d%s"
	}


	for i := start; i <= end; i++ {
		hostnames = append(hostnames, fmt.Sprintf(formatStr, prefix, i, suffix))
	}

	if len(hostnames) == 0 { // Should not happen if start <= end, but as a safeguard
		return nil, fmt.Errorf("expanded to zero hostnames for pattern '%s', check range", pattern)
	}

	return hostnames, nil
}

// NonEmptyNodeIDs filters a list of NodeIDs, returning only those that are not empty strings.
// NonEmptyNodeIDs was moved to pkg/plan/utils.go to break an import cycle.
