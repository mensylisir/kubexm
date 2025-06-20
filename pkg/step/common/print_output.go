package common

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector" // Host parameter is not used by logic but part of interface
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// PrintOutputStepSpec defines parameters for printing content to logs/console.
type PrintOutputStepSpec struct {
	spec.StepMeta `json:",inline"`

	Content string `json:"content,omitempty"`
	FormatAs string `json:"formatAs,omitempty"` // "raw", "info", "warn", "error", "success", "debug"
	LogOnly  bool   `json:"logOnly,omitempty"`  // If true, only log, don't attempt direct stdout if logger handles it.
}

// NewPrintOutputStepSpec creates a new PrintOutputStepSpec.
func NewPrintOutputStepSpec(name, description, content string) *PrintOutputStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Print Output"
	}
	finalDescription := description
	// Description refined in populateDefaults

	return &PrintOutputStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Content: content,
		// FormatAs and LogOnly defaulted in populateDefaults
	}
}

// Name returns the step's name.
func (s *PrintOutputStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *PrintOutputStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *PrintOutputStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *PrintOutputStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *PrintOutputStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *PrintOutputStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *PrintOutputStepSpec) populateDefaults(logger runtime.Logger) {
	if s.FormatAs == "" {
		s.FormatAs = "info"
		// No need to log this default unless verbose debugging is on for the framework itself
	}
	s.FormatAs = strings.ToLower(s.FormatAs)

	// LogOnly defaults to false (zero value)

	if s.StepMeta.Description == "" {
		maxLength := 30
		if len(s.Content) < maxLength {
			maxLength = len(s.Content)
		}
		ellipsis := ""
		if len(s.Content) > maxLength {
			ellipsis = "..."
		}
		s.StepMeta.Description = fmt.Sprintf("Prints content (format: %s): \"%.*s%s\"",
			s.FormatAs, maxLength, s.Content, ellipsis)
	}
}

// Precheck for PrintOutputStep always returns false, as printing is an action to be performed.
func (s *PrintOutputStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	s.populateDefaults(ctx.GetLogger()) // Logger passed for consistency, though not used by this populateDefaults
	if s.Content == "" {
		ctx.GetLogger().Debug("No content to print, skipping step.", "step", s.GetName())
		return true, nil // If no content, consider it "done" or skippable
	}
	return false, nil // Always run if there's content
}

// Run executes the printing/logging.
func (s *PrintOutputStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	// Host parameter is ignored as this step typically prints general output, not host-specific.
	logger := ctx.GetLogger().With("step", s.GetName())
	s.populateDefaults(logger) // Ensures FormatAs is set

	if s.Content == "" {
		logger.Debug("No content to print.")
		return nil
	}

	// If LogOnly is true, all outputs go through the logger.
	// If LogOnly is false, "raw" might print to fmt.Println if logger adds too much decoration.
	// However, most modern loggers used in CLIs are configured to output to console for info/warn/error levels.
	// So, LogOnly=false might be redundant if logger is well-configured.
	// For simplicity, assume logger handles console output for standard levels.
	// "raw" will be treated as special if LogOnly is false.

	switch s.FormatAs {
	case "raw":
		if !s.LogOnly {
			// This provides the most direct, unadorned output to stdout.
			// If the primary logger also writes to stdout, this might lead to double printing
			// or slight formatting differences. This is for cases where truly raw output is needed.
			fmt.Println(s.Content)
			// If we want to ensure it's also in logs, even if raw to console:
			// logger.Info("[RAW] " + s.Content) // Or a specific raw log method if available
		} else {
			logger.Info(s.Content) // If LogOnly, even "raw" goes through logger.Info
		}
	case "warn":
		logger.Warn(s.Content)
	case "error":
		logger.Error(s.Content) // Note: This does not make the step fail. It just logs as error level.
	case "success":
		// Assuming logger might have a Success method or equivalent styling for Info.
		// If not, it will fall back to Info or be handled by logger's implementation.
		if sl, ok := logger.(interface{ Success(string) }); ok {
			sl.Success(s.Content)
		} else {
			logger.Info("[SUCCESS] " + s.Content) // Fallback for success
		}
	case "debug":
		logger.Debug(s.Content)
	case "info":
		fallthrough // Fallthrough for info and any other unspecified format
	default:
		logger.Info(s.Content)
	}
	return nil
}

// Rollback for PrintOutputStep is a no-op.
func (s *PrintOutputStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Debug("PrintOutputStep has no rollback action.")
	return nil
}

var _ step.Step = (*PrintOutputStepSpec)(nil)
