// Package datashare provides utilities for data sharing between steps
package datashare

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// StepDataHelper provides helper functions for step data sharing
type StepDataHelper struct {
	dataBus *runtime.SimpleDataBus
}

// NewStepDataHelper creates a new StepDataHelper
func NewStepDataHelper(ctx runtime.ExecutionContext) *StepDataHelper {
	return &StepDataHelper{
		dataBus: runtime.NewSimpleDataBus(ctx),
	}
}

// KubeadmInitData holds the data produced by kubeadm init step
type KubeadmInitData struct {
	Token      string
	CertKey    string
	CACertHash string
}

// PublishKubeadmInitData publishes kubeadm init data
func (h *StepDataHelper) PublishKubeadmInitData(data KubeadmInitData) error {
	return h.dataBus.PublishKubeadmInitData(data.Token, data.CertKey, data.CACertHash)
}

// SubscribeKubeadmInitData subscribes to kubeadm init data
func (h *StepDataHelper) SubscribeKubeadmInitData() (*KubeadmInitData, error) {
	token, found := h.dataBus.SubscribeKubeadmInitToken()
	if !found {
		return nil, fmt.Errorf("kubeadm init token not found")
	}

	certKey, found := h.dataBus.SubscribeKubeadmInitCertKey()
	if !found {
		return nil, fmt.Errorf("kubeadm init cert key not found")
	}

	caCertHash, found := h.dataBus.SubscribeKubeadmInitCACertHash()
	if !found {
		return nil, fmt.Errorf("kubeadm init CA cert hash not found")
	}

	return &KubeadmInitData{
		Token:      token,
		CertKey:    certKey,
		CACertHash: caCertHash,
	}, nil
}

// PublishString publishes a string value
func (h *StepDataHelper) PublishString(key, value string) error {
	return h.dataBus.Publish("task", key, value)
}

// PublishInt publishes an int value
func (h *StepDataHelper) PublishInt(key string, value int) error {
	return h.dataBus.Publish("task", key, value)
}

// PublishBool publishes a bool value
func (h *StepDataHelper) PublishBool(key string, value bool) error {
	return h.dataBus.Publish("task", key, value)
}

// SubscribeString subscribes to a string value
func (h *StepDataHelper) SubscribeString(key string) (string, bool) {
	return h.dataBus.SubscribeString(key)
}

// SubscribeInt subscribes to an int value
func (h *StepDataHelper) SubscribeInt(key string) (int, bool) {
	return h.dataBus.SubscribeInt(key)
}

// SubscribeBool subscribes to a bool value
func (h *StepDataHelper) SubscribeBool(key string) (bool, bool) {
	return h.dataBus.SubscribeBool(key)
}