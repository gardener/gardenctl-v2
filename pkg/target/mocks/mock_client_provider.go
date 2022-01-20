// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardenctl-v2/pkg/target (interfaces: ClientProvider)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// MockClientProvider is a mock of ClientProvider interface.
type MockClientProvider struct {
	ctrl     *gomock.Controller
	recorder *MockClientProviderMockRecorder
}

// MockClientProviderMockRecorder is the mock recorder for MockClientProvider.
type MockClientProviderMockRecorder struct {
	mock *MockClientProvider
}

// NewMockClientProvider creates a new mock instance.
func NewMockClientProvider(ctrl *gomock.Controller) *MockClientProvider {
	mock := &MockClientProvider{ctrl: ctrl}
	mock.recorder = &MockClientProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClientProvider) EXPECT() *MockClientProviderMockRecorder {
	return m.recorder
}

// FromClientConfig mocks base method.
func (m *MockClientProvider) FromClientConfig(arg0 clientcmd.ClientConfig) (client.Client, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FromClientConfig", arg0)
	ret0, _ := ret[0].(client.Client)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FromClientConfig indicates an expected call of FromClientConfig.
func (mr *MockClientProviderMockRecorder) FromClientConfig(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FromClientConfig", reflect.TypeOf((*MockClientProvider)(nil).FromClientConfig), arg0)
}
