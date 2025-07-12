package util

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
)

// PipelineUtils provides utility functions for working with pipelines
type PipelineUtils struct{}

// NewPipelineUtils creates a new pipeline utilities instance
func NewPipelineUtils() *PipelineUtils {
	return &PipelineUtils{}
}

// ValidatePipelineName checks if a pipeline name is valid according to kubexm conventions
func (pu *PipelineUtils) ValidatePipelineName(name string) error {
	if name == "" {
		return fmt.Errorf("pipeline name cannot be empty")
	}
	
	if len(name) > common.MaxPipelineNameLength {
		return fmt.Errorf("pipeline name cannot exceed %d characters", common.MaxPipelineNameLength)
	}
	
	// Check for invalid characters
	for _, char := range name {
		found := false
		for _, validChar := range common.PipelineNameValidCharacters {
			if char == validChar {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("pipeline name contains invalid character: %c", char)
		}
	}
	
	// Cannot start or end with special characters
	for _, invalidChar := range common.PipelineNameInvalidStartEndChars {
		if strings.HasPrefix(name, string(invalidChar)) || strings.HasSuffix(name, string(invalidChar)) {
			return fmt.Errorf("pipeline name cannot start or end with special characters (%s)", common.PipelineNameInvalidStartEndChars)
		}
	}
	
	return nil
}

// SanitizePipelineName sanitizes a pipeline name to make it valid
func (pu *PipelineUtils) SanitizePipelineName(name string) string {
	if name == "" {
		return "unnamed-pipeline"
	}
	
	// Replace invalid characters with dashes
	var sanitized strings.Builder
	for _, char := range name {
		found := false
		for _, validChar := range common.PipelineNameValidCharacters {
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
	result = strings.Trim(result, common.PipelineNameInvalidStartEndChars)
	
	// Ensure it's not empty after sanitization
	if result == "" {
		result = "sanitized-pipeline"
	}
	
	// Truncate if too long
	if len(result) > common.MaxPipelineNameLength {
		result = result[:common.MaxPipelineNameLength]
		result = strings.Trim(result, common.PipelineNameInvalidStartEndChars)
	}
	
	return result
}

// ValidatePipelineType checks if a pipeline type is valid
func (pu *PipelineUtils) ValidatePipelineType(pipelineType string) error {
	validTypes := []string{
		common.PipelineTypeClusterCreate,
		common.PipelineTypeClusterDelete,
		common.PipelineTypeClusterUpgrade,
		common.PipelineTypeClusterScale,
		common.PipelineTypeNodeAdd,
		common.PipelineTypeNodeRemove,
		common.PipelineTypeMaintenance,
		common.PipelineTypeBackup,
		common.PipelineTypeRestore,
	}
	
	for _, validType := range validTypes {
		if pipelineType == validType {
			return nil
		}
	}
	
	return fmt.Errorf("invalid pipeline type '%s', must be one of: %s", pipelineType, strings.Join(validTypes, ", "))
}

// ValidateExecutionStrategy checks if an execution strategy is valid
func (pu *PipelineUtils) ValidateExecutionStrategy(strategy string) error {
	validStrategies := []string{
		common.PipelineExecutionSequential,
		common.PipelineExecutionParallel,
		common.PipelineExecutionConditional,
		common.PipelineExecutionPhased,
	}
	
	for _, validStrategy := range validStrategies {
		if strategy == validStrategy {
			return nil
		}
	}
	
	return fmt.Errorf("invalid execution strategy '%s', must be one of: %s", strategy, strings.Join(validStrategies, ", "))
}

// ValidatePipelineDependencies validates that pipeline dependencies don't create cycles
func (pu *PipelineUtils) ValidatePipelineDependencies(pipelines map[string][]string) error {
	// Check for cycles using depth-first search
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)
	
	var hasCycle func(string) bool
	hasCycle = func(pipeline string) bool {
		visited[pipeline] = true
		recursionStack[pipeline] = true
		
		for _, dep := range pipelines[pipeline] {
			if !visited[dep] {
				if hasCycle(dep) {
					return true
				}
			} else if recursionStack[dep] {
				return true
			}
		}
		
		recursionStack[pipeline] = false
		return false
	}
	
	for pipeline := range pipelines {
		if !visited[pipeline] {
			if hasCycle(pipeline) {
				return fmt.Errorf("circular dependency detected involving pipeline '%s'", pipeline)
			}
		}
	}
	
	// Check dependency depth
	maxDepth := pu.calculateMaxDependencyDepth(pipelines)
	if maxDepth > common.MaxPipelineDependencyDepth {
		return fmt.Errorf("dependency depth %d exceeds maximum allowed depth %d", maxDepth, common.MaxPipelineDependencyDepth)
	}
	
	return nil
}

// calculateMaxDependencyDepth calculates the maximum dependency depth
func (pu *PipelineUtils) calculateMaxDependencyDepth(pipelines map[string][]string) int {
	depths := make(map[string]int)
	
	var calculateDepth func(string) int
	calculateDepth = func(pipeline string) int {
		if depth, exists := depths[pipeline]; exists {
			return depth
		}
		
		maxDepth := 0
		for _, dep := range pipelines[pipeline] {
			depDepth := calculateDepth(dep)
			if depDepth > maxDepth {
				maxDepth = depDepth
			}
		}
		
		depths[pipeline] = maxDepth + 1
		return depths[pipeline]
	}
	
	maxOverallDepth := 0
	for pipeline := range pipelines {
		depth := calculateDepth(pipeline)
		if depth > maxOverallDepth {
			maxOverallDepth = depth
		}
	}
	
	return maxOverallDepth
}

// TopologicalSortPipelines returns pipelines sorted by their dependencies
func (pu *PipelineUtils) TopologicalSortPipelines(pipelines map[string][]string) ([]string, error) {
	// First validate no cycles exist
	if err := pu.ValidatePipelineDependencies(pipelines); err != nil {
		return nil, err
	}
	
	// Kahn's algorithm for topological sort
	inDegree := make(map[string]int)
	
	// Initialize in-degree count
	for pipeline := range pipelines {
		inDegree[pipeline] = 0
	}
	
	// Calculate in-degrees
	for _, deps := range pipelines {
		for _, dep := range deps {
			inDegree[dep]++
		}
	}
	
	// Find pipelines with no incoming dependencies
	var queue []string
	for pipeline, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, pipeline)
		}
	}
	
	var result []string
	
	for len(queue) > 0 {
		// Remove pipeline from queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)
		
		// Reduce in-degree for dependent pipelines
		for _, dep := range pipelines[current] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}
	
	// Check if all pipelines were processed
	if len(result) != len(pipelines) {
		return nil, fmt.Errorf("topological sort failed - possible circular dependency")
	}
	
	return result, nil
}

// GroupPipelinesByType groups pipelines by their type
func (pu *PipelineUtils) GroupPipelinesByType(pipelines map[string]string) map[string][]string {
	typeGroups := make(map[string][]string)
	
	for pipeline, pipelineType := range pipelines {
		if pipelineType == "" {
			pipelineType = common.PipelineTypeClusterCreate // Default type
		}
		typeGroups[pipelineType] = append(typeGroups[pipelineType], pipeline)
	}
	
	// Sort pipelines within each type
	for pipelineType := range typeGroups {
		sort.Strings(typeGroups[pipelineType])
	}
	
	return typeGroups
}

// EstimateTotalPipelineExecutionTime estimates total execution time for pipelines
func (pu *PipelineUtils) EstimateTotalPipelineExecutionTime(pipelineCount int, avgPipelineDuration time.Duration, parallelism int) time.Duration {
	if pipelineCount <= 0 || parallelism <= 0 {
		return 0
	}
	
	// Simple parallel execution estimation
	if parallelism >= pipelineCount {
		return avgPipelineDuration // All pipelines can run in parallel
	}
	
	// Calculate batches
	batches := (pipelineCount + parallelism - 1) / parallelism
	return time.Duration(batches) * avgPipelineDuration
}

// FilterPipelinesByTag filters pipelines based on tag matching
func (pu *PipelineUtils) FilterPipelinesByTag(pipelineNames []string, tagFilters map[string]string, pipelineTagsGetter func(string) map[string]string) []string {
	var filtered []string
	
	for _, pipelineName := range pipelineNames {
		tags := pipelineTagsGetter(pipelineName)
		matches := true
		
		for filterKey, filterValue := range tagFilters {
			if tags[filterKey] != filterValue {
				matches = false
				break
			}
		}
		
		if matches {
			filtered = append(filtered, pipelineName)
		}
	}
	
	return filtered
}

// CalculatePipelinePath finds the execution path between two pipelines
func (pu *PipelineUtils) CalculatePipelinePath(dependencies map[string][]string, start, end string) ([]string, error) {
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
		
		// Check all dependencies of current pipeline
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
	
	return nil, fmt.Errorf("no path found from pipeline '%s' to pipeline '%s'", start, end)
}

// ValidatePipelineConfiguration validates a pipeline configuration map
func (pu *PipelineUtils) ValidatePipelineConfiguration(config map[string]interface{}) error {
	// Validate common configuration keys
	if timeout, exists := config[common.PipelineConfigTimeout]; exists {
		if timeoutStr, ok := timeout.(string); ok {
			if _, err := time.ParseDuration(timeoutStr); err != nil {
				return fmt.Errorf("invalid timeout value '%s': %w", timeoutStr, err)
			}
		} else {
			return fmt.Errorf("timeout must be a string duration")
		}
	}
	
	if maxConcurrency, exists := config[common.PipelineConfigMaxConcurrency]; exists {
		if concurrency, ok := maxConcurrency.(int); ok {
			if concurrency <= 0 {
				return fmt.Errorf("max concurrency must be positive")
			}
		} else {
			return fmt.Errorf("max concurrency must be an integer")
		}
	}
	
	if retryAttempts, exists := config[common.PipelineConfigRetryAttempts]; exists {
		if attempts, ok := retryAttempts.(int); ok {
			if attempts < 0 {
				return fmt.Errorf("retry attempts cannot be negative")
			}
		} else {
			return fmt.Errorf("retry attempts must be an integer")
		}
	}
	
	return nil
}

// GetPipelineTypeOrder returns the standard execution order of pipeline types
func (pu *PipelineUtils) GetPipelineTypeOrder() []string {
	return []string{
		common.PipelineTypeClusterCreate,
		common.PipelineTypeNodeAdd,
		common.PipelineTypeClusterScale,
		common.PipelineTypeClusterUpgrade,
		common.PipelineTypeMaintenance,
		common.PipelineTypeBackup,
		common.PipelineTypeRestore,
		common.PipelineTypeNodeRemove,
		common.PipelineTypeClusterDelete,
	}
}

// Global instance for convenience
var PipelineUtilities = NewPipelineUtils()