package spec

type StepMeta struct {
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Hidden       bool   `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	AllowFailure bool   `json:"allowFailure,omitempty" yaml:"allowFailure,omitempty"`
}

type TaskMeta struct {
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Hidden       bool   `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	AllowFailure bool   `json:"allowFailure,omitempty" yaml:"allowFailure,omitempty"`
}

type ModuleMeta struct {
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Hidden       bool   `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	AllowFailure bool   `json:"allowFailure,omitempty" yaml:"allowFailure,omitempty"`
}

type PipelineMeta struct {
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	Hidden       bool   `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	AllowFailure bool   `json:"allowFailure,omitempty" yaml:"allowFailure,omitempty"`
}
