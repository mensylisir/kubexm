package util

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
)

// ModuleUtils provides utility functions for working with modules
type ModuleUtils struct{}

// NewModuleUtils creates a new module utilities instance
func NewModuleUtils() *ModuleUtils {
	return &ModuleUtils{}
}

// ValidateModuleName checks if a module name is valid according to kubexm conventions
func (mu *ModuleUtils) ValidateModuleName(name string) error {
	if name == "" {
		return fmt.Errorf("module name cannot be empty")
	}
	
	if len(name) > common.MaxModuleNameLength {
		return fmt.Errorf("module name cannot exceed %d characters", common.MaxModuleNameLength)
	}
	
	// Check for invalid characters
	for _, char := range name {
		found := false
		for _, validChar := range common.ModuleNameValidCharacters {
			if char == validChar {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("module name contains invalid character: %c", char)
		}
	}
	
	// Cannot start or end with special characters
	for _, invalidChar := range common.ModuleNameInvalidStartEndChars {
		if strings.HasPrefix(name, string(invalidChar)) || strings.HasSuffix(name, string(invalidChar)) {
			return fmt.Errorf("module name cannot start or end with special characters (%s)", common.ModuleNameInvalidStartEndChars)
		}
	}
	
	return nil
}

// SanitizeModuleName sanitizes a module name to make it valid
func (mu *ModuleUtils) SanitizeModuleName(name string) string {
	if name == "" {
		return "unnamed-module"
	}
	
	// Replace invalid characters with dashes
	var sanitized strings.Builder
	for _, char := range name {
		found := false
		for _, validChar := range common.ModuleNameValidCharacters {
			if char == validChar {
				found = true
				break
			}
		}
		if found {
			sanitized.WriteRune(char)
		} else {
			sanitized.WriteRune('-')
		}
	}
	
	result := sanitized.String()
	
	// Remove leading and trailing invalid characters
	result = strings.Trim(result, common.ModuleNameInvalidStartEndChars)
	
	// Ensure it's not empty after sanitization
	if result == "" {
		result = "sanitized-module"
	}
	
	// Truncate if too long
	if len(result) > common.MaxModuleNameLength {
		result = result[:common.MaxModuleNameLength]
		result = strings.Trim(result, common.ModuleNameInvalidStartEndChars)
	}
	
	return result
}

// ValidateDependencyChain validates that module dependencies don't create cycles
func (mu *ModuleUtils) ValidateDependencyChain(modules map[string][]string) error {
	// Check for cycles using depth-first search
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)
	
	var hasCycle func(string) bool
	hasCycle = func(module string) bool {
		visited[module] = true
		recursionStack[module] = true
		
		for _, dep := range modules[module] {
			if !visited[dep] {
				if hasCycle(dep) {
					return true
				}
			} else if recursionStack[dep] {
				return true
			}
		}
		
		recursionStack[module] = false
		return false
	}
	
	for module := range modules {
		if !visited[module] {
			if hasCycle(module) {
				return fmt.Errorf("circular dependency detected involving module '%s'", module)
			}
		}
	}
	
	return nil
}

// TopologicalSort returns modules sorted by their dependencies
func (mu *ModuleUtils) TopologicalSort(modules map[string][]string) ([]string, error) {
	// First validate no cycles exist
	if err := mu.ValidateDependencyChain(modules); err != nil {
		return nil, err
	}
	
	// Kahn's algorithm for topological sort
	inDegree := make(map[string]int)
	
	// Initialize in-degree count
	for module := range modules {
		inDegree[module] = 0
	}
	
	// Calculate in-degrees
	for _, deps := range modules {
		for _, dep := range deps {
			inDegree[dep]++
		}
	}
	
	// Find modules with no incoming dependencies
	var queue []string
	for module, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, module)
		}
	}
	
	var result []string
	
	for len(queue) > 0 {
		// Remove module from queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		
		// Reduce in-degree for dependent modules
		for _, dep := range modules[current] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}
	
	// Check if all modules were processed
	if len(result) != len(modules) {
		return nil, fmt.Errorf("topological sort failed - possible circular dependency")
	}
	
	return result, nil
}

// GroupModulesByPhase groups modules by their execution phase
func (mu *ModuleUtils) GroupModulesByPhase(modules map[string]string) map[string][]string {
	phaseGroups := make(map[string][]string)
	
	for module, phase := range modules {
		if phase == "" {
			phase = common.ModulePhaseKubernetes // Default phase
		}
		phaseGroups[phase] = append(phaseGroups[phase], module)
	}
	
	// Sort modules within each phase
	for phase := range phaseGroups {
		sort.Strings(phaseGroups[phase])
	}
	
	return phaseGroups
}

// EstimateTotalModuleExecutionTime estimates total execution time for modules
func (mu *ModuleUtils) EstimateTotalModuleExecutionTime(moduleCount int, avgModuleDuration time.Duration, parallelism int) time.Duration {
	if moduleCount <= 0 || parallelism <= 0 {
		return 0
	}
	
	// Simple parallel execution estimation
	if parallelism >= moduleCount {
		return avgModuleDuration // All modules can run in parallel
	}
	
	// Calculate batches
	batches := (moduleCount + parallelism - 1) / parallelism
	return time.Duration(batches) * avgModuleDuration
}

// FilterModulesByTag filters modules based on tag matching
func (mu *ModuleUtils) FilterModulesByTag(moduleNames []string, tagFilters map[string]string, moduleTagsGetter func(string) map[string]string) []string {
	var filtered []string
	
	for _, moduleName := range moduleNames {
		tags := moduleTagsGetter(moduleName)
		matches := true
		
		for filterKey, filterValue := range tagFilters {
			if tags[filterKey] != filterValue {
				matches = false
				break
			}
		}
		
		if matches {
			filtered = append(filtered, moduleName)
		}
	}
	
	return filtered
}

// CalculateModulePath finds the execution path between two modules
func (mu *ModuleUtils) CalculateModulePath(dependencies map[string][]string, start, end string) ([]string, error) {
	if start == end {
		return []string{start}, nil
	}
	
	// BFS to find shortest path
	queue := [][]string{{start}}
	visited := make(map[string]bool)
	visited[start] = true
	
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		current := path[len(path)-1]
		
		// Check all dependencies of current module
		for _, dep := range dependencies[current] {
			if dep == end {
				return append(path, dep), nil
			}
			
			if !visited[dep] {
				visited[dep] = true
				newPath := append([]string{}, path...)
				newPath = append(newPath, dep)
				queue = append(queue, newPath)
			}
		}
	}
	
	return nil, fmt.Errorf("no path found from module '%s' to module '%s'", start, end)
}

// ValidateModulePhase checks if a module phase is valid
func (mu *ModuleUtils) ValidateModulePhase(phase string) error {
	validPhases := []string{
		common.ModulePhaseInfrastructure,
		common.ModulePhasePreflight,
		common.ModulePhaseRuntime,
		common.ModulePhaseKubernetes,
		common.ModulePhaseNetwork,
		common.ModulePhaseAddons,
		common.ModulePhaseCleanup,
	}
	
	for _, validPhase := range validPhases {
		if phase == validPhase {
			return nil
		}
	}
	
	return fmt.Errorf("invalid module phase '%s', must be one of: %s", phase, strings.Join(validPhases, ", "))
}

// GetPhaseOrder returns the standard execution order of module phases
func (mu *ModuleUtils) GetPhaseOrder() []string {
	return []string{
		common.ModulePhaseInfrastructure,
		common.ModulePhasePreflight,
		common.ModulePhaseRuntime,
		common.ModulePhaseKubernetes,
		common.ModulePhaseNetwork,
		common.ModulePhaseAddons,
		common.ModulePhaseCleanup,
	}
}

// Global instance for convenience
var ModuleUtilities = NewModuleUtils()