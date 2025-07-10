package os

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	etcHostsPath         = "/etc/hosts"
	managedBlockBeginTag = "# BEGIN KUBEXM MANAGED HOSTS"
	managedBlockEndTag   = "# END KUBEXM MANAGED HOSTS"
)

// UpdateEtcHostsStep adds or ensures entries in /etc/hosts within a managed block.
type UpdateEtcHostsStep struct {
	meta    spec.StepMeta
	Entries map[string][]string // IP address -> list of hostnames
	Sudo    bool
}

// NewUpdateEtcHostsStep creates a new UpdateEtcHostsStep.
func NewUpdateEtcHostsStep(instanceName string, entries map[string][]string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "UpdateEtcHostsManaged"
	}
	return &UpdateEtcHostsStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Manages /etc/hosts entries within a designated block for %d IP(s).", len(entries)),
		},
		Entries: entries,
		Sudo:    sudo,
	}
}

func (s *UpdateEtcHostsStep) Meta() *spec.StepMeta {
	return &s.meta
}

// generateManagedBlockContent creates the string content for the managed block.
func (s *UpdateEtcHostsStep) generateManagedBlockContent() string {
	if len(s.Entries) == 0 {
		return "" // Empty block if no entries
	}
	var b strings.Builder
	for ip, hostnames := range s.Entries {
		if len(hostnames) > 0 {
			b.WriteString(ip)
			for _, hn := range hostnames {
				b.WriteString("\t")
				b.WriteString(hn)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// extractPartsAndManagedBlock splits content based on begin and end markers.
// Returns: contentBeforeMarkers, currentManagedBlockContent, contentAfterMarkers, blockFound.
func extractPartsAndManagedBlock(content, beginMarker, endMarker string) (string, string, string, bool) {
	beginIdx := strings.Index(content, beginMarker)
	if beginIdx == -1 {
		return content, "", "", false // Begin marker not found, return all content as "before"
	}

	// Find the end of the beginMarker line
	endOfBeginMarkerLine := strings.Index(content[beginIdx:], "\n")
	if endOfBeginMarkerLine == -1 { // Marker is at the end of the file or on a single line without newline
		endOfBeginMarkerLine = len(content[beginIdx:])
	}
	startOfBlockContent := beginIdx + endOfBeginMarkerLine // Content starts after the newline of the begin marker

	endIdxRelative := strings.Index(content[startOfBlockContent:], endMarker)
	if endIdxRelative == -1 {
		// Begin marker found, but no end marker. This is a broken state.
		// Treat as if block is not properly formed; could return original content or error.
		// For robustness, let's assume the block extends to EOF or is just missing.
		return content[:startOfBlockContent], content[startOfBlockContent:], "", true // Return content after begin as "block"
	}
	endOfBlockContent := startOfBlockContent + endIdxRelative

	// Find the start of the endMarker line to correctly capture content before it.
	// The actual block content ends before the endMarker line begins.

	before := content[:beginIdx]
	block := content[startOfBlockContent : endOfBlockContent] // Content between markers

	// Content after the end marker (including the end marker line itself and what follows)
	after := content[endOfBlockContent:] // This starts with the end marker line

	return strings.TrimRight(before,"\n"), strings.TrimSpace(block), strings.TrimLeft(after,"\n"), true
}


func (s *UpdateEtcHostsStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, err
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, etcHostsPath)
	if err != nil {
		logger.Warn("Failed to check existence of /etc/hosts. Step will run.", "error", err)
		return false, nil
	}

	expectedBlockContent := s.generateManagedBlockContent()
	// If no entries to manage, and file doesn't exist OR doesn't contain our block, it's fine.
	// If no entries and file exists AND contains our block, we might want to remove the block (not done by this precheck).
	if expectedBlockContent == "" {
	    // If we are supposed to ensure an empty block (i.e. remove it), this logic needs adjustment.
	    // For now, if s.Entries is empty, hasManagedBlock should check if an empty block is present.
	    // This means precheck is "done" if no entries are specified AND no Kubexm block exists.
	    if !exists { return true, nil } // No file, no entries to manage, done.

	    contentBytes, _ := runnerSvc.ReadFile(ctx.GoContext(), conn, etcHostsPath)
	    _, _, _, foundOldBlock := extractPartsAndManagedBlock(string(contentBytes), managedBlockBeginTag, managedBlockEndTag)
	    if !foundOldBlock {
		    logger.Info("No entries to manage and no KubeXM managed block found in /etc/hosts.")
		    return true, nil
	    }
	    logger.Info("No entries to manage, but an existing KubeXM managed block was found. Run will clear it.")
	    return false, nil // Run will remove/empty the block
	}

	if !exists {
		logger.Info("/etc/hosts file does not exist. Step will run to create it with managed entries.")
		return false, nil
	}

	contentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, etcHostsPath)
	if err != nil {
		logger.Warn("Failed to read /etc/hosts for precheck. Step will run.", "error", err)
		return false, nil
	}

	_, currentBlockContent, _, found := extractPartsAndManagedBlock(string(contentBytes), managedBlockBeginTag, managedBlockEndTag)

	if !found {
		logger.Info("KubeXM managed block not found in /etc/hosts. Step will run to add it.")
		return false, nil
	}

	// Normalize both for comparison (e.g., consistent line endings, trim trailing newlines from blocks)
	normalizedCurrentBlock := strings.TrimSpace(strings.ReplaceAll(currentBlockContent, "\r\n", "\n"))
	normalizedExpectedBlock := strings.TrimSpace(strings.ReplaceAll(expectedBlockContent, "\r\n", "\n"))

	if normalizedCurrentBlock == normalizedExpectedBlock {
		logger.Info("All specified /etc/hosts entries already exist and match within the managed block.")
		return true, nil
	}

	logger.Info("Managed /etc/hosts block content mismatch. Step will run.",
		//"current_block", normalizedCurrentBlock, // Be careful logging potentially large content
		//"expected_block", normalizedExpectedBlock,
	)
	return false, nil
}

func (s *UpdateEtcHostsStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return err
	}

	var existingContent string
	if exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, etcHostsPath); exists {
		contentBytes, errRead := runnerSvc.ReadFile(ctx.GoContext(), conn, etcHostsPath)
		if errRead != nil {
			logger.Warn("Failed to read existing /etc/hosts. Will create new or overwrite.", "error", errRead)
		} else {
			existingContent = string(contentBytes)
		}
	}

	newManagedBlockContent := s.generateManagedBlockContent()

	var finalLines []string
	finalLines = append(finalLines, managedBlockBeginTag)
	if newManagedBlockContent != "" {
		finalLines = append(finalLines, strings.Split(strings.TrimSpace(newManagedBlockContent), "\n")...)
	}
	finalLines = append(finalLines, managedBlockEndTag)
	managedBlock := strings.Join(finalLines, "\n") + "\n"

	beforeBlock, _, afterBlock, foundBlock := extractPartsAndManagedBlock(existingContent, managedBlockBeginTag, managedBlockEndTag)

	var newFileContentBuilder strings.Builder
	if foundBlock {
		if strings.TrimSpace(beforeBlock) != "" {
			newFileContentBuilder.WriteString(beforeBlock)
			if !strings.HasSuffix(beforeBlock, "\n") { newFileContentBuilder.WriteString("\n") }
		}
		newFileContentBuilder.WriteString(managedBlock) // Includes its own trailing newline
		if strings.TrimSpace(afterBlock) != "" {
			if !strings.HasPrefix(afterBlock, "\n") { newFileContentBuilder.WriteString("\n") }
			newFileContentBuilder.WriteString(afterBlock)
		}
	} else { // No existing block, append ours to whatever was there (or create new file)
		if strings.TrimSpace(existingContent) != "" {
			newFileContentBuilder.WriteString(strings.TrimRight(existingContent, "\n") + "\n\n") // Ensure separation
		}
		newFileContentBuilder.WriteString(managedBlock)
	}

	finalOutput := strings.TrimSpace(newFileContentBuilder.String()) + "\n" // Ensure single trailing newline

	logger.Info("Updating /etc/hosts with managed block.")
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(finalOutput), etcHostsPath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write to /etc/hosts: %w", err)
	}

	logger.Info("/etc/hosts updated successfully with managed block.")
	return nil
}

func (s *UpdateEtcHostsStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	// To rollback, we would ideally restore the /etc/hosts content from before this step ran.
	// A simple rollback could be to remove the managed block.
	// This doesn't restore any entries that might have been *inside* a previous managed block
	// if the new s.Entries was different.
	// For now, remove the block if it exists.

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("rollback: failed to get connector: %w", err)
	}

	if exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, etcHostsPath); exists {
		contentBytes, errRead := runnerSvc.ReadFile(ctx.GoContext(), conn, etcHostsPath)
		if errRead != nil {
			logger.Warn("Rollback: Failed to read /etc/hosts, cannot remove managed block.", "error", errRead)
			return nil
		}
		existingContent := string(contentBytes)
		beforeBlock, _, afterBlock, foundBlock := extractPartsAndManagedBlock(existingContent, managedBlockBeginTag, managedBlockEndTag)
		if foundBlock {
			logger.Info("Rollback: Removing KubeXM managed block from /etc/hosts.")
			var newFileContentBuilder strings.Builder
			if strings.TrimSpace(beforeBlock) != "" {
				newFileContentBuilder.WriteString(strings.TrimRight(beforeBlock, "\n")+"\n")
			}
			if strings.TrimSpace(afterBlock) != "" {
				newFileContentBuilder.WriteString("\n" + strings.TrimLeft(afterBlock, "\n"))
			}

			finalOutput := strings.TrimSpace(newFileContentBuilder.String())
			if finalOutput != "" { finalOutput += "\n"}

			errWrite := runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(finalOutput), etcHostsPath, "0644", s.Sudo)
			if errWrite != nil {
				logger.Error("Rollback: Failed to write updated /etc/hosts after removing block.", "error", errWrite)
			} else {
				logger.Info("Rollback: KubeXM managed block removed from /etc/hosts.")
			}
		} else {
			logger.Info("Rollback: KubeXM managed block not found, no changes made to /etc/hosts.")
		}
	} else {
		logger.Info("Rollback: /etc/hosts file does not exist, no action taken.")
	}
	return nil
}

var _ step.Step = (*UpdateEtcHostsStep)(nil)
```
