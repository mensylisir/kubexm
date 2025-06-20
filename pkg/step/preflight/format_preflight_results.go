package preflight

import (
	"encoding/json"
	"fmt"
	"os" // For tabwriter output to os.Stdout
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/Masterminds/semver/v3" // For version comparison
	"github.com/mensylisir/kubexm/pkg/connector" // Host parameter is not used by logic but part of interface
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"gopkg.in/yaml.v2" // For YAML output
)

// CheckResultToReport defines how to fetch, interpret, and display a single check result.
type CheckResultToReport struct {
	Name           string `json:"name,omitempty"`           // Human-readable name of the check (e.g., "Docker Installed")
	CacheKey       string `json:"cacheKey,omitempty"`       // StepCache key where the result (e.g., bool, string) is stored
	ExpectedValue  string `json:"expectedValue,omitempty"`  // Optional: String representation of the expected value for a "Pass".
	Comparison     string `json:"comparison,omitempty"`     // Optional: "exists", "equals", "not-equals", "contains", "not-contains", "semver-gte", "semver-lt", "is-true", "is-false". Default smart based on ExpectedValue.
	SuccessOnNoKey bool   `json:"successOnNoKey,omitempty"` // If true, and CacheKey not found, consider it a pass.
	ValueFormatter string `json:"valueFormatter,omitempty"` // Optional: "bool-yes-no", "bool-pass-fail".
	Mandatory      bool   `json:"mandatory,omitempty"`      // If true, a FAIL status for this check will be more prominent.
}

// FormatPreflightResultsStepSpec defines parameters for formatting and printing preflight check results.
type FormatPreflightResultsStepSpec struct {
	spec.StepMeta `json:",inline"`

	Title        string                `json:"title,omitempty"`
	Checks       []CheckResultToReport `json:"checks,omitempty"` // Required
	OutputFormat string                `json:"outputFormat,omitempty"` // "table", "json", "yaml"
	PrintToLog   bool                  `json:"printToLog,omitempty"`
}

// NewFormatPreflightResultsStepSpec creates a new FormatPreflightResultsStepSpec.
func NewFormatPreflightResultsStepSpec(name, description string, checks []CheckResultToReport) *FormatPreflightResultsStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Format Preflight Check Results"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if len(checks) == 0 {
		// This is a required field for the step to be meaningful.
	}

	return &FormatPreflightResultsStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Checks: checks,
	}
}

// Name returns the step's name.
func (s *FormatPreflightResultsStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *FormatPreflightResultsStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *FormatPreflightResultsStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *FormatPreflightResultsStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *FormatPreflightResultsStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *FormatPreflightResultsStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *FormatPreflightResultsStepSpec) populateDefaults(logger runtime.Logger) {
	if s.Title == "" {
		s.Title = "Preflight Check Results"
		logger.Debug("Title defaulted.", "title", s.Title)
	}
	if s.OutputFormat == "" {
		s.OutputFormat = "table"
		logger.Debug("OutputFormat defaulted to 'table'.")
	}
	s.OutputFormat = strings.ToLower(s.OutputFormat)

	// PrintToLog defaults to true. If zero value (false), set to true.
	// This makes it default true unless explicitly set false.
	// A common pattern for bools that should default true.
	// If the factory set this to true, this logic isn't strictly needed here.
	// Assuming factory doesn't set it, so we ensure it's true by default here.
	if !s.PrintToLog && !utils.IsFieldExplicitlySet(s, "PrintToLog") { // Hypothetical IsFieldExplicitlySet
		s.PrintToLog = true
		logger.Debug("PrintToLog defaulted to true.")
	}


	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Formats and prints %d preflight check results as '%s'.", len(s.Checks), s.OutputFormat)
	}
}

// Precheck validates inputs.
func (s *FormatPreflightResultsStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger) // Host not used by this step's populateDefaults

	if len(s.Checks) == 0 {
		logger.Info("No checks defined to format. Step will do nothing.")
		return true, nil // Consider it done if there's nothing to format.
	}
	return false, nil // Always run to format and print if there are checks.
}

// Run formats and prints the preflight check results.
func (s *FormatPreflightResultsStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger) // Host not used

	if len(s.Checks) == 0 {
		logger.Info("No checks to format.")
		return nil
	}

	resultsData := []map[string]string{}
	overallStatusIsFail := false

	for _, checkItem := range s.Checks {
		cachedVal, found := ctx.StepCache().Get(checkItem.CacheKey)

		statusString := "INFO" // Default status
		displayValue := "N/A"
		valueIsSet := found

		if found {
			// Format value
			switch checkItem.ValueFormatter {
			case "bool-yes-no":
				if bVal, ok := cachedVal.(bool); ok {
					if bVal { displayValue = "Yes" } else { displayValue = "No" }
				} else { displayValue = fmt.Sprintf("%v (expected bool)", cachedVal) }
			case "bool-pass-fail":
				if bVal, ok := cachedVal.(bool); ok {
					if bVal { displayValue = "Pass" } else { displayValue = "Fail" }
				} else { displayValue = fmt.Sprintf("%v (expected bool)", cachedVal) }
			default:
				displayValue = fmt.Sprintf("%v", cachedVal)
			}
		} else { // Key not found
		    displayValue = "Not Found/Not Run"
		}

		// Determine comparison logic and status
		comparisonType := strings.ToLower(checkItem.Comparison)
		if comparisonType == "" { // Smart default comparison
			if checkItem.ExpectedValue != "" {
				comparisonType = "equals"
			} else if _, isBool := cachedVal.(bool); isBool {
				comparisonType = "is-true" // Default for bools if no expected value is "Pass if true"
			} else {
				comparisonType = "exists" // Default for non-bools if no expected value
			}
		}

		pass := false
		if !valueIsSet {
			if checkItem.SuccessOnNoKey {
				pass = true
				statusString = "PASS (SKIPPED)"
				displayValue = "N/A (Optional)"
			} else {
				pass = false // Key not found for a non-optional check is a fail or info
				statusString = "WARN" // Or FAIL if mandatory
				if checkItem.Mandatory { statusString = "FAIL" }
			}
		} else { // Value is set, perform comparison
			switch comparisonType {
			case "exists":
				pass = true // Already know valueIsSet is true
			case "equals":
				pass = displayValue == checkItem.ExpectedValue || fmt.Sprintf("%v", cachedVal) == checkItem.ExpectedValue
			case "not-equals":
				pass = displayValue != checkItem.ExpectedValue && fmt.Sprintf("%v", cachedVal) != checkItem.ExpectedValue
			case "contains":
				pass = strings.Contains(displayValue, checkItem.ExpectedValue)
			case "not-contains":
				pass = !strings.Contains(displayValue, checkItem.ExpectedValue)
			case "is-true":
				if bVal, ok := cachedVal.(bool); ok { pass = bVal } else { pass = false; statusString = "ERROR (Type)"}
			case "is-false":
				if bVal, ok := cachedVal.(bool); ok { pass = !bVal } else { pass = false; statusString = "ERROR (Type)"}
			case "semver-gte", "semver-lt": // Simplified: use Masterminds/semver
				actualV, errActual := semver.NewVersion(fmt.Sprintf("%v", cachedVal))
				expectedV, errExpected := semver.NewVersion(checkItem.ExpectedValue)
				if errActual != nil || errExpected != nil {
					logger.Warn("Failed to parse versions for semver comparison.", "actual", cachedVal, "expected", checkItem.ExpectedValue, "errorActual", errActual, "errorExpected", errExpected)
					pass = false
					statusString = "ERROR (Ver)"
				} else {
					if comparisonType == "semver-gte" { pass = !actualV.LessThan(expectedV) }
					if comparisonType == "semver-lt" { pass = actualV.LessThan(expectedV) }
				}
			default:
				logger.Warn("Unknown comparison type, defaulting to info.", "type", comparisonType)
				statusString = "INFO" // Cannot determine pass/fail
				pass = false // Or true, depending on desired behavior for unknown comparison
			}

			if pass {
				statusString = "PASS"
			} else {
				if statusString != "ERROR (Type)" && statusString != "ERROR (Ver)" { // Don't override specific errors
					statusString = "FAIL"
				}
			}
		}

		if statusString == "FAIL" && checkItem.Mandatory {
			overallStatusIsFail = true
		}


		resultsData = append(resultsData, map[string]string{
			"Check":  checkItem.Name,
			"Result": displayValue,
			"Status": statusString,
		})
	}

	// Format and print output
	var outputBuffer bytes.Buffer
	switch s.OutputFormat {
	case "table":
		// Sort resultsData by Check name for consistent table output
		sort.Slice(resultsData, func(i, j int) bool {
			return resultsData[i]["Check"] < resultsData[j]["Check"]
		})

		outputBuffer.WriteString(fmt.Sprintf("\n%s\n", s.Title))
		tw := tabwriter.NewWriter(&outputBuffer, 0, 0, 3, ' ', tabwriter.Debug) // Use tabwriter.Debug for borders if desired
		fmt.Fprintln(tw, "CHECK\tRESULT\tSTATUS")
		fmt.Fprintln(tw, "-----\t------\t------")
		for _, res := range resultsData {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", res["Check"], res["Result"], res["Status"])
		}
		tw.Flush()
	case "json":
		jsonData, err := json.MarshalIndent(resultsData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal results to JSON: %w", err)
		}
		outputBuffer.Write(jsonData)
	case "yaml":
		yamlData, err := yaml.Marshal(resultsData)
		if err != nil {
			return fmt.Errorf("failed to marshal results to YAML: %w", err)
		}
		outputBuffer.Write(yamlData)
	default:
		return fmt.Errorf("unsupported OutputFormat: %s", s.OutputFormat)
	}

	// Print to logger and/or console
	if s.PrintToLog {
		// Log line by line for table format to maintain some structure in logs
		if s.OutputFormat == "table" {
			for _, line := range strings.Split(strings.TrimRight(outputBuffer.String(), "\n"), "\n") {
				logger.Info(line)
			}
		} else {
			logger.Info(outputBuffer.String())
		}
	}

	// If not LogOnly, and format is table, also print to console directly for better formatting control.
	// This assumes logger might not be stdout or might add prefixes.
	if !s.LogOnly && s.OutputFormat == "table" {
	     // This might double-print if logger also goes to console.
	     // A common pattern is for CLI tools to have a separate "Print" function for tables.
	     // For now, let's assume if PrintToLog is true, that's enough for console.
	     // If specific direct stdout is needed, this would be:
	     // fmt.Println(outputBuffer.String())
	}


	if overallStatusIsFail {
	    // The step itself doesn't fail, but it can log a summary error if any mandatory check failed.
	    logger.Error("One or more mandatory preflight checks failed. Please review results.")
	}

	return nil
}

// Rollback for FormatPreflightResultsStep is a no-op.
func (s *FormatPreflightResultsStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Debug("FormatPreflightResultsStep has no rollback action.")
	return nil
}

var _ step.Step = (*FormatPreflightResultsStepSpec)(nil)
