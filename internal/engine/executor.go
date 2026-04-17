package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/internal/checkpoint"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
	"golang.org/x/sync/errgroup"
)

// ExecutorOptions configures the executor's behavior.
type ExecutorOptions struct {
	// CheckpointDir is the directory where checkpoints are stored.
	// If empty, checkpointing is disabled.
	CheckpointDir string
	// ClusterName is the cluster this execution belongs to.
	ClusterName string
	// PipelineName is the pipeline being executed.
	PipelineName string
	// DisableCheckpoint disables checkpoint saving even if CheckpointDir is set.
	DisableCheckpoint bool
	// Timeout is the maximum duration for the entire pipeline execution.
	// If zero or negative, no timeout is enforced.
	Timeout time.Duration
	// MaxRetries is the maximum number of retries for a failed step.
	// If zero, no retries are attempted. Default is 0.
	MaxRetries int
	// RetryBaseDelay is the initial delay between retries (exponential backoff).
	// Default is 1 second.
	RetryBaseDelay time.Duration
	// RetryMaxDelay is the maximum delay between retries.
	// Default is 30 seconds.
	RetryMaxDelay time.Duration
}

type dagExecutor struct {
	maxWorkers     int
	opts           ExecutorOptions
	persister      *checkpoint.CheckpointPersister
	retryMaxRetries     int
	retryBaseDelay      time.Duration
	retryMaxDelay       time.Duration
}

type workerResult struct {
	nodeID      plan.NodeID
	status      plan.Status
	message     string
	err         error
	hostResults map[string]*plan.HostResult
	output      map[string]interface{}
}

func NewExecutor() Engine {
	return &dagExecutor{
		maxWorkers: 10,
		retryMaxRetries: 0, // Disabled by default
	}
}

// NewCheckpointExecutor creates an executor with checkpoint/resume support.
func NewCheckpointExecutor(opts ExecutorOptions) Engine {
	var persister *checkpoint.CheckpointPersister
	if opts.CheckpointDir != "" && !opts.DisableCheckpoint {
		persister = checkpoint.NewCheckpointPersister(opts.CheckpointDir)
	}

	// Set retry defaults if not specified
	retryMaxRetries := opts.MaxRetries
	if retryMaxRetries <= 0 {
		retryMaxRetries = 0 // Disabled by default
	}
	retryBaseDelay := opts.RetryBaseDelay
	if retryBaseDelay <= 0 {
		retryBaseDelay = 1 * time.Second
	}
	retryMaxDelay := opts.RetryMaxDelay
	if retryMaxDelay <= 0 {
		retryMaxDelay = 30 * time.Second
	}

	return &dagExecutor{
		maxWorkers: 10,
		opts:       opts,
		persister:  persister,
		retryMaxRetries: retryMaxRetries,
		retryBaseDelay:  retryBaseDelay,
		retryMaxDelay:   retryMaxDelay,
	}
}

func (e *dagExecutor) Execute(execCtx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	result := plan.NewGraphExecutionResult(g.Name)
	log := execCtx.GetLogger()

	if dryRun {
		e.dryRun(log, g, result)
		return result, nil
	}

	// Apply pipeline-level timeout if configured
	if e.opts.Timeout > 0 {
		log.Info("Pipeline timeout configured.", "timeout", e.opts.Timeout)
		timeoutCtx, cancel := context.WithTimeout(execCtx.GoCtx, e.opts.Timeout)
		defer cancel()
		execCtx = execCtx.WithGoContext(timeoutCtx).(*runtime.Context)
	}

	log.Info("Starting execution of graph.", "graphName", g.Name, "totalNodes", len(g.Nodes))
	if err := g.Validate(); err != nil {
		result.Finalize(plan.StatusFailed, fmt.Sprintf("Graph validation failed: %v", err))
		log.Error(err, "Execution graph validation failed")
		return result, err
	}

	// Load checkpoint for resume support
	var ckpt *checkpoint.Checkpoint
	if e.persister != nil && e.opts.ClusterName != "" && e.opts.PipelineName != "" {
		var err error
		ckpt, err = e.persister.Load(e.opts.ClusterName, e.opts.PipelineName)
		if err != nil {
			log.Warn("Failed to load checkpoint, starting fresh.", "error", err)
		} else if ckpt != nil {
			log.Info("Checkpoint loaded, resuming execution.", "resumeCount", ckpt.ResumeCount, "completedNodes", len(ckpt.NodeStates))
			ckpt.ResumeCount++
		}
	}

	inDegree := make(map[plan.NodeID]int)
	dependents := make(map[plan.NodeID][]plan.NodeID)
	for id, node := range g.Nodes {
		inDegree[id] = len(node.Dependencies)
		for _, depID := range node.Dependencies {
			dependents[depID] = append(dependents[depID], id)
		}
		result.NodeResults[id] = plan.NewNodeResult(node.Name, node.StepName)

		// Restore node result from checkpoint if available
		if ckpt != nil && ckpt.NodeStates != nil {
			if nodeState, exists := ckpt.NodeStates[string(id)]; exists {
				e.restoreNodeResult(result.NodeResults[id], &nodeState)
				// Skip already-completed or skipped nodes
				if nodeState.Status == plan.StatusSuccess || nodeState.Status == plan.StatusSkipped {
					// Node already done, still need to count it toward processedNodesCount
					// but it doesn't need to run. We'll handle it in the results loop.
				}
			}
		}
	}

	// Initialize checkpoint if we don't have one loaded
	if ckpt == nil && e.persister != nil && e.opts.ClusterName != "" && e.opts.PipelineName != "" {
		ckpt = &checkpoint.Checkpoint{
			Version:       1,
			ClusterName:   e.opts.ClusterName,
			PipelineName:  e.opts.PipelineName,
			GraphName:     g.Name,
			NodeStates:    make(map[string]checkpoint.NodeState),
			Status:        plan.StatusRunning,
		}
	}

	// Save initial checkpoint
	if ckpt != nil {
		if err := e.persister.Save(e.opts.ClusterName, e.opts.PipelineName, ckpt); err != nil {
			log.Warn("Failed to save initial checkpoint.", "error", err)
		}
	}

	tasks := make(chan plan.NodeID, len(g.Nodes))
	results := make(chan workerResult, len(g.Nodes))
	var wg sync.WaitGroup

	log.Debug("Starting worker pool.", "workerCount", e.maxWorkers)
	for i := 0; i < e.maxWorkers; i++ {
		wg.Add(1)
		go e.worker(execCtx, g, &wg, tasks, results)
	}

	// Determine which nodes are already done from checkpoint
	alreadyDone := make(map[plan.NodeID]bool)
	if ckpt != nil {
		for id, state := range ckpt.NodeStates {
			if state.Status == plan.StatusSuccess || state.Status == plan.StatusSkipped {
				alreadyDone[plan.NodeID(id)] = true
			}
		}
	}

	// Count already-done nodes toward processedNodesCount
	processedNodesCount := len(alreadyDone)

	// Pre-populate task queue with entry nodes that aren't done
	initialQueueSize := 0
	for id, degree := range inDegree {
		if degree == 0 && !alreadyDone[id] {
			tasks <- id
			initialQueueSize++
		}
	}
	log.Debug("Initial tasks dispatched.", "count", initialQueueSize, "alreadyDone", len(alreadyDone))

	for processedNodesCount < len(g.Nodes) {
		res := <-results

		nodeID := res.nodeID
		nodeRes := result.NodeResults[nodeID]

		if nodeRes.Status == plan.StatusPending {
			processedNodesCount++
		}

		nodeRes.Status = res.status
		nodeRes.Message = res.message
		nodeRes.HostResults = res.hostResults
		nodeRes.EndTime = time.Now()

		if res.err != nil {
			log.Error(res.err, "Node execution encountered an error", "nodeID", nodeID, "nodeName", g.Nodes[nodeID].Name)
		}

		// Merge output into context
		if len(res.output) > 0 {
			log.Debug("Merging node output into context.", "nodeID", nodeID, "outputKeys", len(res.output))
			if execCtx.TaskState != nil {
				for k, v := range res.output {
					execCtx.TaskState.Set(k, v)
				}
			} else if execCtx.ModuleState != nil {
				for k, v := range res.output {
					execCtx.ModuleState.Set(k, v)
				}
			} else if execCtx.PipelineState != nil {
				for k, v := range res.output {
					execCtx.PipelineState.Set(k, v)
				}
			}
		}

		// Update checkpoint with node state
		if ckpt != nil {
			e.saveNodeState(ckpt, nodeID, nodeRes, res.hostResults, res.output)
			if err := e.persister.Save(e.opts.ClusterName, e.opts.PipelineName, ckpt); err != nil {
				log.Warn("Failed to save checkpoint after node completion.", "nodeID", nodeID, "error", err)
			}
		}

		log.Info("Node finished.", "nodeID", nodeID, "nodeName", g.Nodes[nodeID].Name, "status", nodeRes.Status)

		if nodeRes.Status == plan.StatusFailed || nodeRes.Status == plan.StatusSkipped {
			nodesToSkipQueue := dependents[nodeID]
			for len(nodesToSkipQueue) > 0 {
				skipID := nodesToSkipQueue[0]
				nodesToSkipQueue = nodesToSkipQueue[1:]

				skipNodeRes := result.NodeResults[skipID]
				if skipNodeRes.Status == plan.StatusPending {
					skipNodeRes.Status = plan.StatusSkipped
					skipNodeRes.Message = fmt.Sprintf("Skipped due to upstream failure/skip of node '%s'", nodeID)
					skipNodeRes.EndTime = time.Now()

					processedNodesCount++
					log.Info("Cascading skip.", "targetNodeID", skipID, "reasonNodeID", nodeID)
					nodesToSkipQueue = append(nodesToSkipQueue, dependents[skipID]...)
				}
			}
		} else {
			for _, dependentID := range dependents[nodeID] {
				inDegree[dependentID]--
				if inDegree[dependentID] == 0 {
					tasks <- dependentID
				}
			}
		}
	}

	log.Debug("All nodes processed. Shutting down workers...")
	close(tasks)
	wg.Wait()
	close(results)

	finalStatus := plan.StatusSuccess
	finalMessage := "Graph execution completed successfully."
	for _, nr := range result.NodeResults {
		if nr.Status == plan.StatusFailed {
			finalStatus = plan.StatusFailed
			finalMessage = "Graph execution failed due to one or more node failures."
			break
		}
	}
	result.Finalize(finalStatus, finalMessage)

	// Update checkpoint status and delete on success
	if ckpt != nil {
		ckpt.Status = finalStatus
		if finalStatus == plan.StatusSuccess {
			// Clean up checkpoint on successful completion
			if err := e.persister.Delete(e.opts.ClusterName, e.opts.PipelineName); err != nil {
				log.Warn("Failed to delete checkpoint after successful completion.", "error", err)
			} else {
				log.Debug("Checkpoint deleted after successful completion.")
			}
		} else {
			// Save final status in case we want to resume later
			if err := e.persister.Save(e.opts.ClusterName, e.opts.PipelineName, ckpt); err != nil {
				log.Warn("Failed to save checkpoint with final status.", "error", err)
			}
		}
	}

	log.Info("Graph execution finished.", "graphName", g.Name, "status", result.Status, "duration", result.EndTime.Sub(result.StartTime))
	return result, nil
}

// restoreNodeResult restores a node result from checkpoint state.
func (e *dagExecutor) restoreNodeResult(nodeRes *plan.NodeResult, nodeState *checkpoint.NodeState) {
	if nodeState == nil {
		return
	}
	nodeRes.Status = nodeState.Status
	nodeRes.Message = nodeState.Message
	if !nodeState.StartedAt.IsZero() {
		nodeRes.StartTime = nodeState.StartedAt
	}
	if !nodeState.CompletedAt.IsZero() {
		nodeRes.EndTime = nodeState.CompletedAt
	}

	// Restore host results
	for hostname, hostState := range nodeState.HostStates {
		hr := &plan.HostResult{
			HostName:  hostname,
			Status:    hostState.Status,
			Message:   hostState.Message,
			Stdout:    hostState.Stdout,
			Stderr:    hostState.Stderr,
			Metadata:  hostState.Metadata,
			StartTime: hostState.StartTime,
			EndTime:   hostState.EndTime,
		}
		if nodeRes.HostResults == nil {
			nodeRes.HostResults = make(map[string]*plan.HostResult)
		}
		nodeRes.HostResults[hostname] = hr
	}

	// Restore output
	if len(nodeState.Output) > 0 {
		nodeRes.Output = nodeState.Output
	}
}

// saveNodeState persists the node result into the checkpoint.
func (e *dagExecutor) saveNodeState(ckpt *checkpoint.Checkpoint, nodeID plan.NodeID, nodeRes *plan.NodeResult, hostResults map[string]*plan.HostResult, output map[string]interface{}) {
	nodeState := checkpoint.NodeState{
		Status:      nodeRes.Status,
		Message:     nodeRes.Message,
		Output:      output,
		HostStates:  make(map[string]checkpoint.HostState),
	}

	if !nodeRes.StartTime.IsZero() {
		nodeState.StartedAt = nodeRes.StartTime
	}
	if !nodeRes.EndTime.IsZero() {
		nodeState.CompletedAt = nodeRes.EndTime
	}

	// Save per-host results
	for hostname, hr := range hostResults {
		nodeState.HostStates[hostname] = checkpoint.HostState{
			Status:    hr.Status,
			Message:   hr.Message,
			Stdout:    hr.Stdout,
			Stderr:    hr.Stderr,
			Metadata:  hr.Metadata,
			StartTime: hr.StartTime,
			EndTime:   hr.EndTime,
		}
	}

	ckpt.NodeStates[string(nodeID)] = nodeState
	ckpt.LastCompletedNode = string(nodeID)
	ckpt.UpdatedAt = time.Now()
}

func (e *dagExecutor) worker(rootCtx *runtime.Context, g *plan.ExecutionGraph, wg *sync.WaitGroup, tasks <-chan plan.NodeID, results chan<- workerResult) {
	defer wg.Done()
	for nodeID := range tasks {
		if g.Nodes[nodeID] == nil {
			continue
		}
		nodeResult := e.runNode(rootCtx, g, nodeID)
		results <- nodeResult
	}
}

func (e *dagExecutor) runNode(rootCtx *runtime.Context, g *plan.ExecutionGraph, nodeID plan.NodeID) workerResult {
	node := g.Nodes[nodeID]
	log := rootCtx.GetLogger().With("nodeID", nodeID, "nodeName", node.Name, "step", node.StepName)

	// Check Condition
	if node.Condition != nil {
		shouldRun, err := node.Condition(rootCtx)
		if err != nil {
			return workerResult{
				nodeID:  nodeID,
				status:  plan.StatusFailed,
				message: fmt.Sprintf("Condition check failed: %v", err),
				err:     err,
			}
		}
		if !shouldRun {
			return workerResult{
				nodeID:  nodeID,
				status:  plan.StatusSkipped,
				message: "Skipped: Condition not met",
			}
		}
	}

	log.Info("Executing node on hosts...", "hosts", node.Hostnames)

	hostGroup, gctx := errgroup.WithContext(rootCtx.GoContext())
	hostResults := make(map[string]*plan.HostResult)
	var mu sync.Mutex

	// Create scoped context
	scopedCtx := rootCtx
	if node.PipelineName != "" {
		scopedCtx = scopedCtx.ForPipeline(node.PipelineName)
	}
	if node.ModuleName != "" {
		scopedCtx = scopedCtx.ForModule(node.ModuleName)
	}
	if node.TaskName != "" {
		scopedCtx = scopedCtx.ForTask(node.TaskName)
	}

	for _, host := range node.Hosts {
		currentHost := host
		hostGroup.Go(func() error {
			// Use scopedCtx instead of rootCtx
			execCtx := runtime.ForHost(scopedCtx, currentHost).WithGoContext(gctx)
			if rc, ok := execCtx.(*runtime.Context); ok {
				execCtx = rc.SetRuntimeConfig(node.RuntimeConfig)
			} else {
				log.Warn("Could not set runtime config: execCtx is not of type *runtime.Context")
			}
			hr := e.runStepOnHost(execCtx, node.Step)
			mu.Lock()
			hostResults[currentHost.GetName()] = hr
			mu.Unlock()
			if hr.Status == plan.StatusFailed {
				return fmt.Errorf("step '%s' on host '%s' failed: %s", node.StepName, currentHost.GetName(), hr.Message)
			}
			return nil
		})
	}

	err := hostGroup.Wait()

	tempNodeResult := &plan.NodeResult{HostResults: hostResults}
	tempNodeResult.AggregateStatus()

	var finalMessage string
	if err != nil {
		finalMessage = err.Error()
	} else {
		finalMessage = "Node completed."
	}

	// Capture Output from HostResults
	// We aggregate metadata from all hosts. Last write wins for now.
	output := make(map[string]interface{})
	for _, hr := range hostResults {
		if hr.Metadata != nil {
			for k, v := range hr.Metadata {
				output[k] = v
			}
		}
	}

	return workerResult{
		nodeID:      nodeID,
		status:      tempNodeResult.Status,
		message:     finalMessage,
		err:         err,
		hostResults: hostResults,
		output:      output,
	}
}
func (e *dagExecutor) runStepOnHost(ctx runtime.ExecutionContext, s step.Step) *plan.HostResult {
	host := ctx.GetHost()
	hr := plan.NewHostResult(host.GetName())
	hr.EndTime = time.Now()
	log := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName())

	// Enforce step-level timeout, defaulting to 5 minutes if not set.
	timeout := s.GetBase().Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	stepCtx := ctx.WithTimeout(timeout)
	log = log.With("timeout", timeout)

	// Ensure cancelFn is called to release resources
	if sc, ok := stepCtx.(interface{ Cancel() }); ok {
		defer sc.Cancel()
	}

	// Panic recovery: prevents a step panic from crashing the worker goroutine.
	var isDone bool
	var preErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				preErr = fmt.Errorf("PANIC in Precheck of step '%s': %v", s.Meta().Name, r)
				log.Error(fmt.Errorf("panic"), "Step panicked during Precheck", "panic", r)
			}
		}()
		isDone, preErr = s.Precheck(stepCtx)
	}()
	if preErr != nil {
		hr.Status = plan.StatusFailed
		hr.Message = fmt.Sprintf("Precheck failed: %v", preErr)
		hr.EndTime = time.Now()
		log.Error(preErr, "Step precheck failed.")
		return hr
	}
	if isDone {
		hr.Status = plan.StatusSuccess
		hr.Message = "Skipped: Precheck condition already met."
		hr.EndTime = time.Now()
		log.Info("Step skipped by precheck (treated as success).")
		return hr
	}
	log.Debug("Precheck passed, executing step.")

	hr.Status = plan.StatusRunning
	log.Info("Running step...")

	var result *types.StepResult
	var runErr error

	// Execute step with retry logic
	for attempt := 0; attempt <= e.retryMaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := e.retryBaseDelay * time.Duration(1<<uint(attempt-1))
			if delay > e.retryMaxDelay {
				delay = e.retryMaxDelay
			}
			log.Info("Retrying step...", "attempt", attempt, "max_retries", e.retryMaxRetries, "delay", delay)
			time.Sleep(delay)
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					hr.Status = plan.StatusFailed
					hr.Message = fmt.Sprintf("PANIC in step '%s': %v", s.Meta().Name, r)
					hr.EndTime = time.Now()
					log.Error(fmt.Errorf("panic"), "Step panicked during Run", "panic", r)
					// Still attempt rollback on panic.
					if rbErr := s.Rollback(stepCtx); rbErr != nil {
						hr.Message = fmt.Sprintf("%s. Rollback failed: %v", hr.Message, rbErr)
						log.Error(rbErr, "Rollback failed after panic.")
					} else {
						log.Info("Rollback successful after panic.")
					}
				}
			}()
			result, runErr = s.Run(stepCtx)
		}()

		// If successful or no error, break out of retry loop
		if runErr == nil {
			break
		}

		// Don't retry if IgnoreError is set (will be handled below)
		if s.GetBase().IgnoreError {
			break
		}

		// Don't retry if this is the last attempt
		if attempt >= e.retryMaxRetries {
			log.Warn("Max retries exceeded for step.", "attempt", attempt, "max_retries", e.retryMaxRetries)
			break
		}

		// Reset host result for retry
		hr = plan.NewHostResult(host.GetName())
		hr.Status = plan.StatusRunning
		log.Warn("Step failed, will retry.", "error", runErr, "attempt", attempt)
	}

	hr.EndTime = time.Now()

	if runErr != nil {
		if s.GetBase().IgnoreError {
			hr.Status = plan.StatusSuccess
			hr.Message = fmt.Sprintf("Run failed but error ignored: %v", runErr)
			log.Warn("Step run failed but IgnoreError is set. Continuing.", "error", runErr)
		} else {
			hr.Status = plan.StatusFailed
			hr.Message = fmt.Sprintf("Run failed: %v", runErr)
			log.Error(runErr, "Step run failed.")
			log.Info("Attempting rollback...")
			if rbErr := s.Rollback(stepCtx); rbErr != nil {
				hr.Message = fmt.Sprintf("%s. Rollback failed: %v", hr.Message, rbErr)
				log.Error(rbErr, "Rollback failed.")
			} else {
				log.Info("Rollback successful.")
			}
		}
		return hr
	}

	if result != nil {
		switch result.Status {
		case types.StepStatusSkipped:
			hr.Status = plan.StatusSuccess
			hr.Message = result.Message
			hr.Metadata = result.Metadata
		case types.StepStatusCompleted:
			hr.Status = plan.StatusSuccess
			hr.Message = result.Message
			hr.Metadata = result.Metadata
			if result.Artifacts != nil {
				for _, artifact := range result.Artifacts {
					hr.AddArtifact(artifact)
				}
			}
		case types.StepStatusFailed:
			hr.Status = plan.StatusFailed
			hr.Message = result.Message
			hr.Metadata = result.Metadata
		default:
			hr.Status = plan.StatusSuccess
			hr.Message = "Step executed successfully."
		}
	} else {
		hr.Status = plan.StatusSuccess
		hr.Message = "Step executed successfully."
	}

	log.Info("Step executed successfully.", "status", hr.Status)
	return hr
}

func (e *dagExecutor) dryRun(logger *logger.Logger, g *plan.ExecutionGraph, result *plan.GraphExecutionResult) {
	logger.Info("--- Dry Run Execution Graph ---", "graphName", g.Name)
	for id, node := range g.Nodes {
		nodeRes := plan.NewNodeResult(node.Name, node.StepName)
		nodeRes.Status = plan.StatusSkipped
		nodeRes.Message = "Dry run: Node execution skipped."
		nodeRes.StartTime = time.Now()
		nodeRes.EndTime = time.Now()

		hostNames := make([]string, len(node.Hosts))
		logHostNames := make([]string, len(node.Hosts))
		for i, h := range node.Hosts {
			hostNames[i] = h.GetName()
			logHostNames[i] = h.GetName()
			hr := plan.NewHostResult(h.GetName())
			hr.Status = plan.StatusSkipped
			hr.Message = "Dry run: Host operation skipped."
			hr.StartTime = nodeRes.StartTime
			hr.EndTime = nodeRes.EndTime
			nodeRes.HostResults[h.GetName()] = hr
		}
		result.NodeResults[id] = nodeRes

		logger.Info("Node (Dry Run)",
			"id", id,
			"name", node.Name,
			"step", node.StepName,
			"hosts", logHostNames,
			"dependencies", node.Dependencies,
		)
		fmt.Printf("  Node: %s (ID: %s)\n", node.Name, id)
		fmt.Printf("  Step: %s\n", node.StepName)
		fmt.Printf("  Hosts: %v\n", hostNames)
		fmt.Printf("  Dependencies: %v\n", node.Dependencies)
		fmt.Printf("  Status: %s (Dry Run)\n", plan.StatusSkipped)
	}
	logger.Info("--- End of Dry Run ---")
}

var _ Engine = &dagExecutor{}

// NewCheckpointExecutorForPipeline creates a checkpoint-aware executor for the given pipeline.
// It uses the engine context's cluster work dir as the checkpoint directory.
func NewCheckpointExecutorForPipeline(engineCtx *runtime.Context, pipelineName string) Engine {
	if engineCtx == nil || engineCtx.ClusterConfig == nil || engineCtx.ClusterConfig.Name == "" {
		// Fall back to non-checkpoint executor if context is incomplete
		return &dagExecutor{maxWorkers: 10}
	}
	// Checkpoints are stored at the cluster work dir level (GlobalWorkDir/clusterName/)
	// so that different clusters don't share checkpoint files.
	clusterWorkDir := engineCtx.GetClusterWorkDir()
	if clusterWorkDir == "" || clusterWorkDir == "/_INVALID_CLUSTER_" {
		clusterWorkDir = "/tmp/kubexm"
	}
	opts := ExecutorOptions{
		CheckpointDir: clusterWorkDir,
		ClusterName:   engineCtx.ClusterConfig.Name,
		PipelineName:  pipelineName,
	}
	return NewCheckpointExecutor(opts)
}
