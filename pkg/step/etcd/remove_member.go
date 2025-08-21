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

type RemoveEtcdMemberStep struct {
	step.Base
	NodeToRemove      connector.Host
	EtcdctlBinaryPath string
}

type RemoveEtcdMemberStepBuilder struct {
	step.Builder[RemoveEtcdMemberStepBuilder, *RemoveEtcdMemberStep]
}

func NewRemoveEtcdMemberStepBuilder(ctx runtime.Context, instanceName string) *RemoveEtcdMemberStepBuilder {
	s := &RemoveEtcdMemberStep{
		EtcdctlBinaryPath: filepath.Join(common.DefaultBinDir, "etcdctl"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove a member from the etcd cluster", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(RemoveEtcdMemberStepBuilder).Init(s)
	return b
}

func (b *RemoveEtcdMemberStepBuilder) WithNodeToRemove(host connector.Host) *RemoveEtcdMemberStepBuilder {
	b.Step.NodeToRemove = host
	return b
}

func (b *RemoveEtcdMemberStepBuilder) WithEtcdctlBinaryPath(path string) *RemoveEtcdMemberStepBuilder {
	b.Step.EtcdctlBinaryPath = path
	return b
}

func (s *RemoveEtcdMemberStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RemoveEtcdMemberStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	if s.NodeToRemove == nil {
		return false, fmt.Errorf("NodeToRemove is not specified for RemoveEtcdMemberStep")
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

	nodeToRemovePeerURL := getPeerURL(s.NodeToRemove)
	if !strings.Contains(stdout, nodeToRemovePeerURL) {
		logger.Info("Target member is not part of the etcd cluster. Step is done.", "member", s.NodeToRemove.GetName())
		return true, nil
	}

	logger.Info("Target member is still in the etcd cluster. Step needs to run.", "member", s.NodeToRemove.GetName())
	return false, nil
}

func (s *RemoveEtcdMemberStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	if s.NodeToRemove == nil {
		return fmt.Errorf("NodeToRemove is not specified for RemoveEtcdMemberStep")
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	caPath, certPath, keyPath := getEtcdctlCertPaths(ctx.GetHost().GetName())

	logger.Info("Listing members to find the ID of the member to remove...", "member", s.NodeToRemove.GetName())
	listCmd := fmt.Sprintf("ETCDCTL_API=3 %s member list --cacert %s --cert %s --key %s --write-out=json",
		s.EtcdctlBinaryPath, caPath, certPath, keyPath)

	stdout, stderr, err := runner.OriginRun(ctx.GoContext(), conn, listCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to list etcd members: %w, stderr: %s", err, stderr)
	}

	var memberList MemberListOutput
	if err := json.Unmarshal([]byte(stdout), &memberList); err != nil {
		return fmt.Errorf("failed to parse member list JSON: %w, output: %s", err, stdout)
	}

	var memberID uint64
	nodeToRemoveName := s.NodeToRemove.GetName()
	for _, member := range memberList.Members {
		if member.Name == nodeToRemoveName {
			memberID = member.ID
			break
		}
	}

	if memberID == 0 {
		logger.Warn("Could not find the target member in the cluster list, it might have been removed already. Step is successful.", "member", nodeToRemoveName)
		return nil
	}

	if len(memberList.Members) <= 2 {
		return fmt.Errorf("cannot remove member from a cluster with 2 or fewer members. This would lead to a loss of quorum")
	}

	memberIDHex := fmt.Sprintf("%x", memberID)
	removeCmd := fmt.Sprintf("ETCDCTL_API=3 %s member remove %s --cacert %s --cert %s --key %s",
		s.EtcdctlBinaryPath, memberIDHex, caPath, certPath, keyPath)

	logger.Info("Executing 'etcdctl member remove'...", "member", nodeToRemoveName, "id", memberIDHex)
	_, stderr, err = runner.OriginRun(ctx.GoContext(), conn, removeCmd, s.Sudo)
	if err != nil {
		if strings.Contains(stderr, "member not found") {
			logger.Warn("Member was not found during removal, it might have been removed by another process. Step is successful.", "id", memberIDHex)
			return nil
		}
		return fmt.Errorf("failed to remove etcd member: %w, stderr: %s", err, stderr)
	}

	logger.Info("Successfully removed member from the cluster.", "member", nodeToRemoveName, "id", memberIDHex)
	return nil
}

func (s *RemoveEtcdMemberStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	if s.NodeToRemove == nil {
		logger.Warn("NodeToRemove is not specified, cannot perform rollback.")
		return nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback")
		return nil
	}

	nodeToAddBack := s.NodeToRemove
	nodeToAddBackName := nodeToAddBack.GetName()
	nodeToAddBackPeerURL := getPeerURL(nodeToAddBack)
	caPath, certPath, keyPath := getEtcdctlCertPaths(ctx.GetHost().GetName())

	addCmd := fmt.Sprintf("ETCDCTL_API=3 %s member add %s --peer-urls=%s --cacert %s --cert %s --key %s",
		s.EtcdctlBinaryPath, nodeToAddBackName, nodeToAddBackPeerURL, caPath, certPath, keyPath)

	logger.Warn("Rolling back by re-adding the removed member...", "member", nodeToAddBackName)
	_, stderr, err := runner.OriginRun(ctx.GoContext(), conn, addCmd, s.Sudo)
	if err != nil {
		if strings.Contains(stderr, "member already exists") {
			logger.Warn("Member already exists, rollback seems complete.", "member", nodeToAddBackName)
			return nil
		}
		logger.Error(err, "Failed to re-add etcd member during rollback.", "stderr", stderr)
	} else {
		logger.Info("Successfully re-added member to the cluster during rollback.", "member", nodeToAddBackName)
	}

	return nil
}

var _ step.Step = (*RemoveEtcdMemberStep)(nil)
