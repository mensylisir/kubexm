package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/plan"
)

// Checkpoint tracks the execution state of a pipeline so it can be resumed after interruption.
type Checkpoint struct {
	// Version is the checkpoint format version for forward/backward compatibility.
	Version int `json:"version"`

	// ClusterName is the cluster this checkpoint belongs to.
	ClusterName string `json:"clusterName"`

	// PipelineName is the pipeline being executed.
	PipelineName string `json:"pipelineName"`

	// GraphName is the execution graph name.
	GraphName string `json:"graphName"`

	// CreatedAt is when the first checkpoint was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when this checkpoint was last updated.
	UpdatedAt time.Time `json:"updatedAt"`

	// NodeStates tracks the execution status of each node.
	// Key: NodeID string
	// Value: NodeState
	NodeStates map[string]NodeState `json:"nodeStates"`

	// ModuleStates tracks module-level progress.
	ModuleStates map[string]ModuleState `json:"moduleStates,omitempty"`

	// PipelineState captures important context values set during execution.
	// These are restored on resume so steps can continue with previous context.
	PipelineState map[string]interface{} `json:"pipelineState,omitempty"`

	// ConfigSnapshot captures a snapshot of relevant config at checkpoint time.
	// This helps validate that the same config is being used on resume.
	ConfigSnapshot *ConfigSnapshot `json:"configSnapshot,omitempty"`

	// Status is the overall pipeline status at checkpoint time.
	Status plan.Status `json:"status"`

	// LastCompletedNode is the most recently completed node ID.
	LastCompletedNode string `json:"lastCompletedNode,omitempty"`

	// ResumeCount tracks how many times this pipeline has been resumed.
	ResumeCount int `json:"resumeCount"`
}

// NodeState represents the execution state of a single graph node.
type NodeState struct {
	// Status is the current execution status of this node.
	Status plan.Status `json:"status"`

	// StartedAt is when the node started executing.
	StartedAt time.Time `json:"startedAt,omitempty"`

	// CompletedAt is when the node finished executing.
	CompletedAt time.Time `json:"completedAt,omitempty"`

	// Message holds the completion message from the step.
	Message string `json:"message,omitempty"`

	// HostStates holds per-host results for this node.
	HostStates map[string]HostState `json:"hostStates,omitempty"`

	// Output captures metadata/values exported by this step.
	Output map[string]interface{} `json:"output,omitempty"`

	// PrecheckSkipped indicates the step was skipped due to Precheck returning (true, nil).
	PrecheckSkipped bool `json:"precheckSkipped,omitempty"`
}

// HostState represents the result of running a step on a single host.
type HostState struct {
	Status    plan.Status `json:"status"`
	Message   string      `json:"message,omitempty"`
	Stdout    string      `json:"stdout,omitempty"`
	Stderr    string      `json:"stderr,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	StartTime time.Time   `json:"startTime,omitempty"`
	EndTime   time.Time   `json:"endTime,omitempty"`
}

// ModuleState tracks module-level progress for coarse-grained resume decisions.
type ModuleState struct {
	Status      plan.Status `json:"status"`
	CompletedNodes []string   `json:"completedNodes,omitempty"`
	FailedNodes  []string     `json:"failedNodes,omitempty"`
}

// ConfigSnapshot captures a hash + summary of config at checkpoint time.
type ConfigSnapshot struct {
	// ConfigHash is a short hash of the full config to detect config changes on resume.
	ConfigHash string `json:"configHash,omitempty"`
	// KubernetesVersion is the target Kubernetes version.
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
	// EtcdVersion is the target Etcd version.
	EtcdVersion string `json:"etcdVersion,omitempty"`
	// ContainerRuntime is the configured container runtime type.
	ContainerRuntime string `json:"containerRuntime,omitempty"`
}

// CheckpointPersister handles saving and loading checkpoints to/from disk.
type CheckpointPersister struct {
	// CheckpointDir is the directory where checkpoints are stored.
	CheckpointDir string
}

// NewCheckpointPersister creates a persister with the given checkpoint directory.
func NewCheckpointPersister(checkpointDir string) *CheckpointPersister {
	return &CheckpointPersister{CheckpointDir: checkpointDir}
}

// CheckpointPath returns the full path to the checkpoint file for a pipeline.
// It stores checkpoints under: {CheckpointDir}/{clusterName}/{pipelineName}.checkpoint.json
// Note: The checkpoint directory is typically the cluster-specific work dir (GlobalWorkDir/clusterName),
// so clusterName in the path provides the cluster-level directory isolation.
func (cp *CheckpointPersister) CheckpointPath(clusterName, pipelineName string) string {
	return filepath.Join(cp.CheckpointDir, fmt.Sprintf("%s.checkpoint.json", pipelineName))
}

// Save persists a checkpoint to disk.
func (cp *CheckpointPersister) Save(clusterName, pipelineName string, ckpt *Checkpoint) error {
	if ckpt == nil {
		return fmt.Errorf("checkpoint is nil")
	}

	ckpt.UpdatedAt = time.Now()
	path := cp.CheckpointPath(clusterName, pipelineName)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	data, err := json.MarshalIndent(ckpt, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint tmp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename checkpoint tmp file: %w", err)
	}

	return nil
}

// Load reads a checkpoint from disk. Returns nil if no checkpoint exists.
func (cp *CheckpointPersister) Load(clusterName, pipelineName string) (*Checkpoint, error) {
	path := cp.CheckpointPath(clusterName, pipelineName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No checkpoint exists
		}
		return nil, fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	var ckpt Checkpoint
	if err := json.Unmarshal(data, &ckpt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return &ckpt, nil
}

// Delete removes the checkpoint file.
func (cp *CheckpointPersister) Delete(clusterName, pipelineName string) error {
	path := cp.CheckpointPath(clusterName, pipelineName)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil // Already gone
	}
	return err
}

// Exists returns true if a checkpoint exists for the given pipeline.
func (cp *CheckpointPersister) Exists(clusterName, pipelineName string) bool {
	path := cp.CheckpointPath(clusterName, pipelineName)
	_, err := os.Stat(path)
	return err == nil
}

// LatestNodeStates returns the node states from the checkpoint, or empty map if none.
func FromCheckpoint(ckpt *Checkpoint) map[string]NodeState {
	if ckpt == nil {
		return nil
	}
	return ckpt.NodeStates
}
