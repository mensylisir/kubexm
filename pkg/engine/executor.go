package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger" // For logger.Logger type
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	// runtime.StepContext is no longer used directly, replaced by engine.StepContext from interface.go
	"github.com/mensylisir/kubexm/pkg/step"
)

// dagExecutor implements the Engine interface for DAG-based execution.
type dagExecutor struct {
	maxWorkers int
}

// NewExecutor creates a new DAG-based execution engine.
func NewExecutor() Engine {
	return &dagExecutor{
		maxWorkers: 10, // Default
	}
}

// Execute processes the execution graph.
func (e *dagExecutor) Execute(ctx EngineExecuteContext, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	result := plan.NewGraphExecutionResult(g.Name)
	logger := ctx.GetLogger()

	if dryRun {
		e.dryRun(logger, g, result) // Pass logger to dryRun
		result.EndTime = time.Now()
		overallStatus := plan.StatusSuccess
		// ... (rest of dry run status logic remains the same)
		for _, nr := range result.NodeResults {
			if nr.Status == plan.StatusFailed {
				overallStatus = plan.StatusFailed
				break
			}
		}
		result.Status = overallStatus
		logger.Info("Dry run complete.", "graphName", g.Name, "status", result.Status)
		return result, nil
	}

	logger.Info("Starting execution of graph.", "graphName", g.Name, "totalNodes", len(g.Nodes))
	result.Status = plan.StatusRunning

	if err := g.Validate(); err != nil {
		result.Status = plan.StatusFailed
		result.EndTime = time.Now()
		logger.Error(err, "Execution graph validation failed")
		return result, fmt.Errorf("graph validation failed: %w", err)
	}
	logger.Debugf("Execution graph validated successfully.") // Changed V(1).Info to Debugf

	inDegree := make(map[plan.NodeID]int)
	dependents := make(map[plan.NodeID][]plan.NodeID)
	queue := make([]plan.NodeID, 0)
	//nodeResultsLock := new(sync.Mutex) // This lock will be used

	for id, node := range g.Nodes {
		inDegree[id] = len(node.Dependencies)
		if inDegree[id] == 0 {
			queue = append(queue, id)
		}
		for _, depID := range node.Dependencies {
			dependents[depID] = append(dependents[depID], id)
		}
		result.NodeResults[id] = plan.NewNodeResult(node.Name, node.StepName)
	}

	if len(queue) == 0 && len(g.Nodes) > 0 {
		msg := "no entry nodes found in a non-empty graph, possibly a cycle or misconfiguration"
		logger.Error(nil, msg)
		result.Status = plan.StatusFailed
		result.EndTime = time.Now()
		return result, fmt.Errorf(msg)
	}
	logger.Debugf("Initial execution queue populated. queueSize: %d, initialQueue: %v", len(queue), queue) // Changed V(1).Info to Debugf

	var wg sync.WaitGroup
	sem := make(chan struct{}, e.maxWorkers)
	processedNodesCount := 0
	mu := sync.Mutex{}
	failedNodes := make(map[plan.NodeID]bool)

	for {
		mu.Lock()
		if len(queue) == 0 && processedNodesCount == len(g.Nodes) {
			mu.Unlock()
			break
		}

		if len(queue) == 0 && processedNodesCount < len(g.Nodes) {
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
				break
			}
			logger.Info("Execution queue is empty, but not all nodes are processed and some are pending/running.", "processedCount", processedNodesCount, "totalNodes", len(g.Nodes))
			mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if len(queue) > 0 {
			nodeID := queue[0]
			queue = queue[1:]
			mu.Unlock()

			nodeToExecute := g.Nodes[nodeID]
			skipNode := false
			var skipReason string
			for _, depID := range nodeToExecute.Dependencies {
				mu.Lock()
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
				nodeRes.StartTime = time.Now()
				nodeRes.EndTime = time.Now()
				logger.Info("Skipping node because its dependency failed", "nodeID", nodeID, "nodeName", nodeToExecute.Name, "reason", skipReason)
				processedNodesCount++
				failedNodes[nodeID] = true
				mu.Unlock()
				continue
			}

			wg.Add(1)
			sem <- struct{}{}

			go func(id plan.NodeID) {
				defer wg.Done()
				defer func() { <-sem }()

				node := g.Nodes[id]
				nodeRes := result.NodeResults[id]
				logHostNames := make([]string, len(node.Hosts))
				for i, h := range node.Hosts {
					logHostNames[i] = h.GetName()
				}
				logger.Info("Executing node", "nodeID", id, "nodeName", node.Name, "step", node.StepName, "hosts", logHostNames)
				mu.Lock()
				nodeRes.Status = plan.StatusRunning
				nodeRes.StartTime = time.Now()
				mu.Unlock()

				nodeGoCtx, nodeCancel := context.WithCancel(ctx.GoContext()) // Use GoContext from EngineExecuteContext
				defer nodeCancel()
				hostGroup, hostCtxGroup := errgroup.WithContext(nodeGoCtx)

				for _, host := range node.Hosts {
					currentHost := host
					hostGroup.Go(func() error {
						stepCtxForHost := ctx.ForHost(currentHost) 
						finalStepCtxForHost := stepCtxForHost.WithGoContext(hostCtxGroup.Context())
						hostRes := e.runStepOnHost(finalStepCtxForHost, node.Step)
						mu.Lock() // Corrected typo: nodeResLock -> nodeResultsLock
						nodeRes.HostResults[currentHost.GetName()] = hostRes
						mu.Unlock() // Corrected typo: nodeResLock -> nodeResultsLock
						if hostRes.Status == plan.StatusFailed {
							return fmt.Errorf("step '%s' on host '%s' failed: %s", node.StepName, currentHost.GetName(), hostRes.Message)
						}
						return nil
					})
				}

				nodeFailed := false
				var determinedStatus plan.Status
				var determinedMessage string

				if err := hostGroup.Wait(); err != nil {
					determinedMessage = err.Error()
					nodeFailed = true
					logger.Error(err, "Node execution failed", "nodeID", id, "nodeName", node.Name)
				}

				// ... (node status determination logic based on hostResults - remains the same)
				if nodeFailed {
					determinedStatus = plan.StatusFailed
					if determinedMessage == "" {
						determinedMessage = "Node failed due to one or more host failures."
					}
				} else {
					allHostsSkippedByPrecheck := true
					anyHostSucceeded := false
					for _, hr := range nodeRes.HostResults {
						if hr.Status == plan.StatusFailed {
							nodeFailed = true
							break
						}
						if hr.Status == plan.StatusSuccess {
							anyHostSucceeded = true
							allHostsSkippedByPrecheck = false
						}
						if hr.Status != plan.StatusSkipped {
							allHostsSkippedByPrecheck = false
						}
					}
					if nodeFailed {
						determinedStatus = plan.StatusFailed
						determinedMessage = "Node failed due to one or more host failures."
					} else if allHostsSkippedByPrecheck && len(nodeRes.HostResults) > 0 {
						determinedStatus = plan.StatusSkipped
						determinedMessage = "All hosts skipped by precheck."
					} else if anyHostSucceeded || len(nodeRes.HostResults) == 0 {
						determinedStatus = plan.StatusSuccess
						determinedMessage = "Node executed successfully."
					} else {
						determinedStatus = plan.StatusSuccess
						determinedMessage = "Node completed; some hosts may have been skipped by precheck."
					}
				}
				endTime := time.Now()
				mu.Lock()
				nodeRes.Status = determinedStatus
				nodeRes.Message = determinedMessage
				nodeRes.EndTime = endTime
				mu.Unlock()

				mu.Lock()
				processedNodesCount++
				logger.Info("Node finished", "nodeID", id, "nodeName", node.Name, "status", nodeRes.Status, "duration", nodeRes.EndTime.Sub(nodeRes.StartTime))
				if nodeRes.Status == plan.StatusFailed {
					failedNodes[id] = true
					e.markDependentsSkipped(logger, g, id, dependents, result.NodeResults, failedNodes, &processedNodesCount)
				} else if nodeRes.Status == plan.StatusSuccess {
					for _, dependentID := range dependents[id] {
						if !failedNodes[dependentID] && result.NodeResults[dependentID].Status == plan.StatusPending {
							inDegree[dependentID]--
							if inDegree[dependentID] == 0 {
								logger.Debugf("Adding node to execution queue. nodeID: %s, nodeName: %s", dependentID, g.Nodes[dependentID].Name) // Changed V(1).Info to Debugf
								queue = append(queue, dependentID)
							}
						}
					}
				}
				mu.Unlock()
			}(nodeID)
		} else {
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
		}
	}

	wg.Wait()
	// ... (final graph status determination logic - remains the same)
	finalStatus := plan.StatusSuccess
	for _, nodeRes := range result.NodeResults {
		if nodeRes.Status == plan.StatusFailed {
			finalStatus = plan.StatusFailed
			break
		}
	}
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
				allSkipped = false
			}
		}
		if !anySuccess && allSkipped && len(g.Nodes) > 0 {
			finalStatus = plan.StatusSkipped
		} else {
			finalStatus = plan.StatusSuccess
		}
		if len(g.Nodes) == 0 {
			finalStatus = plan.StatusSuccess
		}
	}
	result.Status = finalStatus
	result.EndTime = time.Now()
	logger.Info("Graph execution finished.", "graphName", g.Name, "status", result.Status, "duration", result.EndTime.Sub(result.StartTime))
	return result, nil
}

// runStepOnHost executes a single step on a single host.
// It now takes step.StepContext (defined in step/interface.go).
func (e *dagExecutor) runStepOnHost(stepCtx step.StepContext, s step.Step) *plan.HostResult {
	currentHost := stepCtx.GetHost()
	hr := plan.NewHostResult(currentHost.GetName())
	engineLogger := stepCtx.GetLogger().With("engine_step_runner", s.Meta().Name, "host", currentHost.GetName())

	engineLogger.Debugf("Running Precheck")         // Changed V(1).Info to Debugf
	isDone, err := s.Precheck(stepCtx, currentHost) // Pass currentHost, stepCtx is already host-scoped conceptually
	if err != nil {
		hr.Status = plan.StatusFailed
		hr.Message = fmt.Sprintf("Precheck failed: %v", err)
		engineLogger.Error(err, "Precheck failed")
		hr.EndTime = time.Now()
		return hr
	}
	if isDone {
		hr.Status = plan.StatusSkipped
		hr.Skipped = true
		hr.Message = "Skipped: Precheck condition already met."
		engineLogger.Info("Precheck condition met, skipping run.")
		hr.EndTime = time.Now()
		return hr
	}

	hr.Status = plan.StatusRunning
	engineLogger.Info("Running step")
	runErr := s.Run(stepCtx, currentHost) // Pass currentHost
	hr.EndTime = time.Now()

	if runErr != nil {
		hr.Status = plan.StatusFailed
		hr.Message = fmt.Sprintf("Run failed: %v", runErr)
		if cmdErr, ok := runErr.(*connector.CommandError); ok {
			hr.Stdout = cmdErr.Stdout
			hr.Stderr = cmdErr.Stderr
		}
		engineLogger.Error(runErr, "Step run failed")
		engineLogger.Info("Attempting rollback")
		if rbErr := s.Rollback(stepCtx, currentHost); rbErr != nil { // Pass currentHost
			rbMsg := fmt.Sprintf("Rollback failed after run error: %v", rbErr)
			hr.Message = fmt.Sprintf("%s. %s", hr.Message, rbMsg)
			engineLogger.Error(rbErr, "Rollback failed")
		} else {
			engineLogger.Info("Rollback successful.")
		}
		return hr
	}

	hr.Status = plan.StatusSuccess
	hr.Message = "Step executed successfully."
	engineLogger.Info("Step executed successfully.")
	return hr
}

// dryRun prints the execution graph details.
func (e *dagExecutor) dryRun(logger *logger.Logger, g *plan.ExecutionGraph, result *plan.GraphExecutionResult) { // logger passed in
	logger.Info("--- Dry Run Execution Graph ---", "graphName", g.Name)
	fmt.Printf("--- Dry Run Execution Graph: %s ---\n", g.Name)

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
			hr.Skipped = true
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
		fmt.Printf("    Step: %s\n", node.StepName)
		fmt.Printf("    Hosts: %v\n", hostNames)
		fmt.Printf("    Dependencies: %v\n", node.Dependencies)
		fmt.Printf("    Status: %s (Dry Run)\n", plan.StatusSkipped)
	}
	logger.Info("--- End of Dry Run ---")
	fmt.Println("--- End of Dry Run ---")
}

var _ Engine = &dagExecutor{}

func (e *dagExecutor) markDependentsSkipped(logger *logger.Logger, g *plan.ExecutionGraph, // logger passed in
	sourceNodeID plan.NodeID,
	dependentsGraph map[plan.NodeID][]plan.NodeID,
	nodeResults map[plan.NodeID]*plan.NodeResult,
	failedOrSkippedNodes map[plan.NodeID]bool,
	processedNodesCount *int) {

	for _, depID := range dependentsGraph[sourceNodeID] {

		if failedOrSkippedNodes[depID] {
			continue
		}
		nodeRes := nodeResults[depID]
		if nodeRes.Status == plan.StatusPending || nodeRes.Status == plan.StatusRunning {
			nodeRes.Status = plan.StatusSkipped
			nodeRes.Message = fmt.Sprintf("Skipped due to failed prerequisite '%s' (%s)", sourceNodeID, g.Nodes[sourceNodeID].Name)
			if nodeRes.StartTime.IsZero() {
				nodeRes.StartTime = time.Now()
			}
			nodeRes.EndTime = time.Now()
			logger.Info("Marking node as skipped (cascade)", "targetNodeID", depID, "targetNodeName", g.Nodes[depID].Name, "reason", nodeRes.Message)
			failedOrSkippedNodes[depID] = true
			(*processedNodesCount)++
			e.markDependentsSkipped(logger, g, depID, dependentsGraph, nodeResults, failedOrSkippedNodes, processedNodesCount) // Pass logger
		}
	}
}
