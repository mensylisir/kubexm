package common

import (
	"fmt"
	"os"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/olekukonko/tablewriter"
)

type ReportTableStep struct {
	step.Base
	Headers []string
	Rows    [][]string
}

type ReportTableStepBuilder struct {
	step.Builder[ReportTableStepBuilder, *ReportTableStep]
}

func NewReportTableStepBuilder(instanceName string) *ReportTableStepBuilder {
	cs := &ReportTableStep{}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Displaying summary report table...", instanceName)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = true
	cs.Base.Timeout = 1 * time.Minute
	return new(ReportTableStepBuilder).Init(cs)
}

func (b *ReportTableStepBuilder) WithHeaders(headers []string) *ReportTableStepBuilder {
	b.Step.Headers = headers
	return b
}

func (b *ReportTableStepBuilder) WithRows(rows [][]string) *ReportTableStepBuilder {
	b.Step.Rows = rows
	return b
}

func (s *ReportTableStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ReportTableStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if len(s.Rows) == 0 {
		logger.Info("No rows provided for table report, skipping step.")
		return true, nil
	}
	return false, nil
}

func (s *ReportTableStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	if len(s.Rows) == 0 {
		logger.Info("No data to display in table report.")
		return nil
	}
	logger.Info("Printing report table to console...")
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(s.Headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)
	table.AppendBulk(s.Rows)

	fmt.Println()
	table.Render()
	fmt.Println()

	logger.Info("Table report displayed successfully.")
	return nil
}

func (s *ReportTableStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for ReportTableStep is a no-op.")
	return nil
}

var _ step.Step = (*ReportTableStep)(nil)
