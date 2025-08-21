package runtime

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"k8s.io/client-go/tools/record"
	"time"
)

type CoreServiceContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetRunner() runner.Runner
	GetRecorder() record.EventRecorder
}

type ClusterQueryContext interface {
	GetClusterConfig() *v1alpha1.Cluster
	GetHostsByRole(role string) []connector.Host
	GetHostFacts(host connector.Host) (*runner.Facts, error)
	GetControlNode() (connector.Host, error)
}

type FileSystemContext interface {
	GetGlobalWorkDir() string
	GetWorkspace() string
	GetClusterWorkDir() string
	GetHostWorkDir() string
	GetExtractDir() string
	GetUploadDir() string
	GetKubernetesCertsDir() string
	GetEtcdCertsDir() string
	GetHarborCertsDir() string
	GetRegistryCertsDir() string
	GetRepositoryDir() string
	GetComponentArtifactsDir(componentTypeDir string) string
	GetFileDownloadPath(componentName, version, arch, fileName string) string
	GetHostDir(hostname string) string
}

type CacheProviderContext interface {
	GetPipelineCache() cache.PipelineCache
	GetModuleCache() cache.ModuleCache
	GetTaskCache() cache.TaskCache
	GetStepCache() cache.StepCache
	GetPipelineName() string
	GetModuleName() string
	GetTaskName() string
	GetRunID() string
}

type GlobalSettingsContext interface {
	IsVerbose() bool
	ShouldIgnoreErr() bool
	GetGlobalConnectionTimeout() time.Duration
	IsOfflineMode() bool
}
