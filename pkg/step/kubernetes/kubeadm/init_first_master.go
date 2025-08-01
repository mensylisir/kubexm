package kubeadm

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// InitFirstMasterStep is a step to run kubeadm init on the first master.
type InitFirstMasterStep struct {
	step.Base
}

// InitFirstMasterStepBuilder is a builder for InitFirstMasterStep.
type InitFirstMasterStepBuilder struct {
	step.Builder[InitFirstMasterStepBuilder, *InitFirstMasterStep]
}

// NewInitFirstMasterStepBuilder creates a new InitFirstMasterStepBuilder.
func NewInitFirstMasterStepBuilder(ctx runtime.Context, instanceName string) *InitFirstMasterStepBuilder {
	s := &InitFirstMasterStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run kubeadm init on the first master", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 30 * time.Minute
	b := new(InitFirstMasterStepBuilder).Init(s)
	return b
}

// Meta returns the step's metadata.
func (s *InitFirstMasterStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// Precheck checks if the cluster has already been initialized.
func (s *InitFirstMasterStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	// If admin.conf exists, we assume the cluster is initialized.
	adminConfigPath := filepath.Join(common.KubernetesConfigDir, "admin.conf")
	exists, err := runner.Exists(ctx.GoContext(), conn, adminConfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for file '%s' on host %s: %w", adminConfigPath, ctx.GetHost().GetName(), err)
	}
	if exists {
		logger.Info("Kubernetes admin.conf already exists. Step is done.")
		return true, nil
	}
	logger.Info("Kubernetes admin.conf does not exist. Step needs to run.")
	return false, nil
}

// Run executes the kubeadm init command.
func (s *InitFirstMasterStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configPath := filepath.Join(common.KubernetesConfigDir, common.KubeadmInitConfigFileName)
	// The --upload-certs flag is important for joining other control-plane nodes.
	cmd := fmt.Sprintf("kubeadm init --config %s --upload-certs", configPath)

	logger.Info("Running kubeadm init...")
	output, err := runner.SudoExec(ctx.GoContext(), conn, cmd)
	if err != nil {
		return fmt.Errorf("failed to run kubeadm init: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Kubeadm init completed successfully.")
	logger.Debugf("Kubeadm init output:\n%s", string(output))

	// After init, we need to extract the join command, token, and cert hash for other nodes.
	if err := s.parseAndStoreJoinInfo(ctx, string(output)); err != nil {
		return fmt.Errorf("failed to parse and store join info: %w", err)
	}

	return nil
}

func (s *InitFirstMasterStep) parseAndStoreJoinInfo(ctx runtime.ExecutionContext, output string) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "ParseJoinInfo")

	// Regex to find the bootstrap token
	tokenRegex := regexp.MustCompile(`--token\s+([a-z0-9.]{23})`)
	tokenMatches := tokenRegex.FindStringSubmatch(output)
	if len(tokenMatches) != 2 {
		return fmt.Errorf("could not find bootstrap token in kubeadm init output")
	}
	token := tokenMatches[1]
	ctx.Set(common.ContextKeyBootstrapToken, token)
	logger.Infof("Found and stored bootstrap token: %s", token)

	// Regex to find the CA cert hash
	hashRegex := regexp.MustCompile(`--discovery-token-ca-cert-hash\s+sha256:([a-f0-9]{64})`)
	hashMatches := hashRegex.FindStringSubmatch(output)
	if len(hashMatches) != 2 {
		return fmt.Errorf("could not find CA cert hash in kubeadm init output")
	}
	hash := "sha256:" + hashMatches[1]
	ctx.Set(common.ContextKeyCaCertHash, hash)
	logger.Infof("Found and stored CA cert hash: %s", hash)

	// Regex to find the certificate key (only for control plane join)
	keyRegex := regexp.MustCompile(`--certificate-key\s+([a-f0-9]{64})`)
	keyMatches := keyRegex.FindStringSubmatch(output)
	if len(keyMatches) == 2 {
		key := keyMatches[1]
		ctx.Set(common.ContextKeyCertificateKey, key)
		logger.Infof("Found and stored certificate key: %s", key)
	} else {
		// This is not a fatal error, as the key is not needed for worker nodes.
		logger.Warn("Could not find certificate key in kubeadm init output. This is expected if not joining other control planes.")
	}

	return nil
}

// Rollback runs kubeadm reset.
func (s *InitFirstMasterStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		// If we can't connect, we can't do much, but we shouldn't fail the rollback.
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	cmd := "kubeadm reset -f"
	logger.Warnf("Rolling back by running '%s'", cmd)
	if _, err := runner.SudoExec(ctx.GoContext(), conn, cmd); err != nil {
		logger.Errorf("Failed to run '%s' during rollback: %v", cmd, err)
	}

	return nil
}

var _ step.Step = (*InitFirstMasterStep)(nil)
