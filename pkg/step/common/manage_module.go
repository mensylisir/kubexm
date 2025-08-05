package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	modulesTargetPath = common.ModulesLoadDefaultFileTarget
)

type ManageModulesStep struct {
	step.Base
	Modules       []string
	CustomModules []string
}

type ManageModulesStepBuilder struct {
	step.Builder[ManageModulesStepBuilder, *ManageModulesStep]
}

func NewManageModulesStepBuilder(ctx runtime.ExecutionContext, instanceName string) *ManageModulesStepBuilder {
	defaultModules := []string{
		"overlay",
		"br_netfilter",
		"ip_vs",
		"ip_vs_rr",
		"ip_vs_wrr",
		"ip_vs_sh",
	}
	cs := &ManageModulesStep{
		Modules: defaultModules,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Ensure system kernel modules are loaded and configured", instanceName)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 3 * time.Minute
	return new(ManageModulesStepBuilder).Init(cs)
}

func (b *ManageModulesStepBuilder) WithModules(modules []string) *ManageModulesStepBuilder {
	b.Step.Modules = modules
	return b
}

func (b *ManageModulesStepBuilder) WithCustomModules(modules []string) *ManageModulesStepBuilder {
	b.Step.CustomModules = modules
	return b
}

func (s *ManageModulesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageModulesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedModules, err := s.determineRequiredModules(ctx)
	if err != nil {
		return false, fmt.Errorf("precheck: failed to determine required modules: %w", err)
	}

	for _, module := range expectedModules {
		checkLoadedCmd := fmt.Sprintf("lsmod | grep -w ^%s", module)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, checkLoadedCmd, s.Sudo); err != nil {
			logger.Infof("Module '%s' is not loaded. Step needs to run.", module)
			return false, nil
		}
	}

	checkFileCmd := fmt.Sprintf("[ -f %s ]", modulesTargetPath)
	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, checkFileCmd, s.Sudo); err != nil {
		logger.Infof("Modules config file '%s' does not exist. Step needs to run.", modulesTargetPath)
		return false, nil
	}

	logger.Info("All required kernel modules are loaded and config file exists. Step considered done.")
	return true, nil
}

func (s *ManageModulesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	modulesToEnsure, err := s.determineRequiredModules(ctx)
	if err != nil {
		return err
	}
	logger.Infof("Final list of kernel modules to ensure: %v", modulesToEnsure)

	var loadedModules []string
	for _, module := range modulesToEnsure {
		if s.loadModule(ctx, module) {
			loadedModules = append(loadedModules, module)
		}
	}

	if len(loadedModules) > 0 {
		logger.Infof("Writing loaded modules to permanent config: %s", modulesTargetPath)
		content := "# This file is managed by Kubexm. Do not edit manually.\n" + strings.Join(loadedModules, "\n") + "\n"
		if err := atomicWriteRemoteFile(ctx, modulesTargetPath, []byte(content), s.Sudo); err != nil {
			return err
		}
	} else {
		logger.Warn("No kernel modules were successfully loaded, skipping permanent configuration.")
	}

	return nil
}

func (s *ManageModulesStep) determineRequiredModules(ctx runtime.ExecutionContext) ([]string, error) {
	allModulesMap := make(map[string]struct{})

	for _, module := range s.Modules {
		allModulesMap[module] = struct{}{}
	}
	for _, module := range s.CustomModules {
		allModulesMap[module] = struct{}{}
	}

	nfConntrackModule := "nf_conntrack"
	if s.moduleExists(ctx, "nf_conntrack_ipv4") {
		nfConntrackModule = "nf_conntrack_ipv4"
	}
	allModulesMap[nfConntrackModule] = struct{}{}
	finalModules := make([]string, 0, len(allModulesMap))
	for module := range allModulesMap {
		finalModules = append(finalModules, module)
	}
	sort.Strings(finalModules)

	return finalModules, nil
}

func (s *ManageModulesStep) moduleExists(ctx runtime.ExecutionContext, module string) bool {
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	if conn == nil {
		return false
	}

	cmd := fmt.Sprintf("modinfo %s", module)
	_, _, err := runner.OriginRun(ctx.GoContext(), conn, cmd, s.Sudo)
	return err == nil
}

func (s *ManageModulesStep) loadModule(ctx runtime.ExecutionContext, module string) bool {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	if conn == nil {
		return false
	}

	if !s.moduleExists(ctx, module) {
		logger.Warnf("Module '%s' does not exist on this system, skipping.", module)
		return false
	}

	logger.Infof("Loading kernel module: %s", module)
	cmd := fmt.Sprintf("modprobe %s", module)
	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Errorf("Failed to load module '%s'. Error: %v, Stderr: %s", module, err, stderr)
		return false
	}

	return true
}

func (s *ManageModulesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	rmCmd := fmt.Sprintf("rm -f %s", modulesTargetPath)
	logger.Warnf("Rolling back by removing %s", modulesTargetPath)
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	_, _, err = ctx.GetRunner().OriginRun(ctx.GoContext(), conn, rmCmd, s.Sudo)
	return err
}

var _ step.Step = (*ManageModulesStep)(nil)

func atomicWriteRemoteFile(ctx runtime.ExecutionContext, destPath string, content []byte, sudo bool) error {
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

	if err := helpers.WriteContentToRemote(ctx, conn, string(content), tmpFilePath, "0644", sudo); err != nil {
		runner.OriginRun(ctx.GoContext(), conn, "rm -f "+tmpFilePath, sudo)
		return fmt.Errorf("failed to write to temp file %s: %w", tmpFilePath, err)
	}

	_, stderr, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("mv -f %s %s", tmpFilePath, destPath), sudo)
	if err != nil {
		return fmt.Errorf("failed to move temp file to %s: %w, stderr: %s", destPath, err, stderr)
	}
	return nil
}
