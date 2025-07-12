package util

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
)

// TaskUtils provides utility functions for working with tasks
type TaskUtils struct{}

// NewTaskUtils creates a new task utilities instance
func NewTaskUtils() *TaskUtils {
	return &TaskUtils{}
}

// GetTaskTypeName returns the type name of a task interface
func (tu *TaskUtils) GetTaskTypeName(task interface{}) string {
	taskType := reflect.TypeOf(task)
	if taskType.Kind() == reflect.Ptr {
		taskType = taskType.Elem()
	}
	return taskType.Name()
}

// ValidateTaskName checks if a task name is valid according to kubexm conventions
func (tu *TaskUtils) ValidateTaskName(name string) error {
	if name == "" {
		return fmt.Errorf("task name cannot be empty")
	}
	
	if len(name) > common.MaxTaskNameLength {
		return fmt.Errorf("task name cannot exceed %d characters", common.MaxTaskNameLength)
	}
	
	// Check for invalid characters
	for _, char := range name {
		found := false
		for _, validChar := range common.TaskNameValidCharacters {
			if char == validChar {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("task name contains invalid character: %c", char)
		}
	}
	
	// Cannot start or end with special characters
	for _, invalidChar := range common.TaskNameInvalidStartEndChars {
		if strings.HasPrefix(name, string(invalidChar)) || strings.HasSuffix(name, string(invalidChar)) {
			return fmt.Errorf("task name cannot start or end with special characters (%s)", common.TaskNameInvalidStartEndChars)
		}
	}
	
	return nil
}

// EstimateTotalExecutionTime estimates the total execution time for task operations
// This is a utility function that can be used by schedulers and planners
func (tu *TaskUtils) EstimateTotalExecutionTime(numTasks int, avgTaskDuration time.Duration) time.Duration {
	if numTasks <= 0 {
		return 0
	}
	
	// Simple linear estimation - in practice this could be more sophisticated
	// considering parallel execution capabilities
	return time.Duration(numTasks) * avgTaskDuration
}

// CalculateResourceRequirement calculates combined resource requirements
// This is used for resource planning and validation
func (tu *TaskUtils) CalculateResourceRequirement(requirements []map[string]interface{}) map[string]interface{} {
	combined := make(map[string]interface{})
	
	var totalMemoryMB, totalDiskMB, totalNetworkMBps int64
	var maxCPUPercent int
	var maxConcurrency int
	
	for _, req := range requirements {
		if memMB, ok := req["memory_mb"].(int64); ok {
			totalMemoryMB += memMB
		}
		if diskMB, ok := req["disk_mb"].(int64); ok {
			totalDiskMB += diskMB
		}
		if netMBps, ok := req["network_mbps"].(int64); ok {
			totalNetworkMBps += netMBps
		}
		if cpuPercent, ok := req["cpu_percent"].(int); ok && cpuPercent > maxCPUPercent {
			maxCPUPercent = cpuPercent
		}
		if concurrency, ok := req["max_concurrency"].(int); ok {
			maxConcurrency += concurrency
		}
	}
	
	combined["memory_mb"] = totalMemoryMB
	combined["disk_mb"] = totalDiskMB
	combined["network_mbps"] = totalNetworkMBps
	combined["cpu_percent"] = maxCPUPercent
	combined["max_concurrency"] = maxConcurrency
	
	return combined
}

// FilterStringSlice filters a string slice based on a predicate function
func (tu *TaskUtils) FilterStringSlice(slice []string, predicate func(string) bool) []string {
	var result []string
	for _, item := range slice {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

// ContainsString checks if a string is present in a slice
func (tu *TaskUtils) ContainsString(slice []string, target string) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}

// UniqueStrings returns a slice with duplicate strings removed
func (tu *TaskUtils) UniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// SanitizeTaskName sanitizes a task name to make it valid
func (tu *TaskUtils) SanitizeTaskName(name string) string {
	if name == "" {
		return "unnamed-task"
	}
	
	// Replace invalid characters with dashes
	var sanitized strings.Builder
	for _, char := range name {
		found := false
		for _, validChar := range common.TaskNameValidCharacters {
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
	result = strings.Trim(result, common.TaskNameInvalidStartEndChars)
	
	// Ensure it's not empty after sanitization
	if result == "" {
		result = "sanitized-task"
	}
	
	// Truncate if too long
	if len(result) > common.MaxTaskNameLength {
		result = result[:common.MaxTaskNameLength]
		result = strings.Trim(result, common.TaskNameInvalidStartEndChars)
	}
	
	return result
}

// ParseDurationOrDefault parses a duration string or returns a default value
func (tu *TaskUtils) ParseDurationOrDefault(durationStr string, defaultDuration time.Duration) time.Duration {
	if durationStr == "" {
		return defaultDuration
	}
	
	if duration, err := time.ParseDuration(durationStr); err == nil {
		return duration
	}
	
	return defaultDuration
}

// Global instance for convenience
var TaskUtilities = NewTaskUtils()