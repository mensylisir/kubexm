package v1alpha1

type HelmRepo struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	Repo string `json:"repo,omitempty" yaml:"repo,omitempty"`
}
