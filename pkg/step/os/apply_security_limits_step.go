package os

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// SecurityLimitEntry defines a single entry for /etc/security/limits.conf.
type SecurityLimitEntry struct {
	Domain string // e.g., "*", "username", "@group"
	Type   string // "soft", "hard", "-" (for both)
	Item   string // e.g., "nofile", "nproc"
	Value  string // e.g., "65536", "unlimited"
}

// ApplySecurityLimitsStep manages entries in /etc/security/limits.conf.
type ApplySecurityLimitsStep struct {
	meta    spec.StepMeta
	Limits  []SecurityLimitEntry // List of limit entries to ensure/add
	Sudo    bool
	ConfFile string // Default: /etc/security/limits.conf
}

// NewApplySecurityLimitsStep creates a new ApplySecurityLimitsStep.
func NewApplySecurityLimitsStep(instanceName string, limits []SecurityLimitEntry, confFile string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "ApplySecurityLimits"
	}
	cf := confFile
	if cf == "" {
		cf = "/etc/security/limits.conf"
	}
	return &ApplySecurityLimitsStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Applies security limits to %s.", cf),
		},
		Limits:   limits,
		Sudo:     sudo,
		ConfFile: cf,
	}
}

func (s *ApplySecurityLimitsStep) Meta() *spec.StepMeta {
	return &s.meta
}

// entryToString converts a SecurityLimitEntry to its string representation for the file.
func (e *SecurityLimitEntry) entryToString() string {
	return fmt.Sprintf("%s\t%s\t%s\t%s", e.Domain, e.Type, e.Item, e.Value)
}

// lineMatchesEntry checks if a line from limits.conf matches a SecurityLimitEntry.
// This is a basic check; more robust parsing might be needed for complex lines.
func lineMatchesEntry(line string, entry SecurityLimitEntry) bool {
	trimmedLine := strings.TrimSpace(line)
	if strings.HasPrefix(trimmedLine, "#") || trimmedLine == "" {
		return false
	}
	fields := strings.Fields(trimmedLine)
	if len(fields) == 4 {
		return fields[0] == entry.Domain &&
			fields[1] == entry.Type &&
			fields[2] == entry.Item &&
			fields[3] == entry.Value
	}
	return false
}


func (s *ApplySecurityLimitsStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	if len(s.Limits) == 0 {
		logger.Info("No security limits specified to apply.")
		return true, nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, err
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.ConfFile)
	if err != nil {
		logger.Warn("Failed to check existence of security limits file. Step will run.", "file", s.ConfFile, "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("Security limits file does not exist. Step will run to create it.", "file", s.ConfFile)
		return false, nil
	}

	contentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ConfFile)
	if err != nil {
		logger.Warn("Failed to read security limits file for precheck. Step will run.", "file", s.ConfFile, "error", err)
		return false, nil
	}
	currentContent := string(contentBytes)
	lines := strings.Split(currentContent, "\n")

	allLimitsExist := true
	for _, limit := range s.Limits {
		found := false
		for _, line := range lines {
			if lineMatchesEntry(line, limit) {
				found = true
				break
			}
		}
		if !found {
			logger.Info("Security limit entry not found or does not match.", "limit", limit.entryToString())
			allLimitsExist = false
			break
		}
	}

	if allLimitsExist {
		logger.Info("All specified security limits already correctly configured.")
		return true, nil
	}
	return false, nil
}

func (s *ApplySecurityLimitsStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	if len(s.Limits) == 0 {
		logger.Info("No security limits to apply.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return err
	}

	// Read existing content to append new limits if they don't exist.
	var existingContent string
	if exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, s.ConfFile); exists {
		contentBytes, errRead := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ConfFile)
		if errRead != nil {
			logger.Warn("Failed to read existing security limits file. Will attempt to write a new file or append.", "file", s.ConfFile, "error", errRead)
		} else {
			existingContent = string(contentBytes)
		}
	}

	var linesToAdd []string
	currentLines := strings.Split(existingContent, "\n")

	for _, limit := range s.Limits {
		found := false
		for _, line := range currentLines {
			if lineMatchesEntry(line, limit) {
				found = true
				break
			}
		}
		if !found {
			linesToAdd = append(linesToAdd, limit.entryToString())
		}
	}

	if len(linesToAdd) == 0 {
		logger.Info("All security limits already present and correctly configured.")
		return nil
	}

	var finalContentBuilder strings.Builder
	finalContentBuilder.WriteString(existingContent)
	// Ensure a newline before appending if existing content is not empty and doesn't end with one
	if strings.TrimSpace(existingContent) != "" && !strings.HasSuffix(existingContent, "\n") {
		finalContentBuilder.WriteString("\n")
	}
	for _, line := range linesToAdd {
		finalContentBuilder.WriteString(line)
		finalContentBuilder.WriteString("\n")
	}

	logger.Info("Applying new security limits.", "file", s.ConfFile, "entries_to_add", len(linesToAdd))
	// /etc/security/limits.conf is typically 0644
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(finalContentBuilder.String()), s.ConfFile, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write security limits to %s: %w", s.ConfFile, err)
	}

	logger.Info("Security limits applied successfully. A logout/login or reboot may be required for changes to take effect for user sessions.")
	return nil
}

func (s *ApplySecurityLimitsStep) Rollback(ctx step.StepContext, host connector.Host) error {
	// Rollback is complex: requires removing only the exact lines added by this step.
	// This would need storing the original state or a more sophisticated diff/patch.
	ctx.GetLogger().Warn("Rollback for ApplySecurityLimitsStep is not implemented. Manual removal of entries may be needed.", "step", s.meta.Name)
	return nil
}

var _ step.Step = (*ApplySecurityLimitsStep)(nil)
```
