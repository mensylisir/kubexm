package helpers

import (
	"fmt"
)

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
