package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log" // Standard log, consider replacing with custom logger if used elsewhere in main
	"os"
	"time" // For result.EndTime.Sub(result.StartTime)

	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/pkg/cluster" // For NewCreateClusterPipeline
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	// "github.com/mensylisir/kubexm/pkg/logger" // App's own logger if needed
)

var (
	configFile   string
	dryRun       bool
	logLevel     string // This would be for configuring the logger instance
	outputFormat string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kubexm", // Changed from kubexms to kubexm
	Short: "A tool for managing system configurations and deployments.",
	Long: `kubexm is a CLI tool that allows users to define system configurations
and execute deployment pipelines against various hosts.`,
}

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a configuration by running a deployment pipeline",
	Long: `The apply command initializes the runtime environment based on the provided
configuration file, then plans and executes a deployment pipeline.
It supports dry-run mode and various output formats for the execution result.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Consider setting up the application's own logger here based on logLevel
		// For example: appLogger := mylogger.New(logLevel)
		// appLogger.Info("kubexm apply command started")

		if err := runApply(); err != nil {
			// If runApply returns an error, it's already logged by the runtime logger.
			// Standard log.Fatalf will print to stderr and exit.
			log.Fatalf("FATAL: %v", err) // Using standard log for fatal errors
		}
		// appLogger.Info("kubexm apply command finished successfully")
	},
}

func init() {
	// Persistent flags, available to rootCmd and all its children
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/cluster.yaml", "Path to cluster configuration file")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (e.g., debug, info, warn, error) for the application's logger")

	// Flags specific to the apply command
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Execute in dry run mode without making any changes")
	applyCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "Output format for the result (text, json)")

	rootCmd.AddCommand(applyCmd)
}

func runApply() error {
	// Create a new runtime builder
	// The configFile variable is from the cobra flag
	builder := runtime.NewRuntimeBuilder(configFile)

	// Build the runtime context
	ctx, cleanup, err := builder.BuildFromFile(context.Background()) // Use BuildFromFile
	if err != nil {
		log.Printf("Error: Failed to build runtime environment: %v\n", err)
		return fmt.Errorf("failed to build runtime environment: %w", err)
	}
	defer cleanup()

	// Instantiate the CreateClusterPipeline
	// NewCreateClusterPipeline now takes *runtime.Context
	p := cluster.NewCreateClusterPipeline(ctx)

	ctx.Logger.Info("Starting pipeline execution...", "pipeline", p.Name(), "dryRun", dryRun, "configFile", configFile)

	// Run the pipeline. p.Run now returns *plan.GraphExecutionResult
	result, err := p.Run(ctx, dryRun)

	// Handle results and output formatting
	if outputFormat == "json" {
		jsonResult, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			ctx.Logger.Error(marshalErr, "Failed to marshal execution result to JSON")
			printResultAsText(ctx, result) // Attempt to print text if JSON fails
			return fmt.Errorf("failed to marshal result to JSON: %w", marshalErr)
		}
		fmt.Println(string(jsonResult))
	} else {
		printResultAsText(ctx, result)
	}

	if err != nil {
		ctx.Logger.Error(err, "Pipeline execution finished with errors")
		// err from p.Run might be an engine execution error, not a business logic failure.
		// The result.Status reflects business logic outcome.
		// If err is not nil, it's usually a more fundamental problem.
		return err
	}

	// Check the graph execution status
	if result != nil && result.Status == plan.StatusFailed {
		ctx.Logger.Info("Pipeline completed with overall status: Failed.")
		return fmt.Errorf("pipeline %s completed with status: %s", p.Name(), result.Status)
	}

	if result != nil {
		ctx.Logger.Info("Pipeline execution completed.", "status", result.Status, "duration", result.EndTime.Sub(result.StartTime).String())
	} else {
		// Should not happen if err is nil
		ctx.Logger.Warn("Pipeline execution returned nil result and nil error.")
	}
	return nil
}

// printResultAsText prints the GraphExecutionResult in a human-readable text format.
func printResultAsText(ctx *runtime.Context, result *plan.GraphExecutionResult) {
	if result == nil {
		if ctx != nil && ctx.Logger != nil {
			ctx.Logger.Warn("Graph execution result is nil, nothing to print.")
		} else {
			fmt.Println("Graph execution result is nil.")
		}
		return
	}

	fmt.Printf("\n--- Execution Summary ---\n")
	fmt.Printf("Graph Name: %s\n", result.GraphName)
	fmt.Printf("Overall Status: %s\n", result.Status)
	fmt.Printf("Start Time: %s\n", result.StartTime.Format(time.RFC3339))
	if !result.EndTime.IsZero() {
		fmt.Printf("End Time: %s\n", result.EndTime.Format(time.RFC3339))
		fmt.Printf("Duration: %s\n", result.EndTime.Sub(result.StartTime).String())
	}
	fmt.Println("-------------------------")

	// Iterate over NodeResults. Order might not be guaranteed for map iteration.
	// For consistent output, one might sort NodeIDs or collect results in a slice first.
	// For simplicity here, iterating map directly.
	if len(result.NodeResults) > 0 {
		fmt.Println("\nNode Results:")
		for nodeID, nodeResult := range result.NodeResults {
			fmt.Printf("\n  Node ID: %s\n", nodeID)
			fmt.Printf("    Name: %s (Step: %s)\n", nodeResult.NodeName, nodeResult.StepName)
			fmt.Printf("    Status: %s\n", nodeResult.Status)
			fmt.Printf("    Start: %s, End: %s\n", nodeResult.StartTime.Format(time.RFC1123Z), nodeResult.EndTime.Format(time.RFC1123Z))
			if nodeResult.Message != "" {
				fmt.Printf("    Message: %s\n", nodeResult.Message)
			}

			if len(nodeResult.HostResults) > 0 {
				fmt.Println("    Host Results:")
				for hostName, hostResult := range nodeResult.HostResults {
					fmt.Printf("      - %s: [%s]\n", hostName, hostResult.Status)
					if hostResult.Message != "" {
						fmt.Printf("        Message: %s\n", hostResult.Message)
					}
					if hostResult.Skipped {
						// Message for skipped by precheck is usually "Skipped: Precheck condition already met."
					} else if hostResult.Status == plan.StatusFailed {
						if hostResult.Stdout != "" {
							fmt.Printf("        Stdout: %s\n", strings.TrimSpace(hostResult.Stdout))
						}
						if hostResult.Stderr != "" {
							fmt.Printf("        Stderr: %s\n", strings.TrimSpace(hostResult.Stderr))
						}
					} else if hostResult.Status == plan.StatusSuccess {
						// Optionally print stdout/stderr for successful steps if verbose or needed
						if ctx.IsVerbose() && hostResult.Stdout != "" { // Example: only if verbose
							fmt.Printf("        Stdout: %s\n", strings.TrimSpace(hostResult.Stdout))
						}
					}
				}
			} else {
				fmt.Println("    No host-specific results for this node.")
			}
		}
	} else {
		fmt.Println("No node results available.")
	}
	fmt.Println("\n--- End of Execution Summary ---")
}

func main() {
	// Cobra's Execute performs argument parsing, flag handling, and runs the appropriate command.
	if err := rootCmd.Execute(); err != nil {
		// Error from Execute() usually means an issue with command parsing or flag handling.
		// Fatal errors from the RunE functions should be handled there.
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		os.Exit(1)
	}
}
