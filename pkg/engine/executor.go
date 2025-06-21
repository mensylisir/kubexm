package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// dagExecutor implements the Engine interface for DAG-based execution.
type dagExecutor struct {
	// maxWorkers specifies the maximum number of nodes that can be executed concurrently.
	// If 0, it defaults to a reasonable number, e.g., number of CPUs or a fixed value like 10.
	maxWorkers int
}

// NewExecutor creates a new DAG-based execution engine.
// It can be extended to take configuration, like maxWorkers.
func NewExecutor() Engine {
	return &dagExecutor{
		maxWorkers: 10, // Default to 10 concurrent node executions
	}
}

// Execute processes the execution graph.
func (e *dagExecutor) Execute(rtCtx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	result := plan.NewGraphExecutionResult(g.Name)

	if dryRun {
		e.dryRun(rtCtx, g, result)
		result.EndTime = time.Now()
		// For dry run, status is success if no error generating the dry run output.
		// Individual nodes will be marked skipped.
		overallStatus := plan.StatusSuccess
		anyNodeFailedDryRun := false
		for _, nr := range result.NodeResults {
			if nr.Status == plan.StatusFailed {
				anyNodeFailedDryRun = true
				break
			}
		}
		if anyNodeFailedDryRun {
			overallStatus = plan.StatusFailed
		} else if len(g.Nodes) > 0 && len(result.NodeResults) == 0 {
			// if there were nodes but no results, something is off (should be skipped results)
			// but for now, if no failures, it's success.
		} else if len(g.Nodes) == 0 {
			// Empty graph is success
		}

		result.Status = overallStatus
		rtCtx.Logger.Info("Dry run complete.", "graphName", g.Name, "status", result.Status)
		return result, nil
	}

	rtCtx.Logger.Info("Starting execution of graph.", "graphName", g.Name, "totalNodes", len(g.Nodes))
	result.Status = plan.StatusRunning

	// 1. Validate graph
	if err := g.Validate(); err != nil {
		result.Status = plan.StatusFailed
		result.EndTime = time.Now()
		rtCtx.Logger.Error(err, "Execution graph validation failed")
		// Potentially add a global error message to result if such a field exists
		return result, fmt.Errorf("graph validation failed: %w", err)
	}
	rtCtx.Logger.V(1).Info("Execution graph validated successfully.")

	// 2. Initialize data structures for execution
	inDegree := make(map[plan.NodeID]int)
	dependents := make(map[plan.NodeID][]plan.NodeID) // Stores which nodes depend on a given node (reverse of Dependencies)
	queue := make([]plan.NodeID, 0)
	nodeResultsLock := new(sync.Mutex) // To safely update result.NodeResults

	for id, node := range g.Nodes {
		inDegree[id] = len(node.Dependencies)
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
		for _, depID := range node.Dependencies {
			dependents[depID] = append(dependents[depID], id)
		}
		// Initialize NodeResult for all nodes
		result.NodeResults[id] = plan.NewNodeResult(node.Name, node.StepName)
	}

	if len(queue) == 0 && len(g.Nodes) > 0 {
		// This case should be caught by Validate if it means a cycle with no entry points.
		// Or it's an empty graph (handled if len(g.Nodes) == 0).
		msg := "no entry nodes found in a non-empty graph, possibly a cycle or misconfiguration"
		rtCtx.Logger.Error(nil, msg) // Use nil for error if it's a logical one like this
		result.Status = plan.StatusFailed
		result.EndTime = time.Now()
		return result, fmt.Errorf(msg)
	}
	rtCtx.Logger.V(1).Info("Initial execution queue populated.", "queueSize", len(queue), "initialQueue", queue)


	// 3. Execution loop
	var wg sync.WaitGroup
	// Semaphore to limit concurrent node executions
	sem := make(chan struct{}, e.maxWorkers)

	// activeWorkers tracks nodes currently being processed or queued to process by a worker.
	// This is subtly different from just nodes in 'queue'.
	// We need a way to know when all possible work is done.
	processedNodesCount := 0
	// Mutex to protect queue, inDegree, processedNodesCount, and dependents map (if modified dynamically, though dependents is static here)
	mu := sync.Mutex{}

	// failedNodes stores IDs of nodes that have failed, to propagate skipping.
	failedNodes := make(map[plan.NodeID]bool)

	for {
		mu.Lock()
		if len(queue) == 0 && processedNodesCount == len(g.Nodes) {
			// All nodes processed, and queue is empty. Exit condition.
			// This also handles the case of an empty graph from the start.
			mu.Unlock()
			break
		}

		// Propagate failures before dispatching new work
		// This needs to be efficient. If a node fails, all its transitive dependents are skipped.
		// We can do this when a node actually fails, rather than in the main loop.

		if len(queue) == 0 && processedNodesCount < len(g.Nodes) {
			// No nodes in queue, but not all nodes processed.
			// This implies a deadlock (cycle not caught by validate?) or all remaining nodes are skipped due to earlier failures.
			// If all remaining nodes have inDegree > 0 because their prerequisites failed and were marked,
			// they should eventually be processed by the skip propagation logic.
			// This state suggests either a bug in cycle detection, skip propagation, or in-degree logic.
			// For now, let's assume this might happen if all remaining items are to be skipped.
			// The loop should break if all nodes are either Success, Failed, or Skipped.
			allDoneOrSkipped := true
			for id := range g.Nodes {
				nodeRes := result.NodeResults[id]
				if nodeRes.Status == plan.StatusPending || nodeRes.Status == plan.StatusRunning {
					allDoneOrSkipped = false
					break
				}
			}
			if allDoneOrSkipped {
				mu.Unlock()
				break // All nodes are accounted for.
			}
			// If not all done/skipped, and queue is empty, it's an issue.
			// Log this potential deadlock or issue.
			rtCtx.Logger.Info("Execution queue is empty, but not all nodes are processed and some are pending/running. Possible deadlock or issue.", "processedCount", processedNodesCount, "totalNodes", len(g.Nodes))
			mu.Unlock()
			// Give some time for any running tasks to finish and potentially populate the queue.
			// This is a failsafe; ideally, worker completion logic handles this.
			time.Sleep(100 * time.Millisecond)
			continue
		}


		if len(queue) > 0 {
			nodeID := queue[0]
			queue = queue[1:]
			mu.Unlock()

			// Check if this node should be skipped due to failed dependencies
			nodeToExecute := g.Nodes[nodeID]
			skipNode := false
			var skipReason string
			for _, depID := range nodeToExecute.Dependencies {
				mu.Lock() // Protect access to failedNodes map
				isFailed := failedNodes[depID]
				mu.Unlock()
				if isFailed {
					skipNode = true
					skipReason = fmt.Sprintf("Skipped due to failed dependency '%s' (%s)", depID, g.Nodes[depID].Name)
					break
				}
			}

			if skipNode {
				mu.Lock()
				nodeRes := result.NodeResults[nodeID]
				nodeRes.Status = plan.StatusSkipped
				nodeRes.Message = skipReason
				nodeRes.StartTime = time.Now() // Mark start time even for skipped
				nodeRes.EndTime = time.Now()

				rtCtx.Logger.Info("Skipping node", "nodeID", nodeID, "nodeName", nodeToExecute.Name, "reason", skipReason)
				// Propagate this skip/failure status to its own dependents
				e.propagateSkip(rtCtx, g, nodeID, dependents, result, failedNodes, &mu, "transitively skipped") // Mark this node as 'failed' for propagation

				processedNodesCount++
				mu.Unlock()
				continue // Go to next iteration of the main loop
			}


			wg.Add(1)
			sem <- struct{}{} // Acquire semaphore

			go func(id plan.NodeID) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore

				node := g.Nodes[id]
				nodeRes := result.NodeResults[id] // Already initialized

				rtCtx.Logger.Info("Executing node", "nodeID", id, "nodeName", node.Name, "step", node.StepName, "hosts", node.HostNames)
				nodeRes.Status = plan.StatusRunning
				// nodeRes.StartTime is already set when results map was populated

				// Execute step on all hosts for this node
				// Create a new Go context for this node that can be cancelled
				// if any host operation within the node fails catastrophically (not typical for steps).
				// runtime.Context itself has a GoContext, use it as parent.
				nodeGoCtx, nodeCancel := context.WithCancel(rtCtx.GoContext())
				defer nodeCancel() // Ensure cancellation signal is propagated if node func returns early

				hostGroup, hostCtxGroup := errgroup.WithContext(nodeGoCtx)

				for _, host := range node.Hosts {
					currentHost := host // Capture range variable
					hostGroup.Go(func() error {
						stepRuntimeCtx := runtime.NewContextWithGoContext(hostCtxGroup, rtCtx) // Create StepContext from hostCtxGroup
						hostRes := e.runStepOnHost(stepRuntimeCtx, currentHost, node.Step)

						nodeResLock.Lock()
						nodeRes.HostResults[currentHost.GetName()] = hostRes
						nodeResLock.Unlock()

						if hostRes.Status == plan.StatusFailed {
							// If one host fails, the entire node is considered failed.
							// Return an error to cancel other hosts for this node via hostCtxGroup.
							return fmt.Errorf("step '%s' on host '%s' failed: %s", node.StepName, currentHost.GetName(), hostRes.Message)
						}
						return nil
					})
				}

				nodeFailed := false
				if err := hostGroup.Wait(); err != nil {
					// At least one host failed. The node is marked as failed.
					nodeRes.Message = err.Error() // Main error for the node
					nodeFailed = true
					rtCtx.Logger.Error(err, "Node execution failed", "nodeID", id, "nodeName", node.Name)
				}

				// Determine node status based on host results
				// If nodeFailed is true, it's definitely StatusFailed.
				// Otherwise, if all hosts succeeded or were skipped (by precheck), it's Success.
				// If all hosts were skipped by precheck, node is Skipped.
				if nodeFailed {
					nodeRes.Status = plan.StatusFailed
				} else {
					allHostsSkippedByPrecheck := true
					anyHostSucceeded := false
					for _, hr := range nodeRes.HostResults {
						if hr.Status == plan.StatusFailed { // Should not happen if nodeFailed is false
							nodeFailed = true // Should have been caught by hostGroup.Wait()
							break
						}
						if hr.Status == plan.StatusSuccess {
							anyHostSucceeded = true
							allHostsSkippedByPrecheck = false // If one succeeded, not all were skipped
						}
						if hr.Status != plan.StatusSkipped { // A host ran and didn't get skipped by precheck
							allHostsSkippedByPrecheck = false
						}
					}
					if nodeFailed { // Re-check after iterating host results
						nodeRes.Status = plan.StatusFailed
					} else if allHostsSkippedByPrecheck && len(nodeRes.HostResults) > 0 {
						nodeRes.Status = plan.StatusSkipped
						nodeRes.Message = "All hosts skipped by precheck."
					} else if anyHostSucceeded || len(nodeRes.HostResults) == 0 { // No hosts means success for the node
						nodeRes.Status = plan.StatusSuccess
						nodeRes.Message = "Node executed successfully."
					} else {
						// This case implies some hosts were pending or running, which shouldn't happen here.
						// Or all were skipped by precheck but some hosts were not defined.
						// If all hosts were defined and skipped, it's StatusSkipped.
						// If no hosts, it's success. If some hosts succeeded, it's success.
						// Default to success if no explicit failure or full skip.
						// This needs careful thought for partially skipped scenarios.
						// For now, if not failed and not all skipped, consider it success.
						nodeRes.Status = plan.StatusSuccess
						nodeRes.Message = "Node completed; some hosts may have been skipped by precheck."

					}
				}
				nodeRes.EndTime = time.Now()


				mu.Lock()
				defer mu.Unlock() // Ensure unlock before any continue/return from the goroutine's main path

				processedNodesCount++
				rtCtx.Logger.Info("Node finished", "nodeID", id, "nodeName", node.Name, "status", nodeRes.Status, "duration", nodeRes.EndTime.Sub(nodeRes.StartTime))


				if nodeRes.Status == plan.StatusFailed || nodeRes.Status == plan.StatusSkipped { // Treat skipped by precheck as non-failure for propagation
					if nodeRes.Status == plan.StatusFailed {
						failedNodes[id] = true // Mark this node as failed for propagation
						e.propagateSkip(rtCtx, g, id, dependents, result, failedNodes, &mu, "failed dependency")
					}
				} else if nodeRes.Status == plan.StatusSuccess {
					// Node succeeded, update in-degrees of its dependents
					for _, dependentID := range dependents[id] {
						inDegree[dependentID]--
						if inDegree[dependentID] == 0 {
							rtCtx.Logger.V(1).Info("Adding node to execution queue", "nodeID", dependentID, "nodeName", g.Nodes[dependentID].Name)
							queue = append(queue, dependentID)
						}
					}
				}
			}(nodeID) // End of goroutine for node execution
		} else {
			// Queue is empty, but not all nodes processed.
			// Wait for active workers or for failure propagation to complete.
			mu.Unlock()
			time.Sleep(50 * time.Millisecond) // Prevent busy spinning if something is wrong
		}
	} // End of main execution loop

	wg.Wait() // Wait for all node executions to complete

	// Finalize overall graph status
	finalStatus := plan.StatusSuccess
	for _, nodeRes := range result.NodeResults {
		if nodeRes.Status == plan.StatusFailed {
			finalStatus = plan.StatusFailed
			break
		}
		if nodeRes.Status == plan.StatusSkipped && finalStatus != plan.StatusFailed {
			// If all non-failed are skipped, then graph is skipped.
			// If some succeed and some skipped, graph is success.
			// This needs refinement: if all nodes are skipped, is graph skipped or success?
			// Typically, if there were things to do and all were skipped, it's "Skipped".
			// If some succeeded, it's "Success".
		}
	}
	// More precise final status:
	if finalStatus != plan.StatusFailed {
		allSkipped := true
		anySuccess := false
		for _, nodeRes := range result.NodeResults {
			if nodeRes.Status == plan.StatusSuccess {
				anySuccess = true
				allSkipped = false
				break
			}
			if nodeRes.Status != plan.StatusSkipped {
				allSkipped = false // If it's pending/running (should not be) or failed (already handled)
			}
		}
		if !anySuccess && allSkipped && len(g.Nodes) > 0 {
			finalStatus = plan.StatusSkipped
		} else {
			finalStatus = plan.StatusSuccess // Default to success if no failures and at least one success or empty graph
		}
		if len(g.Nodes) == 0 { // Empty graph is always success.
			finalStatus = plan.StatusSuccess
		}
	}


	result.Status = finalStatus
	result.EndTime = time.Now()
	rtCtx.Logger.Info("Graph execution finished.", "graphName", g.Name, "status", result.Status, "duration", result.EndTime.Sub(result.StartTime))
	return result, nil
}


// propagateSkip is called when a node fails or is skipped. It marks all its direct and
// indirect dependents as skipped.
// mu must be locked by the caller if modifications to shared structures (like result.NodeResults, failedNodes)
// are done without their own locks. Here, nodeRes update is protected by its own access pattern.
// failedNodes map needs protection.
func (e *dagExecutor) propagateSkip(rtCtx *runtime.Context, g *plan.ExecutionGraph, failedNodeID plan.NodeID,
	dependents map[plan.NodeID][]plan.NodeID, result *plan.GraphExecutionResult,
	failedNodesMap map[plan.NodeID]bool, sharedLock *sync.Mutex, reasonPrefix string) {

	// Mark the current node as "causally failed" for propagation logic
	// This needs to be done under sharedLock if failedNodesMap is shared and modified by multiple goroutines.
	// The caller (node execution goroutine) already locked mu for failedNodesMap update.
	// If propagateSkip is called from elsewhere, it needs to handle locking.
	// For simplicity, assume caller handles lock for marking failedNodeID in failedNodesMap.
	// This function itself will recursively call, so it should not try to re-lock sharedLock.

	for _, depID := range dependents[failedNodeID] {
		nodeRes := result.NodeResults[depID] // Assumes result.NodeResults is pre-populated

		// Only update if it's still pending. Avoid overwriting a node that already completed/failed.
		// This needs its own lock or careful state management if multiple goroutines can update NodeResult.
		// For now, assume NodeResult status updates are atomic or protected.
		// A better way: use a lock for each NodeResult or for the whole map.
		// The current structure has nodeResultsLock at the GraphExecutionResult level.
		// Let's assume we use that, or a finer-grained approach if needed.
		// For now, let's make a local copy and then update under a shared lock if needed.

		// This function can be called by multiple failing nodes concurrently.
		// We need to ensure that updates to shared `result` and `failedNodesMap` are safe.
		// The `sharedLock` (`mu` from Execute) should be used.

		sharedLock.Lock()
		if nodeRes.Status == plan.StatusPending || nodeRes.Status == plan.StatusRunning {
			nodeRes.Status = plan.StatusSkipped
			nodeRes.Message = fmt.Sprintf("%s: node '%s' (%s)", reasonPrefix, failedNodeID, g.Nodes[failedNodeID].Name)
			if nodeRes.StartTime.IsZero() { nodeRes.StartTime = time.Now() } // Mark start if not already
			nodeRes.EndTime = time.Now()
			failedNodesMap[depID] = true // Mark this dependent as also failed for further propagation

			rtCtx.Logger.Info("Propagating skip to node", "targetNodeID", depID, "targetNodeName", g.Nodes[depID].Name, "reason", nodeRes.Message)

			// Recursively propagate
			// Unlock before recursive call if the recursive call might also try to lock.
			// Here, recursive call is fine as it doesn't re-lock the same sharedLock.
			// But the issue is if multiple branches of propagation happen, they all contend for sharedLock.
			// This is okay.
			e.propagateSkip(rtCtx, g, depID, dependents, result, failedNodesMap, sharedLock, reasonPrefix)
		}
		sharedLock.Unlock()
	}
}


// runStepOnHost executes a single step on a single host.
func (e *dagExecutor) runStepOnHost(stepCtx runtime.StepContext, host connector.Host, s step.Step) *plan.HostResult {
	hr := plan.NewHostResult(host.GetName()) // Initializes status to Pending and StartTime

	logger := stepCtx.GetLogger().With("step", s.Meta().Name, "host", host.GetName()) // Use Meta()

	// Precheck
	logger.V(1).Info("Running Precheck")
	isDone, err := s.Precheck(stepCtx, host)
	if err != nil {
		hr.Status = plan.StatusFailed
		hr.Message = fmt.Sprintf("Precheck failed: %v", err)
		logger.Error(err, "Precheck failed")
		hr.EndTime = time.Now()
		return hr
	}
	if isDone {
		hr.Status = plan.StatusSkipped
		hr.Skipped = true // Explicitly mark as skipped by precheck
		hr.Message = "Skipped: Precheck condition already met."
		logger.Info("Precheck condition met, skipping run.")
		hr.EndTime = time.Now()
		return hr
	}

	// Run
	hr.Status = plan.StatusRunning // Mark as running before actual execution
	logger.Info("Running step")
	runErr := s.Run(stepCtx, host)
	hr.EndTime = time.Now() // Set EndTime as soon as Run finishes or errors

	if runErr != nil {
		hr.Status = plan.StatusFailed
		hr.Message = fmt.Sprintf("Run failed: %v", runErr)
		if cmdErr, ok := runErr.(*connector.CommandError); ok {
			hr.Stdout = cmdErr.Stdout
			hr.Stderr = cmdErr.Stderr
		}
		logger.Error(runErr, "Step run failed")

		// Attempt Rollback
		logger.Info("Attempting rollback")
		if rbErr := s.Rollback(stepCtx, host); rbErr != nil {
			rbMsg := fmt.Sprintf("Rollback failed after run error: %v", rbErr)
			hr.Message = fmt.Sprintf("%s. %s", hr.Message, rbMsg)
			logger.Error(rbErr, "Rollback failed")
		} else {
			logger.Info("Rollback successful.")
		}
		return hr
	}

	hr.Status = plan.StatusSuccess
	hr.Message = "Step executed successfully."
	logger.Info("Step executed successfully.")
	return hr
}

// dryRun prints the execution graph details.
func (e *dagExecutor) dryRun(rtCtx *runtime.Context, g *plan.ExecutionGraph, result *plan.GraphExecutionResult) {
	rtCtx.Logger.Info("--- Dry Run Execution Graph ---", "graphName", g.Name)
	fmt.Printf("--- Dry Run Execution Graph: %s ---\n", g.Name)

	// For dry run, simply iterate nodes. Order doesn't strictly matter for display,
	// but sorting by ID or processing in a somewhat topological order could be nicer.
	// For now, iterate map order.
	for id, node := range g.Nodes {
		nodeRes := plan.NewNodeResult(node.Name, node.StepName)
		nodeRes.Status = plan.StatusSkipped
		nodeRes.Message = "Dry run: Node execution skipped."
		nodeRes.StartTime = time.Now() // Or don't set times for dry run
		nodeRes.EndTime = time.Now()

		hostNames := make([]string, len(node.Hosts))
		for i, h := range node.Hosts {
			hostNames[i] = h.GetName()
			hr := plan.NewHostResult(h.GetName())
			hr.Status = plan.StatusSkipped
			hr.Message = "Dry run: Host operation skipped."
			hr.Skipped = true
			hr.StartTime = nodeRes.StartTime
			hr.EndTime = nodeRes.EndTime
			nodeRes.HostResults[h.GetName()] = hr
		}
		result.NodeResults[id] = nodeRes

		rtCtx.Logger.Info("Node (Dry Run)",
			"id", id,
			"name", node.Name,
			"step", node.StepName,
			"hosts", hostNames,
			"dependencies", node.Dependencies,
		)
		fmt.Printf("  Node: %s (ID: %s)\n", node.Name, id)
		fmt.Printf("    Step: %s\n", node.StepName)
		fmt.Printf("    Hosts: %v\n", hostNames)
		fmt.Printf("    Dependencies: %v\n", node.Dependencies)
		fmt.Printf("    Status: %s (Dry Run)\n", plan.StatusSkipped)
	}
	rtCtx.Logger.Info("--- End of Dry Run ---")
	fmt.Println("--- End of Dry Run ---")
}

// Ensure dagExecutor implements Engine.
var _ Engine = &dagExecutor{}
