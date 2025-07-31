package calico

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

// GenerateCalicoValuesStep 负责根据集群配置生成 Calico 的 Helm values 文件。
// 这个步骤在控制端运行，不与任何远程主机交互。
type GenerateCalicoValuesStep struct {
	step.Base
	// 所有需要传递给模板的字段
	Registry             string
	IPPools              []v1alpha1.CalicoIPPool
	VethMTU              int
	LogSeverityScreen    string
	TyphaEnabled         bool
	TyphaReplicas        int
	TyphaNodeSelector    map[string]string
	OperatorImage        string
	OperatorNodeSelector map[string]string
	OperatorTolerations  []map[string]string
}

// GenerateCalicoValuesStepBuilder 用于构建实例。
type GenerateCalicoValuesStepBuilder struct {
	step.Builder[GenerateCalicoValuesStepBuilder, *GenerateCalicoValuesStep]
}

func NewGenerateCalicoValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateCalicoValuesStepBuilder {
	s := &GenerateCalicoValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Calico Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	clusterCfg := ctx.GetClusterConfig()
	imageProvider := images.NewImageProvider(&ctx)

	// 1. 设置默认值
	s.OperatorNodeSelector = map[string]string{"kubernetes.io/os": "linux"}
	s.OperatorTolerations = []map[string]string{
		{"key": "node-role.kubernetes.io/control-plane", "operator": "Exists", "effect": "NoSchedule"},
	}
	s.VethMTU = 1440
	s.LogSeverityScreen = "Info"
	s.TyphaNodeSelector = map[string]string{"node-role.kubernetes.io/control-plane": ""}

	// 2. 从关键配置中获取值
	if clusterCfg.Spec.Registry != nil && clusterCfg.Spec.Registry.MirroringAndRewriting != nil {
		s.Registry = clusterCfg.Spec.Registry.MirroringAndRewriting.PrivateRegistry
	}

	operatorImg := imageProvider.GetImage("tigera-operator")
	if operatorImg == nil {
		if clusterCfg.Spec.Network.Plugin == string(common.CNITypeCalico) {
			fmt.Fprintf(os.Stderr, "Error: Calico is enabled but 'tigera-operator' image is not found in BOM for K8s version %s\n", clusterCfg.Spec.Kubernetes.Version)
		}
		return nil
	}
	s.OperatorImage = operatorImg.FullName()

	// 3. 应用默认值和用户覆盖逻辑
	defaultNat := true
	defaultBlockSize := 26
	defaultPool := v1alpha1.CalicoIPPool{
		CIDR:          clusterCfg.Spec.Network.KubePodsCIDR,
		Encapsulation: "VXLAN",
		NatOutgoing:   &defaultNat,
		BlockSize:     &defaultBlockSize,
	}

	userCalicoCfg := clusterCfg.Spec.Network.Calico
	if userCalicoCfg != nil {
		// 用户配置覆盖网络设置
		if userCalicoCfg.Networking != nil {
			if userCalicoCfg.Networking.VethMTU != nil {
				s.VethMTU = *userCalicoCfg.Networking.VethMTU
			}
			// 封装模式的默认值，可被用户覆盖
			if userCalicoCfg.Networking.IPIPMode != "" {
				defaultPool.Encapsulation = "IPIP"
			}
			if userCalicoCfg.Networking.VXLANMode != "" {
				defaultPool.Encapsulation = "VXLAN"
			}
		}

		// 用户配置覆盖 IP 池
		if userCalicoCfg.IPAM != nil && len(userCalicoCfg.IPAM.Pools) > 0 {
			s.IPPools = userCalicoCfg.IPAM.Pools
			// 为用户定义的池填充默认值
			for i := range s.IPPools {
				if s.IPPools[i].Encapsulation == "" {
					s.IPPools[i].Encapsulation = defaultPool.Encapsulation
				}
				if s.IPPools[i].NatOutgoing == nil {
					s.IPPools[i].NatOutgoing = &defaultNat
				}
				if s.IPPools[i].BlockSize == nil {
					s.IPPools[i].BlockSize = &defaultBlockSize
				}
			}
		} else {
			s.IPPools = []v1alpha1.CalicoIPPool{defaultPool}
		}

		// 用户配置覆盖 Felix 日志级别
		if userCalicoCfg.FelixConfiguration != nil && userCalicoCfg.FelixConfiguration.LogSeverityScreen != "" {
			s.LogSeverityScreen = userCalicoCfg.FelixConfiguration.LogSeverityScreen
		}

		// 用户配置覆盖 Typha 设置
		if userCalicoCfg.TyphaDeployment != nil {
			if userCalicoCfg.TyphaDeployment.Replicas != nil {
				s.TyphaReplicas = *userCalicoCfg.TyphaDeployment.Replicas
			}
			if userCalicoCfg.TyphaDeployment.NodeSelector != nil {
				s.TyphaNodeSelector = userCalicoCfg.TyphaDeployment.NodeSelector
			}
			if userCalicoCfg.TyphaDeployment.Enabled != nil {
				s.TyphaEnabled = *userCalicoCfg.TyphaDeployment.Enabled
			}
		}
	} else {
		// 用户完全没有提供 calico 配置块，使用完全的默认值
		s.IPPools = []v1alpha1.CalicoIPPool{defaultPool}
	}

	// 基于上下文的智能默认值，仅当用户未明确指定时生效
	if userCalicoCfg == nil || userCalicoCfg.TyphaDeployment == nil || userCalicoCfg.TyphaDeployment.Enabled == nil {
		if len(ctx.GetHostsByRole(common.RoleKubernetes)) > 50 {
			s.TyphaEnabled = true
		}
	}

	b := new(GenerateCalicoValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateCalicoValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCalicoValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeCalico) {
		return true, nil
	}
	return false, nil
}

// getLocalValuesPath 定义了 values.yaml 在集群 artifacts 目录中的约定存储路径。
// 它与 chart .tgz 文件在同一个目录下。
func (s *GenerateCalicoValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart(string(common.CNITypeCalico))
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for calico in BOM")
	}
	// 获取 chart .tgz 文件的完整路径
	chartTgzPath := chart.LocalPath(ctx.GetClusterArtifactsDir())
	// 获取该文件所在的目录
	chartDir := filepath.Dir(chartTgzPath)
	// 将 values.yaml 放在这个目录下
	return filepath.Join(chartDir, "calico-values.yaml"), nil
}

func (s *GenerateCalicoValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("cni/calico/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded calico values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("calicoValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse calico values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render calico values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating Calico Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated Calico Helm values file.")
	return nil
}

func (s *GenerateCalicoValuesStep) Rollback(ctx runtime.ExecutionContext) error {
	if localPath, err := s.getLocalValuesPath(ctx); err == nil {
		os.Remove(localPath)
	}
	return nil
}

var _ step.Step = (*GenerateCalicoValuesStep)(nil)
