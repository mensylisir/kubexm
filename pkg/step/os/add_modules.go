package os

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

type LoadKernelModulesStep struct {
	step.Base
	configFileCreated         bool
	originalConfigFileContent []byte
}

type LoadKernelModulesStepBuilder struct {
	step.Builder[LoadKernelModulesStepBuilder, *LoadKernelModulesStep]
}

func NewLoadKernelModulesStepBuilder(ctx runtime.ExecutionContext, instanceName string) *LoadKernelModulesStepBuilder {
	s := &LoadKernelModulesStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Load kernel modules", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(LoadKernelModulesStepBuilder).Init(s)
	return b
}

func (s *LoadKernelModulesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *LoadKernelModulesStep) getRequiredModules(ctx runtime.ExecutionContext) []string {
	cluster := ctx.GetClusterConfig()

	required := map[string]bool{
		"br_netfilter": true,
		"overlay":      true,
		"ip_vs":        true,
		"ip_vs_rr":     true,
		"ip_vs_wrr":    true,
		"ip_vs_sh":     true,
		"nf_conntrack": true,
		"iscsi_tcp":    true,
	}

	if cluster.Spec.System != nil && len(cluster.Spec.System.Modules) > 0 {
		for _, module := range cluster.Spec.System.Modules {
			required[module] = true
		}
	}

	var modules []string
	for module := range required {
		modules = append(modules, module)
	}
	sort.Strings(modules)

	return modules
}

func (s *LoadKernelModulesStep) getModulesConfFilePath() string {
	return common.ModulesLoadDefaultFileTarget
}

func (s *LoadKernelModulesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	requiredModules := s.getRequiredModules(ctx)

	for _, module := range requiredModules {
		checkCmd := fmt.Sprintf("lsmod | grep -w ^%s", module)
		isLoaded, _ := runner.Check(ctx.GoContext(), conn, checkCmd, s.Sudo)
		if !isLoaded {
			logger.Infof("Kernel module '%s' is not currently loaded. Will ensure configuration.", module)
			return false, nil
		}
	}

	logger.Info("All required kernel modules seem to be loaded.")

	expectedModules := s.getRequiredModules(ctx)
	var availableModulesOnSystem []string
	for _, module := range expectedModules {
		exists, _ := runner.Check(ctx.GoContext(), conn, fmt.Sprintf("modinfo %s", module), s.Sudo)
		if exists {
			availableModulesOnSystem = append(availableModulesOnSystem, module)
		}
	}
	sort.Strings(availableModulesOnSystem)
	expectedContent := strings.Join(availableModulesOnSystem, "\n") + "\n"

	filePath := s.getModulesConfFilePath()
	currentContentBytes, err := runner.ReadFile(ctx.GoContext(), conn, filePath)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "No such file or directory") {
			logger.Infof("Modules config file '%s' not found, needs to be created.", filePath)
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to read modules config file '%s'", filePath)
	}

	if string(currentContentBytes) != expectedContent {
		logger.Infof("Modules config file '%s' content is incorrect. Step needs to run.", filePath)
		return false, nil
	}

	logger.Info("Kernel modules configuration is already correct.")
	return true, nil
}

func (s *LoadKernelModulesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	filePath := s.getModulesConfFilePath()
	requiredModules := s.getRequiredModules(ctx)
	fileExists, err := runner.Exists(ctx.GoContext(), conn, filePath)
	if err != nil {
		return errors.Wrapf(err, "failed to check for existence of '%s'", filePath)
	}
	s.configFileCreated = !fileExists
	if fileExists {
		contentBytes, err := runner.ReadFile(ctx.GoContext(), conn, filePath)
		if err != nil {
			return errors.Wrapf(err, "failed to read existing file '%s' for rollback state", filePath)
		}
		s.originalConfigFileContent = contentBytes
	}
	var availableModules []string
	for _, module := range requiredModules {
		checkCmd := fmt.Sprintf("modinfo %s", module)
		exists, _ := runner.Check(ctx.GoContext(), conn, checkCmd, s.Sudo)

		if exists {
			logger.Infof("Kernel module '%s' is available. Loading with 'modprobe'...", module)
			loadCmd := fmt.Sprintf("modprobe %s", module)
			if output, err := runner.Run(ctx.GoContext(), conn, loadCmd, s.Sudo); err != nil {
				logger.Warnf("Could not modprobe '%s'. It might be built-in. Output: %s", module, output)
			}
			availableModules = append(availableModules, module)
		} else {
			logger.Warnf("Kernel module '%s' not found on the system via 'modinfo'. Skipping.", module)
		}
	}

	if len(availableModules) > 0 {
		logger.Infof("Configuring available modules to load on boot in '%s'...", filePath)
		sort.Strings(availableModules)
		content := strings.Join(availableModules, "\n") + "\n"
		permissions := fmt.Sprintf("0%o", common.DefaultConfigFilePermission)

		err = helpers.WriteContentToRemote(ctx, conn, content, filePath, permissions, s.Sudo)
		if err != nil {
			return errors.Wrapf(err, "failed to write modules config file '%s'", filePath)
		}
	} else {
		logger.Warn("None of the required kernel modules were found on this system. The config file will not be created.")
	}

	logger.Info("Kernel modules configuration completed.")
	return nil
}

func (s *LoadKernelModulesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	filePath := s.getModulesConfFilePath()

	if s.configFileCreated {
		logger.Infof("Rolling back by removing modules config file '%s'...", filePath)
		if err := runner.Remove(ctx.GoContext(), conn, filePath, s.Sudo, false); err != nil {
			return errors.Wrapf(err, "failed to remove modules config file '%s' during rollback", filePath)
		}
	} else if s.originalConfigFileContent != nil {
		logger.Infof("Rolling back by restoring original content of '%s'...", filePath)
		permissions := fmt.Sprintf("0%o", common.DefaultConfigFilePermission)
		err := helpers.WriteContentToRemote(ctx, conn, string(s.originalConfigFileContent), filePath, permissions, s.Sudo)
		if err != nil {
			return errors.Wrapf(err, "failed to restore modules config file '%s' during rollback", filePath)
		}
	} else {
		logger.Info("Skipping rollback as the modules config file was not modified by this step.")
	}

	logger.Warn("Modules loaded in the current session are not unloaded during rollback for system stability.")

	logger.Info("Kernel modules auto-load configuration rolled back.")
	return nil
}
