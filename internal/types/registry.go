package types

// NamespaceRewrite defines rules for rewriting image namespaces
type NamespaceRewrite struct {
	Enabled *bool                  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Rules   []NamespaceRewriteRule `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// NamespaceRewriteRule defines a single rule for namespace rewriting
type NamespaceRewriteRule struct {
	Registry     string `json:"registry,omitempty" yaml:"registry,omitempty"`
	OldNamespace string `json:"oldNamespace" yaml:"oldNamespace"`
	NewNamespace string `json:"newNamespace" yaml:"newNamespace"`
}

// RegistryMirroringAndRewriting defines registry mirroring and namespace rewriting configuration
type RegistryMirroringAndRewriting struct {
	PrivateRegistry   string            `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
	NamespaceOverride string            `json:"namespaceOverride,omitempty" yaml:"namespaceOverride,omitempty"`
	NamespaceRewrite  *NamespaceRewrite `json:"namespaceRewrite,omitempty" yaml:"namespaceRewrite,omitempty"`
}