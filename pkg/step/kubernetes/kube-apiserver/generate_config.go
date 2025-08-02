package apiserver

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateApiServerConfigStep struct {
	step.Base
}

func NewGenerateApiServerConfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateApiServerConfigStep] {
	s := &GenerateApiServerConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate kube-apiserver config file"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type ApiServerTemplateData struct {
	EtcdServers           []string
	EtcdCaFile            string
	EtcdCertFile          string
	EtcdKeyFile           string
	KubeletClientCaFile   string
	KubeletClientCertFile string
	KubeletClientKeyFile  string
	ServiceAccountKeyFile string
	ServiceAccountIssuer  string
	TlsCertFile           string
	TlsPrivateKeyFile     string
}

func (s *GenerateApiServerConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	pkiPath := common.DefaultKubernetesPKIDir

	// This is a placeholder. A real implementation would get this from the cluster spec
	// and the runtime context.
	data := ApiServerTemplateData{
		EtcdServers:           []string{"https://127.0.0.1:2379"},
		EtcdCaFile:            filepath.Join(common.DefaultEtcdPKIDir, "ca.pem"),
		EtcdCertFile:          filepath.Join(pkiPath, "etcd-client.pem"),
		EtcdKeyFile:           filepath.Join(pkiPath, "etcd-client-key.pem"),
		KubeletClientCaFile:   filepath.Join(pkiPath, "ca.pem"),
		KubeletClientCertFile: filepath.Join(pkiPath, "apiserver-kubelet-client.pem"),
		KubeletClientKeyFile:  filepath.Join(pkiPath, "apiserver-kubelet-client-key.pem"),
		ServiceAccountKeyFile: filepath.Join(pkiPath, "sa.pub"),
		ServiceAccountIssuer:  "https://kubernetes.default.svc.cluster.local",
		TlsCertFile:           filepath.Join(pkiPath, "apiserver.pem"),
		TlsPrivateKeyFile:     filepath.Join(pkiPath, "apiserver-key.pem"),
	}

	templateContent, err := templates.Get("kubernetes/kube-apiserver/config.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get apiserver config template: %w", err)
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render apiserver config template: %w", err)
	}

	ctx.Set("kube-apiserver.config.yaml", renderedConfig)
	logger.Info("kube-apiserver.config.yaml generated successfully.")

	return nil
}

func (s *GenerateApiServerConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("kube-apiserver.config.yaml")
	return nil
}

func (s *GenerateApiServerConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
