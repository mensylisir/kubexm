package module

import (
	"reflect" // For reflect.DeepEqual in helper, if used.
	"sort"    // For sorting string slices in helper, if used.
	"strings" // For string checks in names/types
	"testing"
	// "context" // No longer needed for these factory tests
	// "errors" // No longer needed
	// "fmt" // No longer needed
	// "sync/atomic" // No longer needed
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/config"    // For dummy config
	"github.com/mensylisir/kubexm/pkg/spec"     // For spec.ModuleSpec, spec.TaskSpec
	// "github.com/kubexms/kubexms/pkg/runtime" // Not directly needed for factory assembly tests
	// "github.com/kubexms/kubexms/pkg/step"      // For step.GetSpecTypeName if checking hook step types

	// Import module factories
	// moduleContainerd "github.com/mensylisir/kubexm/pkg/module/containerd" // Moved
	// modulePreflight "github.com/mensylisir/kubexm/pkg/module/preflight" // Moved
	// moduleEtcd "github.com/mensylisir/kubexm/pkg/module/etcd" // Moved
)

// Note: MockTask and newTestRuntimeForModule helpers are removed as they were for testing Module.Run,
// which no longer exists in pkg/module/module.go. Tests now focus on factory output (*spec.ModuleSpec).

// TestNewPreflightModule_Factory_IsEnabledLogic has been moved to pkg/module/preflight/preflight_module_test.go

// TestNewContainerdModule_Factory_IsEnabledLogic has been moved to pkg/module/containerd/containerd_module_test.go

// TestNewEtcdModule_Factory_IsEnabledLogic has been moved to pkg/module/etcd/etcd_module_test.go

// reflectDeepEqual helper can be removed if not used.
// func reflectDeepEqual(a, b interface{}) bool { return reflect.DeepEqual(a, b) }
