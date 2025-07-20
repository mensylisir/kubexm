package common

import (
	"bufio"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	limitsTargetPath = common.SecuriryLimitsDefaultFileTarget
)

type LimitEntry struct {
	Domain string
	Type   string
	Item   string
	Value  string
}

type ManageLimitsStep struct {
	step.Base
	LimitsToEnsure []LimitEntry
	CustomLimits   []LimitEntry
}

type ManageLimitsStepBuilder struct {
	step.Builder[ManageLimitsStepBuilder, *ManageLimitsStep]
}

func NewManageLimitsStepBuilder(instanceName string) *ManageLimitsStepBuilder {
	defaultLimits := []LimitEntry{
		{Domain: "*", Type: "soft", Item: "nofile", Value: "1048576"},
		{Domain: "*", Type: "hard", Item: "nofile", Value: "1048576"},
		{Domain: "*", Type: "soft", Item: "nproc", Value: "65536"},
		{Domain: "*", Type: "hard", Item: "nproc", Value: "65536"},
		{Domain: "*", Type: "soft", Item: "memlock", Value: "unlimited"},
		{Domain: "*", Type: "hard", Item: "memlock", Value: "unlimited"},
	}
	cs := &ManageLimitsStep{LimitsToEnsure: defaultLimits}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Ensure system security limits are configured", instanceName)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 2 * time.Minute
	return new(ManageLimitsStepBuilder).Init(cs)
}

func (b *ManageLimitsStepBuilder) WithLimitsToEnsure(limits []LimitEntry) *ManageLimitsStepBuilder {
	b.Step.LimitsToEnsure = limits
	return b
}

func (b *ManageLimitsStepBuilder) WithCustomLimits(limits []LimitEntry) *ManageLimitsStepBuilder {
	b.Step.CustomLimits = limits
	return b
}

func (s *ManageLimitsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageLimitsStep) getFinalLimits() []LimitEntry {
	finalLimitsMap := make(map[string]LimitEntry)
	for _, limit := range s.LimitsToEnsure {
		key := fmt.Sprintf("%s-%s-%s", limit.Domain, limit.Type, limit.Item)
		finalLimitsMap[key] = limit
	}
	for _, limit := range s.CustomLimits {
		key := fmt.Sprintf("%s-%s-%s", limit.Domain, limit.Type, limit.Item)
		finalLimitsMap[key] = limit
	}

	finalLimits := make([]LimitEntry, 0, len(finalLimitsMap))
	for _, limit := range finalLimitsMap {
		finalLimits = append(finalLimits, limit)
	}

	return finalLimits
}

func (s *ManageLimitsStep) rebuildLimitsContent(originalContent string) ([]byte, error) {
	finalLimits := s.getFinalLimits()
	var newLines []string
	limitsFound := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(originalContent))
	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "#") || trimmedLine == "" {
			newLines = append(newLines, line)
			continue
		}

		parts := strings.Fields(trimmedLine)
		if len(parts) < 4 {
			newLines = append(newLines, line)
			continue
		}

		lineDomain, lineType, lineItem := parts[0], parts[1], parts[2]

		isManaged := false
		for _, limitToEnsure := range finalLimits {
			if lineDomain == limitToEnsure.Domain && lineType == limitToEnsure.Type && lineItem == limitToEnsure.Item {
				newLine := fmt.Sprintf("%-16s\t%s\t%s\t%s", limitToEnsure.Domain, limitToEnsure.Type, limitToEnsure.Item, limitToEnsure.Value)
				newLines = append(newLines, newLine)
				key := fmt.Sprintf("%s-%s-%s", lineDomain, lineType, lineItem)
				limitsFound[key] = true
				isManaged = true
				break
			}
		}
		if !isManaged {
			newLines = append(newLines, line)
		}
	}

	for _, limitToEnsure := range finalLimits {
		key := fmt.Sprintf("%s-%s-%s", limitToEnsure.Domain, limitToEnsure.Type, limitToEnsure.Item)
		if !limitsFound[key] {
			newLine := fmt.Sprintf("%-16s\t%s\t%s\t%s", limitToEnsure.Domain, limitToEnsure.Type, limitToEnsure.Item, limitToEnsure.Value)
			newLines = append(newLines, newLine)
		}
	}

	return []byte(strings.Join(newLines, "\n") + "\n"), nil
}

func (s *ManageLimitsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	currentContent, err := ReadRemoteFile(ctx, limitsTargetPath, s.Sudo)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	expectedContent, err := s.rebuildLimitsContent(currentContent)
	if err != nil {
		return false, err
	}

	if strings.TrimSpace(currentContent) == strings.TrimSpace(string(expectedContent)) {
		logger.Info("Security limits are already correctly configured. Step considered done.")
		return true, nil
	}

	logger.Info("Security limits configuration needs to be updated. Step needs to run.")
	return false, nil
}

func (s *ManageLimitsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	currentContent, err := ReadRemoteFile(ctx, limitsTargetPath, s.Sudo)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("run: failed to read %s: %w", limitsTargetPath, err)
	}

	newContent, err := s.rebuildLimitsContent(currentContent)
	if err != nil {
		return fmt.Errorf("failed to rebuild limits content: %w", err)
	}

	logger.Infof("Atomically writing limits configuration to %s", limitsTargetPath)
	return AtomicWriteRemoteFile(ctx, limitsTargetPath, newContent, s.Sudo)
}

func (s *ManageLimitsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for ManageLimitsStep is a no-op due to complexity.")
	return nil
}

var _ step.Step = (*ManageLimitsStep)(nil)

func ReadRemoteFile(ctx runtime.ExecutionContext, path string, sudo bool) (string, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return "", err
	}

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, "cat "+path, sudo)
	if err != nil {
		if strings.Contains(stderr, "No such file or directory") {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("failed to read remote file %s: %w, stderr: %s", path, err, stderr)
	}

	return stdout, nil
}
