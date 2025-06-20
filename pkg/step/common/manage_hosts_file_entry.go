package common

import (
	"bufio"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

// ManageHostsFileEntryStepSpec defines parameters for managing an entry in /etc/hosts.
type ManageHostsFileEntryStepSpec struct {
	spec.StepMeta `json:",inline"`

	IPAddress     string   `json:"ipAddress,omitempty"`
	Hostnames     []string `json:"hostnames,omitempty"`
	State         string   `json:"state,omitempty"` // "present" or "absent"
	HostsFilePath string   `json:"hostsFilePath,omitempty"`
	Sudo          bool     `json:"sudo,omitempty"`
}

// NewManageHostsFileEntryStepSpec creates a new ManageHostsFileEntryStepSpec.
func NewManageHostsFileEntryStepSpec(name, description, ipAddress string, hostnames []string) *ManageHostsFileEntryStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Manage hosts file entry for %s", ipAddress)
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &ManageHostsFileEntryStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		IPAddress: ipAddress,
		Hostnames: hostnames,
	}
}

// Name returns the step's name.
func (s *ManageHostsFileEntryStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ManageHostsFileEntryStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ManageHostsFileEntryStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ManageHostsFileEntryStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ManageHostsFileEntryStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ManageHostsFileEntryStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ManageHostsFileEntryStepSpec) populateDefaults(logger runtime.Logger) {
	if s.State == "" {
		s.State = "present"
		logger.Debug("State defaulted to 'present'.")
	}
	s.State = strings.ToLower(s.State)

	if s.HostsFilePath == "" {
		s.HostsFilePath = "/etc/hosts"
		logger.Debug("HostsFilePath defaulted.", "path", s.HostsFilePath)
	}
	if !s.Sudo && utils.PathRequiresSudo(s.HostsFilePath) {
		s.Sudo = true // Default to sudo if path seems to require it and not explicitly false
		logger.Debug("Sudo defaulted to true due to privileged HostsFilePath.")
	}


	if s.StepMeta.Description == "" {
		verb := "Ensures"
		if s.State == "absent" {
			verb = "Removes"
		}
		s.StepMeta.Description = fmt.Sprintf("%s entry for IP %s with hostnames [%s] in %s.",
			verb, s.IPAddress, strings.Join(s.Hostnames, ", "), s.HostsFilePath)
	}
}

// parseHostsLine splits a hosts file line into IP and a sorted list of hostnames.
// Comments are stripped. Returns IP, sorted hostnames, and original line type (comment, empty, entry).
func parseHostsLine(line string) (ip string, names []string, linetype string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", nil, "empty"
	}
	if strings.HasPrefix(trimmed, "#") {
		return "", nil, "comment"
	}

	parts := strings.Fields(trimmed)
	if len(parts) < 2 {
		return "", nil, "malformed" // Not enough parts for IP + hostname
	}

	ip = parts[0]
	// Validate if parts[0] is a valid IP? For now, assume it is if not a comment.
	names = parts[1:]
	sort.Strings(names) // Sort for consistent comparison
	return ip, names, "entry"
}


// Precheck determines if the /etc/hosts entry is already in the desired state.
func (s *ManageHostsFileEntryStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.IPAddress == "" || len(s.Hostnames) == 0 {
		return false, fmt.Errorf("IPAddress and at least one Hostname must be specified for %s", s.GetName())
	}
	// Sort desired hostnames for consistent comparison
	desiredHostnames := make([]string, len(s.Hostnames))
	copy(desiredHostnames, s.Hostnames)
	sort.Strings(desiredHostnames)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.HostsFilePath)
	if err != nil {
		logger.Warn("Failed to check hosts file existence. Assuming changes are needed.", "path", s.HostsFilePath, "error", err)
		return false, nil
	}
	if !exists && s.State == "present" {
		logger.Info("Hosts file does not exist. Step will create it if possible or fail.", "path", s.HostsFilePath)
		return false, nil
	}
	if !exists && s.State == "absent" {
	    logger.Info("Hosts file does not exist. Desired state 'absent' is met.", "path", s.HostsFilePath)
	    return true, nil
	}


	contentBytes, err := conn.ReadFile(ctx.GoContext(), s.HostsFilePath)
	if err != nil {
		logger.Warn("Failed to read hosts file. Assuming changes are needed.", "path", s.HostsFilePath, "error", err)
		return false, nil // Let Run attempt and handle error
	}
	content := string(contentBytes)
	scanner := bufio.NewScanner(strings.NewReader(content))

	foundExactMatch := false
	var conflictingLines []string

	for scanner.Scan() {
		line := scanner.Text()
		lineIP, lineNames, lineType := parseHostsLine(line)

		if lineType != "entry" {
			continue
		}

		// Check for exact match (IP and all hostnames)
		if lineIP == s.IPAddress {
			if utils.StringSlicesEqual(lineNames, desiredHostnames) {
				foundExactMatch = true
			} else {
				// IP matches, but hostnames differ. This is a conflict if we want to enforce exact hostnames.
				conflictingLines = append(conflictingLines, fmt.Sprintf("IP %s currently maps to [%s], expected [%s]", s.IPAddress, strings.Join(lineNames, " "), strings.Join(desiredHostnames, " ")))
			}
		} else { // Different IP, check if any of our desired hostnames are on this line
			for _, desiredName := range desiredHostnames {
				for _, existingName := range lineNames {
					if desiredName == existingName {
						conflictingLines = append(conflictingLines, fmt.Sprintf("Hostname %s on IP %s conflicts with desired IP %s", desiredName, lineIP, s.IPAddress))
						break
					}
				}
			}
		}
	}

	if s.State == "present" {
		if foundExactMatch && len(conflictingLines) == 0 {
			logger.Info("Hosts file entry already present and correct.", "ip", s.IPAddress, "hostnames", desiredHostnames)
			return true, nil
		}
		if len(conflictingLines) > 0 {
			logger.Info("Conflicting entries found or IP maps to different hostnames.", "conflicts", conflictingLines)
		}
		logger.Info("Hosts file entry not in desired 'present' state or has conflicts.")
		return false, nil
	}

	// s.State == "absent"
	if !foundExactMatch && len(conflictingLines) == 0 { // No exact match for our IP, and our hostnames are not tied to other IPs.
		// This logic for absent is a bit simplified. We are checking if our exact IP->hostnames line is absent,
		// AND that none of our hostnames are present with OTHER IPs.
		// A stricter "absent" might just check if *any* line contains s.IPAddress or any of s.Hostnames.
		// The current Run logic removes lines with s.IPAddress OR any of s.Hostnames.
		// So, for precheck to match Run's "absent", we need to see if any line would be touched by Run's filtering.

		// Re-scan for any line that Run would remove
		scanner = bufio.NewScanner(strings.NewReader(content)) // Re-initialize scanner
		wouldBeRemoved := false
		for scanner.Scan() {
			line := scanner.Text()
			lineIP, lineNames, lineType := parseHostsLine(line)
			if lineType != "entry" { continue }

			if lineIP == s.IPAddress { wouldBeRemoved = true; break }
			for _, hn := range s.Hostnames {
				for _, existingName := range lineNames {
					if hn == existingName { wouldBeRemoved = true; break }
				}
				if wouldBeRemoved { break }
			}
			if wouldBeRemoved { break }
		}
		if !wouldBeRemoved {
			logger.Info("Hosts file entry effectively absent as per Run logic.", "ip", s.IPAddress, "hostnames", s.Hostnames)
			return true, nil
		}
		logger.Info("Hosts file entry needs to be made absent as per Run logic.")
		return false, nil

	}
	logger.Info("Hosts file entry not in desired 'absent' state.")
	return false, nil
}


// Run manages the /etc/hosts file entry.
func (s *ManageHostsFileEntryStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.IPAddress == "" || len(s.Hostnames) == 0 {
		return fmt.Errorf("IPAddress and at least one Hostname must be specified for %s", s.GetName())
	}
	sort.Strings(s.Hostnames) // Ensure consistent order for the new line

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var originalContent string
	fileExists, _ := conn.Exists(ctx.GoContext(), s.HostsFilePath)
	if fileExists {
		contentBytes, errRead := conn.ReadFile(ctx.GoContext(), s.HostsFilePath)
		if errRead != nil {
			// If we can't read it, we can't safely modify it.
			return fmt.Errorf("failed to read hosts file %s on host %s: %w", s.HostsFilePath, host.GetName(), errRead)
		}
		originalContent = string(contentBytes)
	} else if s.State == "absent" {
		logger.Info("Hosts file does not exist, desired state 'absent' is met by default.")
		return nil // Nothing to do
	}


	var newLines []string
	scanner := bufio.NewScanner(strings.NewReader(originalContent))
	modified := false

	for scanner.Scan() {
		line := scanner.Text()
		lineIP, lineNames, lineType := parseHostsLine(line)

		if lineType != "entry" { // Keep comments, empty lines, and malformed lines as is
			newLines = append(newLines, line)
			continue
		}

		// Remove lines that manage the same IP address
		if lineIP == s.IPAddress {
			modified = true
			logger.Debug("Removing existing line for IP.", "line", line)
			continue
		}

		// Remove lines where any of our target hostnames are managed by a different IP (conflict removal)
		keepLine := true
		for _, hnToRemove := range s.Hostnames {
			for _, existingName := range lineNames {
				if hnToRemove == existingName { // This hostname is now managed by s.IPAddress or removed
					modified = true
					logger.Debug("Removing existing line due to hostname conflict.", "line", line, "conflictingHostname", hnToRemove)
					keepLine = false
					break
				}
			}
			if !keepLine { break }
		}
		if keepLine {
			newLines = append(newLines, line)
		}
	}
	if errScan := scanner.Err(); errScan != nil {
		return fmt.Errorf("error scanning hosts file content: %w", errScan)
	}

	if s.State == "present" {
		newLine := fmt.Sprintf("%s\t%s", s.IPAddress, strings.Join(s.Hostnames, " "))
		// Check if the exact line (or equivalent if order doesn't matter) was already filtered out.
		// If modified is true because this IP's line was removed, we are effectively replacing it.
		// If !modified, it means this IP wasn't there, so we add it.
		newLines = append(newLines, newLine)
		logger.Info("Adding new hosts file entry.", "entry", newLine)
		modified = true // Ensure we write if we added a line, even if no old lines were removed.
	}

	if modified {
		logger.Info("Writing updated hosts file.", "path", s.HostsFilePath)
		newContent := strings.Join(newLines, "\n")
		// Ensure there's a trailing newline if content is not empty
		if newContent != "" && !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}

		errWrite := conn.CopyContent(ctx.GoContext(), newContent, s.HostsFilePath, connector.FileStat{
			Permissions: "0644", // Standard /etc/hosts permissions
			Sudo:        s.Sudo,
		})
		if errWrite != nil {
			return fmt.Errorf("failed to write updated hosts file %s on host %s: %w", s.HostsFilePath, host.GetName(), errWrite)
		}
		logger.Info("Hosts file updated successfully.")
	} else {
		logger.Info("No changes needed to hosts file.")
	}
	return nil
}

// Rollback for ManageHostsFileEntryStepSpec is complex.
// If State was "present", it means an entry was added or modified. Rollback would try to remove it.
// If State was "absent", it means an entry was removed. Rolling that back requires knowing the exact previous state.
// For simplicity, this rollback will only attempt to remove the entry if the action was "present".
func (s *ManageHostsFileEntryStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger)

	if s.State == "absent" {
		logger.Warn("Rollback for 'absent' state is not supported as it would require restoring the previous entry which is not stored by this step. Manual intervention may be needed.")
		return nil
	}

	// If state was "present", rollback means trying to remove the entry we might have added/modified.
	logger.Info("Attempting to remove/revert hosts file entry as part of rollback for 'present' state.")

	// Effectively, run the "absent" logic
	originalState := s.State
	s.State = "absent" // Temporarily change state to use Run's removal logic
	err := s.Run(ctx, host) // Re-run with "absent" state to clean up the entry
	s.State = originalState // Restore original state

	if err != nil {
		logger.Error("Error during rollback attempt to remove hosts entry (best effort).", "error", err)
		// Don't fail the rollback operation itself for this best-effort cleanup
		return nil
	}
	logger.Info("Rollback attempt to remove/revert hosts entry finished.")
	return nil
}

var _ step.Step = (*ManageHostsFileEntryStepSpec)(nil)
