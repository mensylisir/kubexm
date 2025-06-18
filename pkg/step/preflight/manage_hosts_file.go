package preflight

import (
	"fmt"
	"sort"
	"strings"
	"time"

	// "github.com/kubexms/kubexms/pkg/connector" // Not directly used, but for consistency
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// UpdateHostsFileStepSpec defines the specification for updating the /etc/hosts file.
type UpdateHostsFileStepSpec struct {
	HostEntries []string `json:"hostEntries,omitempty"`
	BeginMarker string   `json:"beginMarker,omitempty"`
	EndMarker   string   `json:"endMarker,omitempty"`
}

// GetName returns the name of the step.
func (s *UpdateHostsFileStepSpec) GetName() string {
	return "Update /etc/hosts file"
}

// PopulateDefaults fills the spec with default values.
func (s *UpdateHostsFileStepSpec) PopulateDefaults() {
	if len(s.HostEntries) == 0 {
		s.HostEntries = []string{
			"192.168.122.101  vm1.cluster.local vm1",
			"172.30.1.102  registry.cluster.local registry",
			"192.168.122.102  vm2.cluster.local vm2",
			"192.168.122.103  vm3.cluster.local vm3",
			"172.30.1.102  dockerhub.kubekey.local", // Note: Same IP as registry
			"192.168.122.101  lb.kubesphere.local",   // Note: Same IP as vm1
		}
	}
	if s.BeginMarker == "" {
		s.BeginMarker = "# KUBEXMS HOSTS BEGIN"
	}
	if s.EndMarker == "" {
		s.EndMarker = "# KUBEXMS HOSTS END"
	}
}

// UpdateHostsFileStepExecutor implements the logic for updating the /etc/hosts file.
type UpdateHostsFileStepExecutor struct{}

// Check determines if the /etc/hosts file is already correctly configured.
func (e *UpdateHostsFileStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	spec, ok := s.(*UpdateHostsFileStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T", s)
	}
	spec.PopulateDefaults()
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	hostsFilePath := "/etc/hosts"
	contentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, hostsFilePath)
	if err != nil {
		hostCtxLogger.Warnf("Failed to read %s: %v. Assuming configuration is not done.", hostsFilePath, err)
		return false, nil // Cannot read, so not done.
	}
	currentLines := strings.Split(string(contentBytes), "\n")

	var blockLines []string
	inBlock := false
	for _, line := range currentLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == spec.BeginMarker {
			inBlock = true
			continue
		}
		if trimmedLine == spec.EndMarker {
			inBlock = false
			continue
		}
		if inBlock {
			// Normalize by trimming, but keep original for exactness if needed.
			// For comparison, it's better to compare normalized forms.
			if trimmedLine != "" { // Ignore empty lines within the block for comparison
				blockLines = append(blockLines, trimmedLine)
			}
		}
	}

	// Normalize spec.HostEntries for comparison (trimming and sorting)
	normalizedSpecEntries := []string{}
	for _, entry := range spec.HostEntries {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" {
			normalizedSpecEntries = append(normalizedSpecEntries, trimmed)
		}
	}
	sort.Strings(normalizedSpecEntries)
	sort.Strings(blockLines) // Also sort actual block lines for comparison

	if len(normalizedSpecEntries) != len(blockLines) {
		hostCtxLogger.Infof("Number of entries in %s block (%d) does not match expected (%d). Expected: %v, Actual in block: %v",
			hostsFilePath, len(blockLines), len(normalizedSpecEntries), normalizedSpecEntries, blockLines)
		return false, nil
	}

	for i := range normalizedSpecEntries {
		if normalizedSpecEntries[i] != blockLines[i] {
			hostCtxLogger.Infof("Hosts file block content mismatch. Expected: '%s', Found in block: '%s'. Full expected: %v, Full actual in block: %v",
				normalizedSpecEntries[i], blockLines[i], normalizedSpecEntries, blockLines)
			return false, nil
		}
	}

	hostCtxLogger.Infof("/etc/hosts file block is correctly configured between markers '%s' and '%s'.", spec.BeginMarker, spec.EndMarker)
	return true, nil
}

// Execute updates the /etc/hosts file.
func (e *UpdateHostsFileStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	spec, ok := s.(*UpdateHostsFileStepSpec)
	if !ok {
		myErr := fmt.Errorf("Execute: unexpected spec type %T", s)
		stepName := "UpdateHostsFile (type error)"; if s != nil { stepName = s.GetName() }
		return step.NewResult(stepName, ctx.Host.Name, time.Now(), myErr)
	}
	spec.PopulateDefaults()

	stepName := spec.GetName()
	startTime := time.Now()
	res := step.NewResult(stepName, ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Sugar()

	hostsFilePath := "/etc/hosts"

	// Read current /etc/hosts content
	currentContentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, hostsFilePath)
	if err != nil {
		// If file doesn't exist, we can treat it as empty and create it.
		// However, /etc/hosts should always exist.
		hostCtxLogger.Warnf("Failed to read %s: %v. Proceeding as if it's empty or will be created.", hostsFilePath, err)
		currentContentBytes = []byte{} // Treat as empty
	}
	currentLines := strings.Split(string(currentContentBytes), "\n")

	// Remove existing block and prepare new content
	newLines := []string{}
	inBlock := false
	for _, line := range currentLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == spec.BeginMarker {
			inBlock = true
			continue // Skip begin marker, will add new one
		}
		if trimmedLine == spec.EndMarker {
			inBlock = false
			continue // Skip end marker, will add new one
		}
		if !inBlock {
			newLines = append(newLines, line)
		}
	}

	// Construct the new block
	var newBlockContent strings.Builder
	newBlockContent.WriteString(spec.BeginMarker + "\n")
	for _, entry := range spec.HostEntries {
		newBlockContent.WriteString(strings.TrimSpace(entry) + "\n")
	}
	newBlockContent.WriteString(spec.EndMarker + "\n")

	// Combine existing content (outside markers) with the new block.
	// We need to ensure there are no excessive blank lines, especially at the end or where block was.

	finalContentBuilder := strings.Builder{}
	hasContent := false
	// Add lines that were outside any previous block
	for _, line := range newLines {
		// Avoid adding multiple empty lines if the original content ended with them,
		// or if removing the block left consecutive newlines.
		// This logic is basic; a more robust approach might trim all trailing newlines then add one back.
		if strings.TrimSpace(line) == "" {
			if hasContent && !strings.HasSuffix(finalContentBuilder.String(), "\n\n") { // Avoid more than two newlines
				finalContentBuilder.WriteString(line + "\n")
			}
		} else {
			finalContentBuilder.WriteString(line + "\n")
			hasContent = true
		}
	}

	// Remove trailing newline from existing content if it's just newlines, before appending block
	currentFinalContent := strings.TrimRight(finalContentBuilder.String(), "\n")
	finalContentBuilder.Reset()
	finalContentBuilder.WriteString(currentFinalContent)
	if currentFinalContent != "" { // Add a newline if there was content before the block
		finalContentBuilder.WriteString("\n")
	}

	finalContentBuilder.WriteString(newBlockContent.String())


	// Write the new content back to /etc/hosts
	// Runner.WriteFile needs sudo for /etc/hosts
	hostCtxLogger.Infof("Writing updated content to %s", hostsFilePath)
	if err := ctx.Host.Runner.WriteFile(ctx.GoContext, []byte(finalContentBuilder.String()), hostsFilePath, "0644", true); err != nil {
		res.Error = fmt.Errorf("failed to write updated %s: %w", hostsFilePath, err)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Infof("%s updated successfully.", hostsFilePath)

	// Perform a post-execution check
	done, checkErr := e.Check(s, ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed verification: %v", res.Error); return res
	}
	if !done {
		errMsg := "post-execution check indicates /etc/hosts file is not correctly configured"
		res.Error = fmt.Errorf(errMsg)
		res.SetFailed(errMsg); hostCtxLogger.Errorf("Step failed verification: %s", errMsg); return res
	}

	res.SetSucceeded("/etc/hosts file updated successfully.")
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&UpdateHostsFileStepSpec{}), &UpdateHostsFileStepExecutor{})
}
