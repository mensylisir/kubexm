package v1alpha1

type Extra struct {
	Hosts    []EtcHost    `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	Resolves []EtcResolve `json:"resolves,omitempty" yaml:"resolves,omitempty"`
}

type EtcHost struct {
	Domain  string `json:"domain,omitempty" yaml:"domain,omitempty"`
	Address string `json:"address,omitempty" yaml:"address,omitempty"`
}

type EtcResolve struct {
	Address string `json:"address,omitempty" yaml:"address,omitempty"`
}
