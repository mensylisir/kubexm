package kubevip

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateKubeVipManifestStep struct {
	step.Base
}

func NewGenerateKubeVipManifestStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateKubeVipManifestStep] {
	s := &GenerateKubeVipManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate kube-vip static pod manifest"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type KubeVipTemplateData struct {
	Image         string
	VIP           string
	Interface     string
}

func (s *GenerateKubeVipManifestStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	// In a real scenario, this data would come from the cluster spec
	data := KubeVipTemplateData{
		Image:     "ghcr.io/kube-vip/kube-vip:v0.5.7",
		VIP:       "192.168.1.200",
		Interface: "eth0",
	}

	dummyTemplate := `
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  name: kube-vip
  namespace: kube-system
spec:
  containers:
  - args:
    - manager
    env:
    - name: vip_arp
      value: "true"
    - name: port
      value: "6443"
    - name: vip_interface
      value: "{{ .Interface }}"
    - name: vip_cidr
      value: "32"
    - name: dns_mode
      value: first
    - name: vip_ddns
      value: "false"
    - name: cp_enable
      value: "true"
    - name: cp_namespace
      value: kube-system
    - name: svc_enable
      value: "false"
    - name: vip_leaderelection
      value: "true"
    - name: vip_leasename
      value: kube-vip
    - name: vip_leaseduration
      value: "5"
    - name: vip_renewdeadline
      value: "3"
    - name: vip_retryperiod
      value: "1"
    - name: address
      value: {{ .VIP }}
    image: {{ .Image }}
    imagePullPolicy: IfNotPresent
    name: kube-vip
    resources: {}
    securityContext:
      capabilities:
        add:
        - NET_ADMIN
        - NET_RAW
    volumeMounts:
    - mountPath: /etc/kubernetes/admin.conf
      name: kubeconfig
  hostAliases:
  - hostnames:
    - kubernetes
    ip: 127.0.0.1
  hostNetwork: true
  volumes:
  - hostPath:
      path: /etc/kubernetes/admin.conf
    name: kubeconfig
status: {}
`
	renderedManifest, err := templates.Render(dummyTemplate, data)
	if err != nil {
		return fmt.Errorf("failed to render kube-vip manifest: %w", err)
	}

	ctx.Set("kube-vip.yaml", renderedManifest)
	logger.Info("kube-vip.yaml generated successfully.")

	return nil
}

func (s *GenerateKubeVipManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("kube-vip.yaml")
	return nil
}

func (s *GenerateKubeVipManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
