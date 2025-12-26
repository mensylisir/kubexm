package util

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func ContainsStringWithEmpty(slice []string, s string) bool {
	if s == "" {
		return true
	}
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func ContainsInt(slice []int, item int) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

func NetworksOverlap(n1, n2 *net.IPNet) bool {
	if n1 == nil || n2 == nil {
		return false // Cannot overlap if one is nil
	}
	return (n1.Contains(n2.IP) && n2.Mask != nil) || (n2.Contains(n1.IP) && n1.Mask != nil)
}

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

	for defaultArgKey, defaultArgValue := range defaultArgs {
		prefix := defaultArgKey
		if _, exists := existingArgPrefixes[prefix]; !exists {
			finalArgs = append(finalArgs, defaultArgValue)
		}
	}
	return finalArgs
}

func ExpandHostRange(pattern string) ([]string, error) {
	re := regexp.MustCompile(`^(.*)\[([0-9]+):([0-9]+)\](.*)$`)
	matches := re.FindStringSubmatch(pattern)

	if len(matches) == 0 {
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
	if len(startStr) == 1 || (len(startStr) > 1 && startStr[0] != '0') {
		formatStr = "%s%d%s"
	}

	for i := start; i <= end; i++ {
		hostnames = append(hostnames, fmt.Sprintf(formatStr, prefix, i, suffix))
	}

	if len(hostnames) == 0 {
		return nil, fmt.Errorf("expanded to zero hostnames for pattern '%s', check range", pattern)
	}

	return hostnames, nil
}

func ParseCPU(cpuStr string) (int64, error) {
	if cpuStr == "" {
		return 0, fmt.Errorf("empty CPU string")
	}

	if strings.HasSuffix(cpuStr, "m") {
		milliStr := strings.TrimSuffix(cpuStr, "m")
		milli, err := strconv.ParseFloat(milliStr, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse CPU millicores '%s': %w", cpuStr, err)
		}
		return int64(milli * 1000000), nil
	}

	cores, err := strconv.ParseFloat(cpuStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU cores '%s': %w", cpuStr, err)
	}

	return int64(cores * 1000000000), nil
}

func ParseMemory(memStr string) (int64, error) {
	if memStr == "" {
		return 0, fmt.Errorf("empty memory string")
	}

	units := map[string]int64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024,
		"Pi": 1024 * 1024 * 1024 * 1024 * 1024,
		"k":  1000,
		"M":  1000 * 1000,
		"G":  1000 * 1000 * 1000,
		"T":  1000 * 1000 * 1000 * 1000,
		"P":  1000 * 1000 * 1000 * 1000 * 1000,
	}

	for unit, multiplier := range units {
		if strings.HasSuffix(memStr, unit) {
			valueStr := strings.TrimSuffix(memStr, unit)
			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse memory value '%s': %w", memStr, err)
			}
			return int64(value * float64(multiplier)), nil
		}
	}

	bytes, err := strconv.ParseInt(memStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory bytes '%s': %w", memStr, err)
	}

	return bytes, nil
}

func UniqueStrings(input []string) []string {
	seen := make(map[string]struct{})
	result := []string{}

	for _, item := range input {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func FirstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func MergeStringMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func MergeBoolMaps(maps ...map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func UniqueStringSlice(input []string) []string {
	if input == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))
	for _, item := range input {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func ParseCaCertHashFromOutput(output string) (string, error) {
	re := regexp.MustCompile(`--discovery-token-ca-cert-hash\s+sha256:([a-f0-9]{64})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", fmt.Errorf("ca cert hash (sha256:...) not found in kubeadm output")
	}
	return matches[1], nil
}

func ParseTokenFromOutput(output string) (string, error) {
	re := regexp.MustCompile(`--token\s+([a-z0-9]{6}\.[a-z0-9]{16})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", fmt.Errorf("bootstrap token not found in kubeadm output")
	}
	return matches[1], nil
}

func ParseCertificateKeyFromOutput(output string) (string, error) {
	re := regexp.MustCompile(`--certificate-key\s+([a-f0-9]{64})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) != 2 {
		return "", fmt.Errorf("certificate key not found in kubeadm output, did you use --upload-certs?")
	}
	return matches[1], nil
}