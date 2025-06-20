package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log" // Standard log, consider replacing with custom logger if used elsewhere in main
	"os"
	"time" // For result.EndTime.Sub(result.StartTime)

	"github.com/spf13/cobra"
	// Corrected and expected import paths:
	"github.com/mensylisir/kubexm/pkg/pipeline"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	// Assuming your logger package is under pkg/logger
	// "github.com/mensylisir/kubexm/pkg/logger" // This is for the app's own logger, distinct from runtime ctx logger if needed
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
	// Using context.Background() for the top-level Go context for the build process
	// The logger within this ctx is from the runtime package
	ctx, cleanup, err := builder.Build(context.Background())
	if err != nil {
		// No ctx.Logger available yet if Build failed early. Use standard log or a pre-build app logger.
		log.Printf("Error: Failed to build runtime environment: %v\n", err) // Print to stdout/stderr
		return fmt.Errorf("failed to build runtime environment: %w", err)
	}
	defer cleanup() // Ensure cleanup function (e.g., closing connection pool) is called

	// Instantiate the desired pipeline
	// Using NewDeployAppPipeline as specified in the issue
	p := pipeline.NewDeployAppPipeline()

	ctx.Logger.Info("Starting pipeline execution...", "pipeline", p.Name(), "dryRun", dryRun, "configFile", configFile)

	// Run the pipeline
	result, err := p.Run(ctx, dryRun) // Pass the main runtime context

	// Handle results and output formatting
	if outputFormat == "json" {
		jsonResult, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			ctx.Logger.Error(marshalErr, "Failed to marshal execution result to JSON")
			// Still print text result if JSON marshalling fails
			printResultAsText(ctx, result)
			return fmt.Errorf("failed to marshal result to JSON: %w", marshalErr)
		}
		fmt.Println(string(jsonResult))
	} else {
		printResultAsText(ctx, result)
	}

	if err != nil {
		ctx.Logger.Error(err, "Pipeline execution finished with errors")
		return err // Return the error from pipeline.Run
	}

	if result != nil && result.Status == plan.StatusFailed {
		ctx.Logger.Info("Pipeline completed with failed status.")
		return fmt.Errorf("pipeline %s completed with status: %s", p.Name(), result.Status)
	}

	ctx.Logger.Info("Pipeline execution completed successfully.", "status", result.Status, "duration", result.EndTime.Sub(result.StartTime).String())
	return nil
}

// printResultAsText prints the execution result in a human-readable text format.
func printResultAsText(ctx *runtime.Context, result *plan.ExecutionResult) {
	if result == nil {
		if ctx != nil && ctx.Logger != nil {
			ctx.Logger.Warn("Execution result is nil, nothing to print.")
		} else {
			fmt.Println("Execution result is nil.")
		}
		return
	}

	fmt.Printf("\n--- Execution Summary ---\n")
	fmt.Printf("Pipeline Status: %s\n", result.Status)
	fmt.Printf("Start Time: %s\n", result.StartTime.Format(time.RFC3339))
	if !result.EndTime.IsZero() {
		fmt.Printf("End Time: %s\n", result.EndTime.Format(time.RFC3339))
		fmt.Printf("Duration: %s\n", result.EndTime.Sub(result.StartTime).String())
	}
	fmt.Println("-------------------------")

	for i, phaseResult := range result.PhaseResults {
		fmt.Printf("\nPhase %d: %s [%s]\n", i+1, phaseResult.PhaseName, phaseResult.Status)
		fmt.Println("-------------------------")
		for j, actionResult := range phaseResult.ActionResults {
			fmt.Printf("  Action %d.%d: %s [%s]\n", i+1, j+1, actionResult.ActionName, actionResult.Status)
			if len(actionResult.HostResults) > 0 {
				fmt.Println("    Hosts:")
				for hostName, hostResult := range actionResult.HostResults {
					fmt.Printf("      - %s: [%s]\n", hostName, hostResult.Status)
					if hostResult.Skipped {
						fmt.Printf("        Message: %s\n", hostResult.Message)
					} else if hostResult.Status == plan.StatusFailed {
						fmt.Printf("        Error: %s\n", hostResult.Message)
						if hostResult.Stdout != "" {
							fmt.Printf("        Stdout: %s\n", hostResult.Stdout)
						}
						if hostResult.Stderr != "" {
							fmt.Printf("        Stderr: %s\n", hostResult.Stderr)
						}
					} else { // Success
						fmt.Printf("        Message: %s\n", hostResult.Message)
						// Optionally print stdout/stderr for successful steps if verbose
					}
				}
			} else {
				fmt.Println("    No hosts targeted for this action or no results.")
			}
		}
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
