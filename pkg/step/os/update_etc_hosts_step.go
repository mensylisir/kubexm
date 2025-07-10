package os

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// UpdateEtcHostsStep adds or ensures entries in /etc/hosts.
type UpdateEtcHostsStep struct {
	meta    spec.StepMeta
	Entries map[string][]string // IP address -> list of hostnames
	Sudo    bool
	// TODO: Add a way to remove entries if needed for rollback or cleanup
}

// NewUpdateEtcHostsStep creates a new UpdateEtcHostsStep.
// entries: a map where key is IP and value is a slice of hostnames for that IP.
func NewUpdateEtcHostsStep(instanceName string, entries map[string][]string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "UpdateEtcHosts"
	}
	return &UpdateEtcHostsStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Updates /etc/hosts with %d entries.", len(entries)),
		},
		Entries: entries,
		Sudo:    sudo,
	}
}

func (s *UpdateEtcHostsStep) Meta() *spec.StepMeta {
	return &s.meta
}

// hasEntry checks if a specific IP and all its associated hostnames exist as a line in hostsContent.
func (s *UpdateEtcHostsStep) hasEntry(hostsContent, ip string, hostnames []string) bool {
	normalizedHostnames := make(map[string]bool)
	for _, hn := range hostnames {
		normalizedHostnames[strings.ToLower(hn)] = true
	}

	for _, line := range strings.Split(hostsContent, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "#") || trimmedLine == "" {
			continue
		}
		fields := strings.Fields(trimmedLine)
		if len(fields) < 2 {
			continue
		}
		lineIP := fields[0]
		if lineIP == ip {
			// IP matches, check if all hostnames are present in this line
			foundAllHostnamesInLine := true
			lineHostnames := make(map[string]bool)
			for _, hnField := range fields[1:] {
				if strings.HasPrefix(hnField, "#") { // Stop if comment starts
					break
				}
				lineHostnames[strings.ToLower(hnField)] = true
			}

			for hn := range normalizedHostnames {
				if !lineHostnames[hn] {
					foundAllHostnamesInLine = false
					break
				}
			}
			if foundAllHostnamesInLine && len(normalizedHostnames) == len(lineHostnames) { // Ensure exact match of hostnames for this IP
				return true
			}
		}
	}
	return false
}


func (s *UpdateEtcHostsStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	if len(s.Entries) == 0 {
		logger.Info("No /etc/hosts entries specified to update.")
		return true, nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, err
	}

	hostsFilePath := "/etc/hosts"
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, hostsFilePath)
	if err != nil {
		logger.Warn("Failed to check existence of /etc/hosts. Step will run.", "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("/etc/hosts file does not exist. Step will run to create/populate it.")
		return false, nil
	}

	contentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, hostsFilePath)
	if err != nil {
		logger.Warn("Failed to read /etc/hosts for precheck. Step will run.", "error", err)
		return false, nil
	}
	hostsContent := string(contentBytes)

	allEntriesExist := true
	for ip, hostnames := range s.Entries {
		if !s.hasEntry(hostsContent, ip, hostnames) {
			logger.Info("Missing /etc/hosts entry.", "ip", ip, "hostnames", hostnames)
			allEntriesExist = false
			break
		}
	}

	if allEntriesExist {
		logger.Info("All specified /etc/hosts entries already exist.")
		return true, nil
	}
	return false, nil
}

func (s *UpdateEtcHostsStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	if len(s.Entries) == 0 {
		logger.Info("No /etc/hosts entries to update.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return err
	}

	hostsFilePath := "/etc/hosts"

	// Read existing content
	var existingContent string
	if exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, hostsFilePath); exists {
		contentBytes, errRead := runnerSvc.ReadFile(ctx.GoContext(), conn, hostsFilePath)
		if errRead != nil {
			logger.Warn("Failed to read existing /etc/hosts, new entries will be appended to a new or potentially incomplete file.", "error", errRead)
		} else {
			existingContent = string(contentBytes)
		}
	}

	var linesToAdd []string
	for ip, hostnames := range s.Entries {
		if !s.hasEntry(existingContent, ip, hostnames) {
			entryLine := fmt.Sprintf("%s\t%s", ip, strings.Join(hostnames, "\t"))
			linesToAdd = append(linesToAdd, entryLine)
		}
	}

	if len(linesToAdd) == 0 {
		logger.Info("No new /etc/hosts entries needed, all already exist or match.")
		return nil
	}

	// Append new entries
	// Ensure there's a newline at the end of existing content if it's not empty
	appendContent := strings.Join(linesToAdd, "\n") + "\n"

	var newFileContent string
	if strings.TrimSpace(existingContent) == "" {
		newFileContent = appendContent
	} else if strings.HasSuffix(strings.TrimRight(existingContent, "\n"), "\n") { // Check if ends with newline, ignoring trailing blank lines
		newFileContent = existingContent + appendContent
	} else {
		newFileContent = existingContent + "\n" + appendContent
	}

	logger.Info("Updating /etc/hosts with new entries.")
	// Typically /etc/hosts is 0644
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(newFileContent), hostsFilePath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write to /etc/hosts: %w", err)
	}

	logger.Info("/etc/hosts updated successfully.")
	return nil
}

func (s *UpdateEtcHostsStep) Rollback(ctx step.StepContext, host connector.Host) error {
	// Rollback is complex: would need to remove only the lines added by this step.
	// This requires storing which lines were added or a more sophisticated diff/patch mechanism.
	// For now, a no-op or manual intervention is assumed.
	ctx.GetLogger().Warn("Rollback for UpdateEtcHostsStep is not implemented. Manual removal of entries may be needed.", "step", s.meta.Name)
	return nil
}

var _ step.Step = (*UpdateEtcHostsStep)(nil)
```
