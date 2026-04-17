package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	stepcommon "github.com/mensylisir/kubexm/internal/step/common"
	"github.com/mensylisir/kubexm/internal/task"
)

// CheckConnectivityTask checks SSH connectivity to all configured hosts.
type CheckConnectivityTask struct {
	task.Base
}

func NewCheckConnectivityTask() task.Task {
	return &CheckConnectivityTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckConnectivity",
				Description: "Check SSH connectivity to all configured hosts",
			},
		},
	}
}

func (t *CheckConnectivityTask) Name() string {
	return t.Meta.Name
}

func (t *CheckConnectivityTask) Description() string {
	return t.Meta.Description
}

func (t *CheckConnectivityTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Connectivity check is always required
	return true, nil
}

func (t *CheckConnectivityTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	// Get all hosts from all roles
	allHosts := t.getAllHosts(ctx)
	if len(allHosts) == 0 {
		return plan.NewEmptyFragment(t.Name()), nil
	}

	var entryNodes []plan.NodeID

	for _, host := range allHosts {
		checkStep, err := stepcommon.NewCheckConnectivityStepBuilder(host).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create check connectivity step for %s: %w", host.GetAddress(), err)
		}

		hostList := []remotefw.Host{host}
		nodeName := fmt.Sprintf("CheckConnectivity-%s", host.GetAddress())
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name: nodeName,
			Step: checkStep,
			Hosts: hostList,
		})
		entryNodes = append(entryNodes, nodeID)
	}

	// All connectivity checks can run in parallel
	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

func (t *CheckConnectivityTask) getAllHosts(ctx runtime.TaskContext) []remotefw.Host {
	var allHosts []remotefw.Host

	// Get masters
	masters := ctx.GetHostsByRole(common.RoleMaster)
	allHosts = append(allHosts, masters...)

	// Get workers
	workers := ctx.GetHostsByRole(common.RoleWorker)
	allHosts = append(allHosts, workers...)

	// Get etcd nodes (if different from masters)
	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
	for _, eh := range etcdHosts {
		found := false
		for _, ah := range allHosts {
			if eh.GetAddress() == ah.GetAddress() {
				found = true
				break
			}
		}
		if !found {
			allHosts = append(allHosts, eh)
		}
	}

	// Get load balancer nodes (if any)
	lbHosts := ctx.GetHostsByRole(common.RoleLoadBalancer)
	allHosts = append(allHosts, lbHosts...)

	return allHosts
}

// Ensure task.Task interface is implemented.
var _ task.Task = (*CheckConnectivityTask)(nil)