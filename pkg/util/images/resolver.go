package images

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"strings"
)

const (
	ChangePrefix v1alpha1.NamespaceRewritePolicy = "ChangePrefix"
)

type RewriteOptions struct {
	Registry  string
	Namespace string
}

type Resolver struct {
	rewrite          RewriteOptions
	namespaceRewrite *v1alpha1.NamespaceRewrite
}

func NewResolver(rewrite RewriteOptions, namespaceRewrite *v1alpha1.NamespaceRewrite) *Resolver {
	return &Resolver{
		rewrite:          rewrite,
		namespaceRewrite: namespaceRewrite,
	}
}

func (r *Resolver) Full(img Image) string {
	return fmt.Sprintf("%s:%s", r.Repo(img), img.Tag)
}

func (r *Resolver) Repo(img Image) string {
	var (
		registry  = img.RepoAddr
		namespace = img.Namespace
	)

	if r.rewrite.Registry != "" {
		registry = r.rewrite.Registry
	}
	if r.rewrite.Namespace != "" {
		namespace = r.rewrite.Namespace
	}

	if r.namespaceRewrite != nil {
		switch r.namespaceRewrite.Policy {
		case ChangePrefix:
			matchFound := false
			for _, src := range r.namespaceRewrite.Src {
				if strings.HasPrefix(namespace, src) {
					namespace = strings.Replace(namespace, src, r.namespaceRewrite.Dest, 1)
					matchFound = true
					break
				}
			}
			if !matchFound {
				namespace = fmt.Sprintf("%s/%s", r.namespaceRewrite.Dest, namespace)
			}
		default:
		}
	}

	var parts []string
	if registry != "" {
		parts = append(parts, registry)
	}
	if namespace != "" {
		parts = append(parts, namespace)
	}
	parts = append(parts, img.Repo)

	return strings.Join(parts, "/")
}
