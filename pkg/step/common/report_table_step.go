package common

import (
	"fmt"
	"github.com/olekukonko/tablewriter" // External library for table formatting
	"os"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ReportTableStep prints data in a table format to the console.
type ReportTableStep struct {
	meta    spec.StepMeta
	Headers []string
	Rows    [][]string // Data rows for the table
	// Alternatively, Rows could be a key to fetch data from TaskCache/ModuleCache
	// DataCacheKey string
}

// NewReportTableStep creates a new ReportTableStep.
func NewReportTableStep(instanceName string, headers []string, rows [][]string) step.Step {
	name := instanceName
	if name == "" {
		name = "DisplayTableReport"
	}
	return &ReportTableStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Displays a report in a table format.",
		},
		Headers: headers,
		Rows:    rows,
	}
}

func (s *ReportTableStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Precheck for ReportTableStep always returns false, as it's meant to always execute to show current data.
func (s *ReportTableStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	if len(s.Rows) == 0 {
		// If there are no rows to display, consider the step "done" or skippable.
		ctx.GetLogger().Info("No rows provided for table report, skipping.", "step", s.meta.Name)
		return true, nil
	}
	return false, nil
}

// Run prints the table.
func (s *ReportTableStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name)

	if len(s.Rows) == 0 {
		logger.Info("No data to display in table report.")
		// This case should ideally be caught by Precheck.
		return nil
	}

	// Using tablewriter library for nice console tables.
	// This library needs to be added to go.mod: go get github.com/olekukonko/tablewriter
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(s.Headers)
	table.SetBorder(true)    // Set Border to true
	table.SetRowLine(true)   // Enable row line
	table.AppendBulk(s.Rows) // Add Bulk Data

	// For more complex alignment or per-column settings, tablewriter offers more options.
	// Example: table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT})

	fmt.Println() // Add a newline before the table for better spacing
	table.Render()
	fmt.Println() // Add a newline after the table

	logger.Info("Table report displayed.")
	return nil
}

// Rollback for ReportTableStep is a no-op as it only prints information.
func (s *ReportTableStep) Rollback(ctx step.StepContext, host connector.Host) error {
	return nil
}

var _ step.Step = (*ReportTableStep)(nil)
