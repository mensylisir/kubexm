package etcd

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	DefaultLeafCertExpirationThreshold = 90 * 24 * time.Hour
)

var patternsToCheck = []string{
	"member-*.pem",
	"admin-*.pem",
	"node-*.pem",
}

type CheckLeafCertsExpirationStep struct {
	step.Base
	remoteCertsDir      string
	ExpirationThreshold time.Duration
}

type CheckLeafCertsExpirationStepBuilder struct {
	step.Builder[CheckLeafCertsExpirationStepBuilder, *CheckLeafCertsExpirationStep]
}

func NewCheckLeafCertsExpirationStepBuilder(ctx runtime.Context, instanceName string) *CheckLeafCertsExpirationStepBuilder {
	s := &CheckLeafCertsExpirationStep{
		remoteCertsDir:      common.DefaultEtcdPKIDir,
		ExpirationThreshold: DefaultLeafCertExpirationThreshold,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check the expiration of etcd leaf certificates on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(CheckLeafCertsExpirationStepBuilder).Init(s)
	return b
}

func (s *CheckLeafCertsExpirationStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckLeafCertsExpirationStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Infof("Checking leaf certificates expiration")
	return false, nil
}

func (s *CheckLeafCertsExpirationStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Checking expiration for etcd leaf certificates...")

	anyCertRequiresRenewal := false

	for _, pattern := range patternsToCheck {
		remotePathPattern := filepath.Join(s.remoteCertsDir, pattern)

		findCmd := fmt.Sprintf("find %s -type f 2>/dev/null", remotePathPattern)
		stdout, err := runner.Run(ctx.GoContext(), conn, findCmd, s.Sudo)
		if err != nil {
			logger.Warnf("Could not find files for pattern '%s'. This may be expected.", remotePathPattern)
			continue
		}

		remoteFiles := strings.Fields(string(stdout))
		for _, remoteFile := range remoteFiles {
			logger.Debugf("Checking certificate: %s", remoteFile)

			certData, err := runner.ReadFile(ctx.GoContext(), conn, remoteFile)
			if err != nil {
				logger.Warnf("Failed to read remote certificate file '%s': %v", remoteFile, err)
				continue
			}

			pemBlock, _ := pem.Decode(certData)
			if pemBlock == nil {
				logger.Warnf("Failed to decode PEM block from '%s'", remoteFile)
				continue
			}

			cert, err := x509.ParseCertificate(pemBlock.Bytes)
			if err != nil {
				logger.Warnf("Failed to parse certificate from '%s': %v", remoteFile, err)
				continue
			}

			remaining := time.Until(cert.NotAfter)
			if remaining < s.ExpirationThreshold {
				remainingDays := int(remaining.Hours() / 24)
				if remaining <= 0 {
					logger.Errorf("FATAL: Leaf certificate '%s' has already expired on %s!", remoteFile, cert.NotAfter.Format("2006-01-02"))
				} else {
					logger.Warnf("Leaf certificate '%s' is expiring soon (in %d days). Renewal is required.", remoteFile, remainingDays)
				}
				anyCertRequiresRenewal = true
			}
		}
	}

	cacheKey := fmt.Sprintf(common.CacheKubexmEtcdLeafCertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	ctx.GetTaskCache().Set(cacheKey, anyCertRequiresRenewal)
	ctx.GetModuleCache().Set(cacheKey, anyCertRequiresRenewal)
	ctx.GetPipelineCache().Set(cacheKey, anyCertRequiresRenewal)

	if anyCertRequiresRenewal {
		logger.Warnf("One or more leaf certificates on this node require renewal. Result has been saved to cache ('%s': true).", cacheKey)
	} else {
		logger.Info("All leaf certificates on this node are valid. Result has been saved to cache ('%s': false).", cacheKey)
	}

	return nil
}

func (s *CheckLeafCertsExpirationStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CheckLeafCertsExpirationStep)(nil)
