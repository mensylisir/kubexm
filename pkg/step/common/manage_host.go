package common

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type HostAction string

const (
	ActionPresent HostAction = "present"
	ActionAbsent  HostAction = "absent"
)

type ManageHostsStep struct {
	step.Base
	Action    HostAction
	IP        string
	Hostnames []string
}

type ManageHostsStepBuilder struct {
	step.Builder[ManageHostsStepBuilder, *ManageHostsStep]
}

func NewManageHostsStepBuilder(ctx runtime.ExecutionContext, instanceName string, action HostAction, ip string, hostnames ...string) *ManageHostsStepBuilder {
	uniqueNames := make(map[string]struct{})
	finalNames := []string{}
	for _, name := range hostnames {
		if name != "" {
			if _, exists := uniqueNames[name]; !exists {
				uniqueNames[name] = struct{}{}
				finalNames = append(finalNames, name)
			}
		}
	}

	cs := &ManageHostsStep{
		Action:    action,
		IP:        ip,
		Hostnames: finalNames,
	}

	cs.Base.Meta.Name = instanceName
	descAction := "Ensuring"
	if action == ActionAbsent {
		descAction = "Removing"
	}
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>%s hosts entry for IP [%s] with hostnames [%s]", instanceName, descAction, ip, strings.Join(finalNames, ", "))
	cs.Base.Sudo = false
	cs.Base.Timeout = 2 * time.Minute
	return new(ManageHostsStepBuilder).Init(cs)
}

func (b *ManageHostsStepBuilder) WithIP(ip string) *ManageHostsStepBuilder {
	b.Step.IP = ip
	return b
}

func (b *ManageHostsStepBuilder) WithHostnames(hostnames []string) *ManageHostsStepBuilder {
	b.Step.Hostnames = hostnames
	return b
}

func (b *ManageHostsStepBuilder) WithAction(action HostAction) *ManageHostsStepBuilder {
	b.Step.Action = action
	return b
}

func (s *ManageHostsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageHostsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	hostsContent, err := s.readRemoteFile(ctx, "/etc/hosts")
	if err != nil {
		if os.IsNotExist(err) && s.Action == ActionAbsent {
			logger.Info("/etc/hosts not found, which satisfies the 'absent' state. Step considered done.")
			return true, nil
		}
		return false, fmt.Errorf("precheck: failed to read /etc/hosts: %w", err)
	}

	foundCorrectEntry := false
	foundAnyMatchingHostname := false
	hostnamesMap := s.getHostnamesMap()

	scanner := bufio.NewScanner(bytes.NewReader([]byte(hostsContent)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		lineIP := parts[0]
		lineHostnames := s.sliceToMap(parts[1:])
		for hn := range hostnamesMap {
			if _, exists := lineHostnames[hn]; exists {
				foundAnyMatchingHostname = true
				break
			}
		}
		if lineIP == s.IP && s.mapsAreEqual(hostnamesMap, lineHostnames) {
			foundCorrectEntry = true
		}
	}

	switch s.Action {
	case ActionPresent:
		if foundCorrectEntry {
			logger.Info("Correct hosts entry already exists. Step considered done.")
			return true, nil
		}
		logger.Info("Hosts entry is missing or incorrect. Step needs to run.")
		return false, nil
	case ActionAbsent:
		if !foundAnyMatchingHostname {
			logger.Info("No hosts entries found for the specified hostnames. Step considered done.")
			return true, nil
		}
		logger.Info("Found hosts entries that need to be removed. Step needs to run.")
		return false, nil
	}

	return false, fmt.Errorf("unknown action: %s", s.Action)
}

func (s *ManageHostsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	hostsContent, err := s.readRemoteFile(ctx, "/etc/hosts")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("run: failed to read /etc/hosts: %w", err)
	}
	newContent, err := s.rebuildHostsContent([]byte(hostsContent))
	if err != nil {
		return fmt.Errorf("failed to rebuild hosts content: %w", err)
	}

	logger.Info("Atomically writing updated content to /etc/hosts")
	return s.atomicWriteRemoteFile(ctx, "/etc/hosts", newContent)
}

func (s *ManageHostsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	oppositeAction := ActionPresent
	if s.Action == ActionPresent {
		oppositeAction = ActionAbsent
	}

	logger.Infof("Rolling back by performing the opposite action: '%s'", oppositeAction)

	rollbackStep := NewManageHostsStepBuilder(
		ctx,
		fmt.Sprintf("rollback-%s", s.Base.Meta.Name),
		oppositeAction,
		s.IP,
		s.Hostnames...,
	).Build()
	return rollbackStep.Run(ctx)
}

func (s *ManageHostsStep) rebuildHostsContent(originalContent []byte) ([]byte, error) {
	var newLines []string
	hostnamesToManage := s.getHostnamesMap()
	mergedHostnames := make(map[string]struct{})
	if s.Action == ActionPresent {
		for hn := range hostnamesToManage {
			mergedHostnames[hn] = struct{}{}
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(originalContent))
	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "#") || trimmedLine == "" {
			newLines = append(newLines, line)
			continue
		}
		parts := strings.Fields(trimmedLine)
		if len(parts) < 2 {
			newLines = append(newLines, line)
			continue
		}

		lineIP := parts[0]
		lineHostnamesSlice := parts[1:]

		if s.Action == ActionPresent {
			if lineIP == s.IP {
				for _, hn := range lineHostnamesSlice {
					mergedHostnames[hn] = struct{}{}
				}
				continue
			}

			for _, hn := range lineHostnamesSlice {
				if _, exists := hostnamesToManage[hn]; exists {
					return nil, fmt.Errorf(
						"hostname conflict: hostname '%s' is managed for IP %s, but already exists with IP %s",
						hn, s.IP, lineIP)
				}
			}
			newLines = append(newLines, line)

		} else {
			var keptHostnames []string
			for _, hn := range lineHostnamesSlice {
				if _, exists := hostnamesToManage[hn]; !exists {
					keptHostnames = append(keptHostnames, hn)
				}
			}

			if len(keptHostnames) > 0 {
				if len(keptHostnames) < len(lineHostnamesSlice) {
					newLine := fmt.Sprintf("%s\t%s", lineIP, strings.Join(keptHostnames, " "))
					newLines = append(newLines, newLine)
				} else {
					newLines = append(newLines, line)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if s.Action == ActionPresent {
		finalHostnames := []string{}
		for hn := range mergedHostnames {
			finalHostnames = append(finalHostnames, hn)
		}
		sort.Strings(finalHostnames)

		if len(finalHostnames) > 0 {
			expectedLine := fmt.Sprintf("%s\t%s", s.IP, strings.Join(finalHostnames, " "))
			newLines = append(newLines, expectedLine)
		}
	}

	return []byte(strings.Join(newLines, "\n") + "\n"), nil
}

func (s *ManageHostsStep) atomicWriteRemoteFile(ctx runtime.ExecutionContext, destPath string, content []byte) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("mktemp -p %s", filepath.Dir(destPath)), s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w, stderr: %s", err, stderr)
	}
	tmpFilePath := strings.TrimSpace(stdout)

	if err := helpers.WriteContentToRemote(ctx, conn, string(content), tmpFilePath, "0644", s.Sudo); err != nil {
		runner.OriginRun(ctx.GoContext(), conn, "rm -f "+tmpFilePath, s.Sudo)
		return fmt.Errorf("failed to write to temp file %s: %w", tmpFilePath, err)
	}

	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("chmod 644 %s", tmpFilePath), s.Sudo); err != nil {
		return fmt.Errorf("failed to chmod temp file %s: %w, stderr: %s", tmpFilePath, err, stderr)
	}

	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("mv -f %s %s", tmpFilePath, destPath), s.Sudo); err != nil {
		return fmt.Errorf("failed to move temp file to %s: %w, stderr: %s", destPath, err, stderr)
	}

	return nil
}

func (s *ManageHostsStep) readRemoteFile(ctx runtime.ExecutionContext, path string) (string, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return "", err
	}

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, "cat "+path, s.Sudo)

	if err != nil {
		if strings.Contains(stderr, "No such file or directory") {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("failed to read remote file %s: %w, stderr: %s", path, err, stderr)
	}

	return stdout, nil
}

func (s *ManageHostsStep) getHostnamesMap() map[string]struct{} {
	return s.sliceToMap(s.Hostnames)
}

func (s *ManageHostsStep) sliceToMap(slice []string) map[string]struct{} {
	m := make(map[string]struct{}, len(slice))
	for _, v := range slice {
		m[v] = struct{}{}
	}
	return m
}

func (s *ManageHostsStep) mapsAreEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

var _ step.Step = (*ManageHostsStep)(nil)
