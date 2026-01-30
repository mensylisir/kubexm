package tool

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ParseCPU parses a CPU string (e.g., "100m", "1", "2Gi") into a resource.Quantity
func ParseCPU(cpuStr string) (*resource.Quantity, error) {
	s := strings.TrimSpace(cpuStr)
	if s == "" {
		return nil, fmt.Errorf("cpu string cannot be empty")
	}
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cpu string '%s': %w", cpuStr, err)
	}
	return &q, nil
}

// ParseMemory parses a memory string (e.g., "1Gi", "512Mi", "1G") into a resource.Quantity
func ParseMemory(memStr string) (*resource.Quantity, error) {
	return parseResourceWithUnitConversion("memory", memStr)
}

// ParseStorage parses a storage string into a resource.Quantity
func ParseStorage(storageStr string) (*resource.Quantity, error) {
	return parseResourceWithUnitConversion("storage", storageStr)
}

func parseResourceWithUnitConversion(resourceType, valueStr string) (*resource.Quantity, error) {
	s := strings.TrimSpace(valueStr)
	if s == "" {
		return nil, fmt.Errorf("%s string cannot be empty", resourceType)
	}
	toParse := s
	for hostUnit, k8sUnit := range common.HostUnitMap {
		if strings.HasSuffix(strings.ToUpper(s), strings.ToUpper(hostUnit)) {
			numPart := s[:len(s)-len(hostUnit)]

			if _, err := strconv.ParseFloat(numPart, 64); err == nil {
				toParse = numPart + k8sUnit
				break
			}
		}
	}
	q, err := resource.ParseQuantity(toParse)
	if err != nil {
		if toParse != s {
			return nil, fmt.Errorf("failed to parse %s string '%s' (normalized to '%s'): %w", resourceType, valueStr, toParse, err)
		}
		return nil, fmt.Errorf("failed to parse %s string '%s': %w", resourceType, valueStr, err)
	}
	return &q, nil
}
