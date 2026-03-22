package util

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hostRangeRegex = regexp.MustCompile(`^(.*)\[([0-9]+):([0-9]+)\](.*)$`)

func ExpandHostRange(pattern string) ([]string, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, errors.New("host pattern cannot be empty")
	}

	matches := hostRangeRegex.FindStringSubmatch(pattern)
	if len(matches) == 0 {
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
		return nil, fmt.Errorf("start range (%d) cannot be greater than end range (%d) in pattern '%s'", start, end, pattern)
	}

	count := end - start + 1
	hostnames := make([]string, 0, count)

	var formatStr string
	if len(startStr) > 1 && startStr[0] == '0' {
		formatStr = fmt.Sprintf("%%s%%0%dd%%s", len(startStr))
	} else {
		formatStr = "%s%d%s"
	}

	for i := start; i <= end; i++ {
		hostnames = append(hostnames, fmt.Sprintf(formatStr, prefix, i, suffix))
	}

	return hostnames, nil
}

func ExpandRoleGroupHosts(hosts []string) ([]string, error) {
	if hosts == nil {
		return nil, nil
	}
	if len(hosts) == 0 {
		return []string{}, nil
	}

	expanded := make([]string, 0, len(hosts))

	for _, h := range hosts {
		currentHosts, err := ExpandHostRange(h)
		if err != nil {
			return nil, fmt.Errorf("error expanding host pattern '%s': %w", h, err)
		}
		expanded = append(expanded, currentHosts...)
	}
	return expanded, nil
}
