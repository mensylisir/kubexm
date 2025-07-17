package engine

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/step"
)

type dagExecutor struct {
	maxWorkers int
}

type workerResult struct {
	nodeID      plan.NodeID
	status      plan.Status
	message     string
	err         error
	hostResults map[string]*plan.HostResult
}

func NewExecutor() Engine {
	return &dagExecutor{
		maxWorkers: 10,
	}
}

func (e *dagExecutor) Execute(rootCtx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	result := plan.NewGraphExecutionResult(g.Name)
	log := rootCtx.GetLogger()

	if dryRun {
		e.dryRun(log, g, result)
		return result, nil
	}

	log.Info("Starting execution of graph.", "graphName", g.Name, "totalNodes", len(g.Nodes))
	if err := g.Validate(); err != nil {
		result.Finalize(plan.StatusFailed, fmt.Sprintf("Graph validation failed: %v", err))
		log.Error(err, "Execution graph validation failed")
		return result, err
	}

	inDegree := make(map[plan.NodeID]int)
	dependents := make(map[plan.NodeID][]plan.NodeID)
	for id, node := range g.Nodes {
		inDegree[id] = len(node.Dependencies)
		for _, depID := range node.Dependencies {
			dependents[depID] = append(dependents[depID], id)
		}
		result.NodeResults[id] = plan.NewNodeResult(node.Name, node.StepName)
	}

	tasks := make(chan plan.NodeID, len(g.Nodes))
	results := make(chan workerResult, len(g.Nodes))
	var wg sync.WaitGroup

	log.Debug("Starting worker pool.", "workerCount", e.maxWorkers)
	for i := 0; i < e.maxWorkers; i++ {
		wg.Add(1)
		go e.worker(rootCtx, g, &wg, tasks, results)
	}

	initialQueueSize := 0
	for id, degree := range inDegree {
		if degree == 0 {
			tasks <- id
			initialQueueSize++
		}
	}
	log.Debug("Initial tasks dispatched.", "count", initialQueueSize)

	processedNodesCount := 0
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

	log.Info("Graph execution finished.", "graphName", g.Name, "status", result.Status, "duration", result.EndTime.Sub(result.StartTime))
	return result, nil
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
	log.Info("Executing node on hosts...", "hosts", node.Hostnames)

	hostGroup, gctx := errgroup.WithContext(rootCtx.GoContext())
	hostResults := make(map[string]*plan.HostResult)
	var mu sync.Mutex

	for _, host := range node.Hosts {
		currentHost := host
		hostGroup.Go(func() error {
			execCtx := runtime.ForHost(rootCtx, currentHost).WithGoContext(gctx)
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

	return workerResult{
		nodeID:      nodeID,
		status:      tempNodeResult.Status,
		message:     finalMessage,
		err:         err,
		hostResults: hostResults,
	}
}
func (e *dagExecutor) runStepOnHost(ctx runtime.ExecutionContext, s step.Step) *plan.HostResult {
	host := ctx.GetHost()
	hr := plan.NewHostResult(host.GetName())
	log := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName())

	log.Debug("Running Precheck...")
	isDone, err := s.Precheck(ctx)
	if err != nil {
		hr.Status = plan.StatusFailed
		hr.Message = fmt.Sprintf("Precheck failed: %v", err)
		hr.EndTime = time.Now()
		log.Error(err, "Step precheck failed.")
		return hr
	}
	if isDone {
		hr.Status = plan.StatusSkipped
		hr.Message = "Skipped: Precheck condition already met."
		hr.EndTime = time.Now()
		log.Info("Step skipped by precheck.")
		return hr
	}

	hr.Status = plan.StatusRunning
	log.Info("Running step...")
	runErr := s.Run(ctx)
	hr.EndTime = time.Now()

	if runErr != nil {
		hr.Status = plan.StatusFailed
		hr.Message = fmt.Sprintf("Run failed: %v", runErr)
		log.Error(runErr, "Step run failed.")
		log.Info("Attempting rollback...")
		if rbErr := s.Rollback(ctx); rbErr != nil {
			hr.Message = fmt.Sprintf("%s. Rollback failed: %v", hr.Message, rbErr)
			log.Error(rbErr, "Rollback failed.")
		} else {
			log.Info("Rollback successful.")
		}
		return hr
	}

	hr.Status = plan.StatusSuccess
	hr.Message = "Step executed successfully."
	log.Info("Step executed successfully.")
	return hr
}

func (e *dagExecutor) dryRun(logger *logger.Logger, g *plan.ExecutionGraph, result *plan.GraphExecutionResult) {
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
	fmt.Println("--- End of Dry Run ---")
}

var _ Engine = &dagExecutor{}
