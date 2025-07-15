package config

import (
	"errors"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"regexp"
	"strconv"
	"strings"
)

func ValidateRoleGroupHosts(roleGroups *v1alpha1.RoleGroupsSpec, hosts []v1alpha1.HostSpec) error {
	validHosts := make(map[string]struct{})
	for _, host := range hosts {
		validHosts[host.Name] = struct{}{}
	}

	allHosts := []string{}
	allHosts = append(allHosts, roleGroups.Master...)
	allHosts = append(allHosts, roleGroups.Worker...)
	allHosts = append(allHosts, roleGroups.Etcd...)
	allHosts = append(allHosts, roleGroups.LoadBalancer...)
	allHosts = append(allHosts, roleGroups.Storage...)
	allHosts = append(allHosts, roleGroups.Registry...)

	for _, hostName := range allHosts {
		if _, ok := validHosts[hostName]; !ok {
			return fmt.Errorf("host '%s' from an expanded roleGroup is not defined in spec.hosts", hostName)
		}
	}
	return nil
}

func ExpandRoleGroupHosts(hosts []string) ([]string, error) {
	if hosts == nil {
		return nil, nil
	}
	expanded := make([]string, 0, len(hosts))
	for _, h := range hosts {
		currentHosts, err := ExpandHostRange(h)
		if err != nil {
			return nil, fmt.Errorf("error expanding host range '%s': %w", h, err)
		}
		expanded = append(expanded, currentHosts...)
	}
	return expanded, nil
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
