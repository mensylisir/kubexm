package runtime

import (
	"context"
	"fmt"
	"os"

	// rtcontext "github.com/mensylisir/kubexm/pkg/runtime/context" // Alias for the new context
	// The above line was commented out in my previous attempt, it should be active.
	// Let's ensure the alias is correctly defined and used.
	// The type *rtcontext.Context is already used in NewRuntimeFromYAML signature in the current file content,
	// so the import must exist. It might be that the `read_files` output is stale or my previous diff
	// already added it but I missed it in my reasoning.
	// I will construct the import block as it *should* be.

	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner" // Kept for runnerNewRunner
	"{{MODULE_NAME}}/pkg/parser"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	rtcontext "github.com/mensylisir/kubexm/pkg/runtime/context" // Alias for the new context
)

// osReadFile is a variable that defaults to os.ReadFile, allowing it to be mocked for tests.
var osReadFile = os.ReadFile

// NewRuntimeFromYAML creates a new runtime Context from YAML data using RuntimeBuilder.
// It returns the main runtime Context (from pkg/runtime/context.go), a cleanup function, and an error if initialization fails.
// Note: The return type *rtcontext.Context here refers to pkg/runtime/context.Context.
func NewRuntimeFromYAML(yamlData []byte, baseLogger *logger.Logger) (*rtcontext.Context, func(), error) {
	if baseLogger == nil {
		baseLogger = logger.Get() // Fallback to global default logger
	}
	// Ensure the logger passed to the builder carries the "phase" information
	initPhaseLogger := baseLogger.With("phase", "runtime_init_from_yaml")

	parsedConfig, err := parser.ParseClusterYAML(yamlData)
	if err != nil {
		initPhaseLogger.Error(err, "Failed to parse cluster YAML")
		return nil, nil, fmt.Errorf("failed to parse cluster YAML: %w", err)
	}

	if parsedConfig == nil {
		// This error needs to be created before being passed to the logger
		err := fmt.Errorf("parsed configuration is nil after parsing YAML")
		initPhaseLogger.Error(err, "Parsed configuration is nil")
		return nil, nil, err
	}

	initPhaseLogger.Info("Successfully parsed cluster YAML", "clusterName", parsedConfig.ObjectMeta.Name)

	v1alpha1.SetDefaults_Cluster(parsedConfig)
	initPhaseLogger.Info("Applied v1alpha1 defaults to the cluster configuration", "clusterName", parsedConfig.ObjectMeta.Name)

	// Use an empty RuntimeBuilder struct.
	// The configFile field within RuntimeBuilder is not used by BuildFromConfig.
	builder := RuntimeBuilder{}

	// Create a background context for the builder.
	// Callers who need specific context (e.g., for cancellation) should manage it before calling NewRuntimeFromYAML,
	// or NewRuntimeFromYAML's signature would need to change to accept a context.
	buildCtx := context.Background()

	// Pass initPhaseLogger as the baseLogger to BuildFromConfig
	rtCtx, cleanup, err := builder.BuildFromConfig(buildCtx, parsedConfig, initPhaseLogger)
	if err != nil {
		return nil, nil, err // Propagate error from BuildFromConfig
	}
	return rtCtx, cleanup, nil
}

// runnerNewRunner allows runner.NewRunner to be replaced for testing.
// This variable is kept if builder.go or other non-deprecated parts use it.
var runnerNewRunner = runner.NewRunner
