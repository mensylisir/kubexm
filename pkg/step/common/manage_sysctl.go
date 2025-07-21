package common

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/templates"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	sysctlTemplateKey = "os/sysctl.conf.tpl"
	sysctlTargetPath  = common.KubernetesSysctlConfFileTarget
)

type ManageSysctlStep struct {
	step.Base
	IPv6Support   bool
	CustomSysctls map[string]string
}

type ManageSysctlStepBuilder struct {
	step.Builder[ManageSysctlStepBuilder, *ManageSysctlStep]
}

func NewManageSysctlStepBuilder(ctx runtime.ExecutionContext, instanceName string) *ManageSysctlStepBuilder {
	cs := &ManageSysctlStep{}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Ensure system kernel parameters are configured", instanceName)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 2 * time.Minute
	return new(ManageSysctlStepBuilder).Init(cs)
}

func (b *ManageSysctlStepBuilder) WithIPv6(ipv6 bool) *ManageSysctlStepBuilder {
	b.Step.IPv6Support = ipv6
	return b
}

func (b *ManageSysctlStepBuilder) WithCustomSysctls(custom map[string]string) *ManageSysctlStepBuilder {
	b.Step.CustomSysctls = custom
	return b
}

func (s *ManageSysctlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageSysctlStep) render() ([]byte, error) {
	templateContent, err := templates.Get(sysctlTemplateKey)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New("sysctl").Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template string for %s: %w", sysctlTemplateKey, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return nil, fmt.Errorf("failed to execute template %s: %w", sysctlTemplateKey, err)
	}

	sysctlMap := make(map[string]string)
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			sysctlMap[key] = value
		}
	}

	if s.CustomSysctls != nil {
		for key, value := range s.CustomSysctls {
			sysctlMap[key] = value
		}
	}

	var finalContent strings.Builder
	finalContent.WriteString("# This file is managed by Kubexm. Do not edit manually.\n")
	keys := make([]string, 0, len(sysctlMap))
	for k := range sysctlMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		finalContent.WriteString(fmt.Sprintf("%s = %s\n", key, sysctlMap[key]))
	}

	return []byte(finalContent.String()), nil
}

func (s *ManageSysctlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	expectedContent, err := s.render()
	if err != nil {
		return false, err
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	currentContent, _, err := ctx.GetRunner().OriginRun(ctx.GoContext(), conn, "cat "+sysctlTargetPath, s.Sudo)
	if err != nil {
		logger.Infof("Destination file '%s' does not exist or cannot be read. Step needs to run.", sysctlTargetPath)
		return false, nil
	}

	expectedMap, _ := parseSysctlContent(expectedContent)
	currentMap, _ := parseSysctlContent([]byte(currentContent))

	if mapsAreEqual(expectedMap, currentMap) {
		logger.Infof("Content of '%s' is already up to date. Step considered done.", sysctlTargetPath)
		return true, nil
	}

	logger.Infof("Content of '%s' is outdated. Step needs to run.", sysctlTargetPath)
	return false, nil
}

func (s *ManageSysctlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	runner := ctx.GetRunner()

	content, err := s.render()
	if err != nil {
		return err
	}

	logger.Infof("Atomically writing sysctl configuration to %s", sysctlTargetPath)
	if err := AtomicWriteRemoteFile(ctx, sysctlTargetPath, content, s.Sudo); err != nil {
		return err
	}

	postCmd := "sysctl --system"
	logger.Infof("Applying new sysctl configuration by running: %s", postCmd)
	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, postCmd, s.Sudo); err != nil {
		logger.Warnf("Command '%s' finished. Stderr (may not be an error): %s", postCmd, stderr)
	}

	return nil
}

func (s *ManageSysctlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	rmCmd := fmt.Sprintf("rm -f %s", sysctlTargetPath)
	logger.Warnf("Rolling back by removing %s", sysctlTargetPath)
	_, _, err = ctx.GetRunner().OriginRun(ctx.GoContext(), conn, rmCmd, s.Sudo)
	return err
}

var _ step.Step = (*ManageSysctlStep)(nil)

func AtomicWriteRemoteFile(ctx runtime.ExecutionContext, destPath string, content []byte, sudo bool) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	destDir := filepath.Dir(destPath)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", destDir)
	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, mkdirCmd, sudo); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w, stderr: %s", destDir, err, stderr)
	}

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("mktemp -p %s", destDir), sudo)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w, stderr: %s", err, stderr)
	}
	tmpFilePath := strings.TrimSpace(stdout)

	if err := runner.WriteFile(ctx.GoContext(), conn, content, tmpFilePath, "0644", sudo); err != nil {
		runner.OriginRun(ctx.GoContext(), conn, "rm -f "+tmpFilePath, sudo)
		return fmt.Errorf("failed to write to temp file %s: %w", tmpFilePath, err)
	}

	_, stderr, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("mv -f %s %s", tmpFilePath, destPath), sudo)
	if err != nil {
		return fmt.Errorf("failed to move temp file to %s: %w, stderr: %s", destPath, err, stderr)
	}
	return nil
}

func parseSysctlContent(content []byte) (map[string]string, error) {
	sysctlMap := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			sysctlMap[key] = value
		}
	}
	return sysctlMap, scanner.Err()
}

func mapsAreEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v1 := range a {
		v2, ok := b[k]
		if !ok || v1 != v2 {
			return false
		}
	}
	return true
}
