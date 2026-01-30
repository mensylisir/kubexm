package runtime

import (
	"fmt"
	"sync"
)

// TypedStateBag provides type-safe state storage for step data passing.
// This replaces the generic string-based StateBag for better type safety.
type TypedStateBag[T any] struct {
	data map[string]T
	mu   sync.RWMutex
}

func NewTypedStateBag[T any]() *TypedStateBag[T] {
	return &TypedStateBag[T]{
		data: make(map[string]T),
	}
}

func (s *TypedStateBag[T]) Set(key string, value T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

func (s *TypedStateBag[T]) Get(key string) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *TypedStateBag[T]) GetOrDefault(key string, defaultValue T) T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if val, ok := s.data[key]; ok {
		return val
	}
	return defaultValue
}

func (s *TypedStateBag[T]) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *TypedStateBag[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// StepOutput is a marker interface for step outputs.
// Steps should define their output as a concrete struct that implements this.
type StepOutput interface {
	isStepOutput()
}

// StepInput is a marker interface for step inputs.
// Steps should define their input as a concrete struct that implements this.
type StepInput interface {
	isStepInput()
}

// Common step output types for reuse

type DownloadOutput struct {
	DownloadPath string
	Checksum     string
	ChecksumType string
	ArchiveName  string
}

func (*DownloadOutput) isStepOutput() {}

type ExtractOutput struct {
	ExtractedPath string
	ArchivePath   string
}

func (*ExtractOutput) isStepOutput() {}

type CertificateOutput struct {
	CertPath string
	KeyPath  string
	CAPath   string
}

func (*CertificateOutput) isStepOutput() {}

type ServiceOutput struct {
	ServiceName string
	ServicePath string
	ConfigPath  string
	IsActive    bool
	IsEnabled   bool
}

func (*ServiceOutput) isStepOutput() {}

type HelmOutput struct {
	ReleaseName string
	Namespace   string
	ChartPath   string
	ValuesPath  string
}

func (*HelmOutput) isStepOutput() {}

type KubernetesOutput struct {
	KubeconfigPath string
	ContextName    string
	Namespace      string
}

func (*KubernetesOutput) isStepOutput() {}

type NetworkOutput struct {
	InterfaceName string
	IPAddress     string
	SubnetMask    string
	Gateway       string
	DNS           []string
}

func (*NetworkOutput) isStepOutput() {}

type StorageOutput struct {
	VolumePath string
	MountPath  string
	Filesystem string
	Size       string
}

func (*StorageOutput) isStepOutput() {}

// Registry for step output types
type StepOutputRegistry struct {
	registry map[string]StepOutput
	mu       sync.RWMutex
}

var GlobalStepOutputRegistry = NewStepOutputRegistry()

func NewStepOutputRegistry() *StepOutputRegistry {
	return &StepOutputRegistry{
		registry: make(map[string]StepOutput),
	}
}

func (r *StepOutputRegistry) Register(name string, output StepOutput) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registry[name] = output
}

func (r *StepOutputRegistry) Get(name string) (StepOutput, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	output, ok := r.registry[name]
	return output, ok
}

func (r *StepOutputRegistry) MustGet(name string) StepOutput {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if output, ok := r.registry[name]; ok {
		return output
	}
	panic(fmt.Sprintf("step output type not registered: %s", name))
}
