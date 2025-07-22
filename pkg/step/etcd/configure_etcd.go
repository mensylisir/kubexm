package etcd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type EtcdConfigData struct {
	Name                     string
	DataDir                  string
	ListenPeerURLs           string
	InitialAdvertisePeerURLs string
	ListenClientURLs         string
	AdvertiseClientURLs      string
	InitialCluster           string
	InitialClusterToken      string
	InitialClusterState      string
	CertFile                 string
	KeyFile                  string
	TrustedCAFile            string
	PeerCertFile             string
	PeerKeyFile              string
	PeerTrustedCAFile        string
	SnapshotCount            string
	AutoCompactionRetention  string
}

type ConfigureEtcdStep struct {
	step.Base
	EtcdNodes               []connector.Host
	RemoteConfDir           string
	RemotePKIDir            string
	DataDir                 string
	InitialClusterToken     string
	InitialClusterState     string
	CaCertFileName          string
	SnapshotCount           string
	AutoCompactionRetention string
	renderedContent         []byte
	PermissionDir           string
	PermissionFile          string
}

type ConfigureEtcdStepBuilder struct {
	step.Builder[ConfigureEtcdStepBuilder, *ConfigureEtcdStep]
}

func NewConfigureEtcdStepBuilder(ctx runtime.Context, instanceName string) *ConfigureEtcdStepBuilder {
	s := &ConfigureEtcdStep{
		EtcdNodes:               ctx.GetHostsByRole(common.RoleEtcd),
		RemoteConfDir:           common.EtcdDefaultConfDirTarget,
		RemotePKIDir:            common.DefaultEtcdPKIDir,
		DataDir:                 common.EtcdDefaultDataDirTarget,
		InitialClusterToken:     "kubexm-etcd-cluster",
		InitialClusterState:     "new",
		CaCertFileName:          common.EtcdCaPemFileName,
		SnapshotCount:           "10000",
		AutoCompactionRetention: "8",
		PermissionDir:           "0755",
		PermissionFile:          "0644",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure etcd for current node using etcd.conf.yaml", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(ConfigureEtcdStepBuilder).Init(s)
	return b
}

func (b *ConfigureEtcdStepBuilder) WithRemoteConfDir(path string) *ConfigureEtcdStepBuilder {
	b.Step.RemoteConfDir = path
	return b
}

func (b *ConfigureEtcdStepBuilder) WithRemotePKIDir(path string) *ConfigureEtcdStepBuilder {
	b.Step.RemotePKIDir = path
	return b
}

func (b *ConfigureEtcdStepBuilder) WithDataDir(path string) *ConfigureEtcdStepBuilder {
	b.Step.DataDir = path
	return b
}

func (b *ConfigureEtcdStepBuilder) WithInitialClusterToken(token string) *ConfigureEtcdStepBuilder {
	b.Step.InitialClusterToken = token
	return b
}

func (b *ConfigureEtcdStepBuilder) WithInitialClusterState(state string) *ConfigureEtcdStepBuilder {
	b.Step.InitialClusterState = state
	return b
}

func (b *ConfigureEtcdStepBuilder) WithCaCertFileName(name string) *ConfigureEtcdStepBuilder {
	b.Step.CaCertFileName = name
	return b
}

func (b *ConfigureEtcdStepBuilder) WithSnapshotCount(count string) *ConfigureEtcdStepBuilder {
	b.Step.SnapshotCount = count
	return b
}

func (b *ConfigureEtcdStepBuilder) WithAutoCompactionRetention(retention string) *ConfigureEtcdStepBuilder {
	b.Step.AutoCompactionRetention = retention
	return b
}

func (b *ConfigureEtcdStepBuilder) WithPermissionDir(permission string) *ConfigureEtcdStepBuilder {
	b.Step.PermissionDir = permission
	return b
}

func (b *ConfigureEtcdStepBuilder) WithPermissionFile(permission string) *ConfigureEtcdStepBuilder {
	b.Step.PermissionFile = permission
	return b
}

func (s *ConfigureEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")
	content, err := s.renderConfig(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to render expected config for precheck: %w", err)
	}
	s.renderedContent = content

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	remotePath := filepath.Join(s.RemoteConfDir, "etcd.conf.yaml")

	exists, err := runner.Exists(ctx.GoContext(), conn, remotePath)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of %s: %w", remotePath, err)
	}
	if !exists {
		logger.Info("etcd.conf.yaml does not exist on remote host. Step needs to run.", "path", remotePath)
		return false, nil
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, remotePath)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			logger.Warn("Failed to read remote etcd.conf.yaml due to permission error, will re-run step to fix it.", "path", remotePath)
			return false, nil
		}
		return false, fmt.Errorf("failed to read remote file %s for content check: %w", remotePath, err)
	}

	expectedSum := sha256.Sum256(s.renderedContent)
	remoteSum := sha256.Sum256(remoteContent)

	if hex.EncodeToString(expectedSum[:]) == hex.EncodeToString(remoteSum[:]) {
		logger.Info("Remote etcd.conf.yaml is up-to-date. Step is done.", "path", remotePath)
		return true, nil
	}

	logger.Info("Remote etcd.conf.yaml content has changed. Step needs to run to update it.", "path", remotePath)
	return false, nil
}

func (s *ConfigureEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()

	if s.renderedContent == nil {
		content, err := s.renderConfig(ctx)
		if err != nil {
			return err
		}
		s.renderedContent = content
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteConfDir, s.PermissionDir, s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote config directory %s on node %s: %w", s.RemoteConfDir, ctx.GetHost().GetName(), err)
	}

	remotePath := filepath.Join(s.RemoteConfDir, "etcd.conf.yaml")
	logger.Info("Writing etcd config file", "node", ctx.GetHost().GetName(), "path", remotePath)

	if err := runner.WriteFile(ctx.GoContext(), conn, s.renderedContent, remotePath, s.PermissionFile, s.Sudo); err != nil {
		return fmt.Errorf("failed to write etcd config to %s on node %s: %w", remotePath, ctx.GetHost().GetName(), err)
	}

	logger.Info("Etcd configuration file for current node has been deployed successfully.")
	return nil
}

func (s *ConfigureEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	runner := ctx.GetRunner()

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback, unable to remove config file")
		return nil
	}

	remotePath := filepath.Join(s.RemoteConfDir, "etcd.conf.yaml")
	logger.Warn("Rolling back by removing etcd config file", "node", ctx.GetHost().GetName(), "path", remotePath)

	if err := runner.Remove(ctx.GoContext(), conn, remotePath, s.Sudo, false); err != nil {
		logger.Error(err, "Failed to remove remote config file during rollback")
	}
	return nil
}

func (s *ConfigureEtcdStep) renderConfig(ctx runtime.ExecutionContext) ([]byte, error) {
	currentHost := ctx.GetHost()
	nodeName := currentHost.GetName()

	templateContent, err := templates.Get("etcd/etcd.conf.yaml.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to get embedded etcd config template: %w", err)
	}

	var initialCluster []string
	for _, node := range s.EtcdNodes {
		peerAddress := node.GetInternalAddress()
		if peerAddress == "" {
			peerAddress = node.GetAddress()
		}
		initialCluster = append(initialCluster, fmt.Sprintf("%s=https://%s:2380", node.GetName(), peerAddress))
	}
	initialClusterStr := strings.Join(initialCluster, ",")

	listenAddress := currentHost.GetInternalAddress()
	if listenAddress == "" {
		listenAddress = currentHost.GetAddress()
	}
	advertiseAddress := currentHost.GetAddress()

	data := EtcdConfigData{
		Name:                     nodeName,
		DataDir:                  s.DataDir,
		ListenPeerURLs:           fmt.Sprintf("https://%s:2380", listenAddress),
		InitialAdvertisePeerURLs: fmt.Sprintf("https://%s:2380", advertiseAddress),
		ListenClientURLs:         fmt.Sprintf("https://%s:2379,https://127.0.0.1:2379", listenAddress),
		AdvertiseClientURLs:      fmt.Sprintf("https://%s:2379", advertiseAddress),
		InitialCluster:           initialClusterStr,
		InitialClusterToken:      s.InitialClusterToken,
		InitialClusterState:      s.InitialClusterState,
		TrustedCAFile:            filepath.Join(s.RemotePKIDir, s.CaCertFileName),
		CertFile:                 filepath.Join(s.RemotePKIDir, fmt.Sprintf(common.EtcdNodeCertFileNamePattern, nodeName)),
		KeyFile:                  filepath.Join(s.RemotePKIDir, fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, nodeName)),
		PeerTrustedCAFile:        filepath.Join(s.RemotePKIDir, s.CaCertFileName),
		PeerCertFile:             filepath.Join(s.RemotePKIDir, fmt.Sprintf(common.EtcdMemberCertFileNamePattern, nodeName)),
		PeerKeyFile:              filepath.Join(s.RemotePKIDir, fmt.Sprintf(common.EtcdMemberKeyFileNamePattern, nodeName)),
		SnapshotCount:            s.SnapshotCount,
		AutoCompactionRetention:  s.AutoCompactionRetention,
	}

	tmpl, err := template.New("etcd.conf.yaml").Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse etcd config template: %w", err)
	}
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return nil, fmt.Errorf("failed to render etcd config template for node %s: %w", nodeName, err)
	}

	return buffer.Bytes(), nil
}

var _ step.Step = (*ConfigureEtcdStep)(nil)
