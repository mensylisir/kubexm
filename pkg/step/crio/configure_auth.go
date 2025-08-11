// pkg/step/crio/configure_auth.go

package crio

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type authFileContent struct {
	Auths map[string]authEntry `json:"auths"`
}
type authEntry struct {
	Auth string `json:"auth"`
}

type ConfigureAuthStep struct {
	step.Base
	Auths      map[string]v1alpha1.RegistryAuth
	TargetPath string
}

type ConfigureAuthStepBuilder struct {
	step.Builder[ConfigureAuthStepBuilder, *ConfigureAuthStep]
}

func NewConfigureAuthStepBuilder(ctx runtime.Context, instanceName string) *ConfigureAuthStepBuilder {
	cfg := ctx.GetClusterConfig().Spec
	if cfg.Registry == nil || len(cfg.Registry.Auths) == 0 {
		return nil
	}

	mergedAuths := make(map[string]v1alpha1.RegistryAuth)

	if cfg.Registry != nil && cfg.Registry.Auths != nil {
		for server, auth := range cfg.Registry.Auths {
			mergedAuths[server] = auth
		}
	}

	if cfg.Kubernetes != nil && cfg.Kubernetes.ContainerRuntime != nil && cfg.Kubernetes.ContainerRuntime.Crio != nil && cfg.Kubernetes.ContainerRuntime.Crio.Auths != nil {
		for server, auth := range cfg.Kubernetes.ContainerRuntime.Crio.Auths {
			mergedAuths[string(server)] = auth
		}
	}

	s := &ConfigureAuthStep{
		Auths:      mergedAuths,
		TargetPath: common.CRIODefaultAuthFile,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure CRI-O registry authentication", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureAuthStepBuilder).Init(s)
	return b
}

func (s *ConfigureAuthStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureAuthStep) renderContent() (string, error) {
	content := authFileContent{
		Auths: make(map[string]authEntry),
	}
	for server, auth := range s.Auths {
		if auth.Auth != "" {
			content.Auths[server] = authEntry{Auth: auth.Auth}
		} else if auth.Username != "" && auth.Password != "" {
			authStr := fmt.Sprintf("%s:%s", auth.Username, auth.Password)
			encodedAuth := base64.StdEncoding.EncodeToString([]byte(authStr))
			content.Auths[server] = authEntry{Auth: encodedAuth}
		}
	}
	if len(content.Auths) == 0 {
		return "", nil
	}
	jsonBytes, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth.json content: %w", err)
	}
	return string(jsonBytes), nil
}

func (s *ConfigureAuthStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderContent()
	if err != nil {
		return false, fmt.Errorf("failed to render expected content for precheck: %w", err)
	}

	if expectedContent == "" {
		logger.Info("No authentications to configure. Step is done.")
		return true, nil
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for auth file '%s': %w", s.TargetPath, err)
	}

	if exists {
		remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.TargetPath)
		if err != nil {
			logger.Warnf("Auth file '%s' exists but failed to read, will overwrite. Error: %v", s.TargetPath, err)
			return false, nil
		}

		if string(remoteContent) == expectedContent {
			logger.Infof("Auth file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}

		logger.Infof("Auth file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("Auth file '%s' does not exist. Configuration is required.", s.TargetPath)
	return false, nil
}

func (s *ConfigureAuthStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	content, err := s.renderContent()
	if err != nil {
		return err
	}
	if content == "" {
		logger.Info("No valid authentications found to configure. Skipping.")
		return nil
	}

	targetDir := filepath.Dir(s.TargetPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create auth config directory '%s': %w", targetDir, err)
	}

	logger.Infof("Writing authentication config file to %s", s.TargetPath)
	return helpers.WriteContentToRemote(ctx, conn, content, s.TargetPath, "0600", s.Sudo)
}

func (s *ConfigureAuthStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing: %s", s.TargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Errorf("Failed to remove '%s' during rollback: %v", s.TargetPath, err)
		}
	}
	return nil
}

var _ step.Step = (*ConfigureAuthStep)(nil)
