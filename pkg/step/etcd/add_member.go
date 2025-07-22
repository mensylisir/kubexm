package etcd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type MemberInfo struct {
	ID         uint64   `json:"ID"`
	Name       string   `json:"name"`
	PeerURLs   []string `json:"peerURLs"`
	ClientURLs []string `json:"clientURLs"`
}

type MemberListOutput struct {
	Members []*MemberInfo `json:"members"`
}

type AddEtcdMemberStep struct {
	step.Base
	NewNode           connector.Host
	EtcdctlBinaryPath string
}

type AddEtcdMemberStepBuilder struct {
	step.Builder[AddEtcdMemberStepBuilder, *AddEtcdMemberStep]
}

func NewAddEtcdMemberStepBuilder(ctx runtime.Context, instanceName string) *AddEtcdMemberStepBuilder {
	s := &AddEtcdMemberStep{
		EtcdctlBinaryPath: filepath.Join(common.DefaultBinDir, "etcdctl"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Register a new member in the etcd cluster", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(AddEtcdMemberStepBuilder).Init(s)
	return b
}

func (b *AddEtcdMemberStepBuilder) WithNewNode(host connector.Host) *AddEtcdMemberStepBuilder {
	b.Step.NewNode = host
	return b
}

func (b *AddEtcdMemberStepBuilder) WithEtcdctlBinaryPath(path string) *AddEtcdMemberStepBuilder {
	b.Step.EtcdctlBinaryPath = path
	return b
}

func (s *AddEtcdMemberStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *AddEtcdMemberStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	if s.NewNode == nil {
		return false, fmt.Errorf("NewNode is not specified for AddEtcdMemberStep")
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	caPath, certPath, keyPath := getEtcdctlCertPaths(ctx.GetHost().GetName())
	listCmd := fmt.Sprintf("ETCDCTL_API=3 %s member list --cacert %s --cert %s --key %s",
		s.EtcdctlBinaryPath, caPath, certPath, keyPath)

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, listCmd, s.Sudo)
	if err != nil {
		logger.Warn("Failed to list etcd members during precheck, proceeding with run phase.", "error", err, "stderr", stderr)
		return false, nil
	}

	newNodePeerURL := getPeerURL(s.NewNode)
	if strings.Contains(stdout, newNodePeerURL) {
		logger.Info("New member is already part of the etcd cluster. Step is done.", "member", s.NewNode.GetName())
		return true, nil
	}

	logger.Info("New member is not yet in the etcd cluster. Step needs to run.", "member", s.NewNode.GetName())
	return false, nil
}

func (s *AddEtcdMemberStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	if s.NewNode == nil {
		return fmt.Errorf("NewNode is not specified for AddEtcdMemberStep")
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	newNodeName := s.NewNode.GetName()
	newNodePeerURL := getPeerURL(s.NewNode)
	caPath, certPath, keyPath := getEtcdctlCertPaths(ctx.GetHost().GetName())

	addCmd := fmt.Sprintf("ETCDCTL_API=3 %s member add %s --peer-urls=%s --cacert %s --cert %s --key %s",
		s.EtcdctlBinaryPath, newNodeName, newNodePeerURL, caPath, certPath, keyPath)

	logger.Info("Executing 'etcdctl member add' on current node...", "newMember", newNodeName, "peerURL", newNodePeerURL)
	_, stderr, err := runner.OriginRun(ctx.GoContext(), conn, addCmd, s.Sudo)
	if err != nil {
		if strings.Contains(stderr, "member already exists") {
			logger.Warn("etcd member already exists, but precheck did not detect it. Treating as success.", "member", newNodeName)
			return nil
		}
		return fmt.Errorf("failed to add etcd member: %w, stderr: %s", err, stderr)
	}

	logger.Info("New member has been successfully registered in the etcd cluster.", "member", newNodeName)
	return nil
}

func (s *AddEtcdMemberStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	if s.NewNode == nil {
		logger.Warn("NewNode is not specified, cannot perform rollback.")
		return nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback")
		return nil
	}

	caPath, certPath, keyPath := getEtcdctlCertPaths(ctx.GetHost().GetName())

	logger.Info("Listing members to find the ID of the new member for removal...", "member", s.NewNode.GetName())
	listCmd := fmt.Sprintf("ETCDCTL_API=3 %s member list --cacert %s --cert %s --key %s --write-out=json",
		s.EtcdctlBinaryPath, caPath, certPath, keyPath)

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, listCmd, s.Sudo)
	if err != nil {
		logger.Error(err, "Failed to list etcd members during rollback, unable to remove member.", "stderr", stderr)
		return nil
	}

	var memberList MemberListOutput
	if err := json.Unmarshal([]byte(stdout), &memberList); err != nil {
		logger.Error(err, "Failed to parse member list JSON during rollback.", "output", stdout)
		return nil
	}

	var memberID uint64
	newNodeName := s.NewNode.GetName()
	for _, member := range memberList.Members {
		if member.Name == newNodeName {
			memberID = member.ID
			break
		}
	}

	if memberID == 0 {
		logger.Warn("Could not find the new member in the cluster list, it might have been removed already. Rollback is considered complete.", "member", newNodeName)
		return nil
	}

	memberIDHex := fmt.Sprintf("%x", memberID)
	removeCmd := fmt.Sprintf("ETCDCTL_API=3 %s member remove %s --cacert %s --cert %s --key %s",
		s.EtcdctlBinaryPath, memberIDHex, caPath, certPath, keyPath)

	logger.Warn("Executing 'etcdctl member remove' to roll back the addition...", "member", newNodeName, "id", memberIDHex)
	_, stderr, err = runner.OriginRun(ctx.GoContext(), conn, removeCmd, s.Sudo)
	if err != nil {
		if strings.Contains(stderr, "member not found") {
			logger.Warn("Member was not found during removal, it might have been removed by another process. Rollback is considered complete.", "id", memberIDHex)
			return nil
		}
		logger.Error(err, "Failed to remove etcd member during rollback.", "id", memberIDHex, "stderr", stderr)
	} else {
		logger.Info("Successfully removed member from the cluster during rollback.", "member", newNodeName, "id", memberIDHex)
	}

	return nil
}

func getPeerURL(node connector.Host) string {
	peerAddress := node.GetInternalAddress()
	if peerAddress == "" {
		peerAddress = node.GetAddress()
	}
	return fmt.Sprintf("https://%s:2380", peerAddress)
}

func getEtcdctlCertPaths(nodeName string) (caPath, certPath, keyPath string) {
	caPath = filepath.Join(common.DefaultEtcdPKIDir, common.EtcdCaPemFileName)
	certPath = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName))
	keyPath = filepath.Join(common.DefaultEtcdPKIDir, fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName))
	return
}

var _ step.Step = (*AddEtcdMemberStep)(nil)
