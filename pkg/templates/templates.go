package templates

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"text/template"
)

//go:embed cd/argocd/*.tmpl
//go:embed cni/calico/*.tmpl
//go:embed cni/cilium/*.tmpl
//go:embed cni/flannel/*.tmpl
//go:embed cni/hybridnet/*.tmpl
//go:embed cni/kubeovn/*.tmpl
//go:embed cni/multus/*.tmpl
//go:embed containerd/*.tmpl
//go:embed crio/*.tmpl
//go:embed dns/*.tmpl
//go:embed docker/*.tmpl
//go:embed etcd/*.tmpl
//go:embed gateway/ingress-nginx/*.tmpl
//go:embed isulad/*.tmpl
//go:embed kubernetes/audit/*.tmpl
//go:embed kubernetes/kube-apiserver/*.tmpl
//go:embed kubernetes/kube-controller-manager/*.tmpl
//go:embed kubernetes/kube-proxy/*.tmpl
//go:embed kubernetes/kube-scheduler/*.tmpl
//go:embed kubernetes/kubeadm/*.tmpl
//go:embed kubernetes/kubeconfig/*.tmpl
//go:embed kubernetes/kubectl/*.tmpl
//go:embed kubernetes/kubelet/*.tmpl
//go:embed kubernetes/rbac/*.tmpl
//go:embed loadbalancer/haproxy/*.tmpl
//go:embed loadbalancer/nginx/*.tmpl
//go:embed loadbalancer/keepalived/*.tmpl
//go:embed os/*.tmpl
//go:embed storage/longhorn/*.tmpl
//go:embed storage/nfs/*.tmpl
//go:embed storage/openebs-local/*.tmpl
var embeddedTemplates embed.FS

func Get(templateName string) (string, error) {
	content, err := fs.ReadFile(embeddedTemplates, templateName)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded template '%s': %w", templateName, err)
	}
	return string(content), nil
}

func List() ([]string, error) {
	var files []string
	err := fs.WalkDir(embeddedTemplates, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk embedded templates: %w", err)
	}
	return files, nil
}

func Render(templateContent string, data interface{}) (string, error) {
	tmpl, err := template.New("").Parse(templateContent)
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", err
	}

	return buffer.String(), nil
}
