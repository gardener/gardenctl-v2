// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardenctl-v2/pkg/target (interfaces: Manager)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	gardenclient "github.com/gardener/gardenctl-v2/internal/gardenclient"
	config "github.com/gardener/gardenctl-v2/pkg/config"
	target "github.com/gardener/gardenctl-v2/pkg/target"
)

// MockManager is a mock of Manager interface.
type MockManager struct {
	ctrl     *gomock.Controller
	recorder *MockManagerMockRecorder
}

// MockManagerMockRecorder is the mock recorder for MockManager.
type MockManagerMockRecorder struct {
	mock *MockManager
}

// NewMockManager creates a new mock instance.
func NewMockManager(ctrl *gomock.Controller) *MockManager {
	mock := &MockManager{ctrl: ctrl}
	mock.recorder = &MockManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockManager) EXPECT() *MockManagerMockRecorder {
	return m.recorder
}

// Configuration mocks base method.
func (m *MockManager) Configuration() *config.Config {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Configuration")
	ret0, _ := ret[0].(*config.Config)
	return ret0
}

// Configuration indicates an expected call of Configuration.
func (mr *MockManagerMockRecorder) Configuration() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Configuration", reflect.TypeOf((*MockManager)(nil).Configuration))
}

// CurrentTarget mocks base method.
func (m *MockManager) CurrentTarget() (target.Target, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CurrentTarget")
	ret0, _ := ret[0].(target.Target)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CurrentTarget indicates an expected call of CurrentTarget.
func (mr *MockManagerMockRecorder) CurrentTarget() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CurrentTarget", reflect.TypeOf((*MockManager)(nil).CurrentTarget))
}

// GardenClient mocks base method.
func (m *MockManager) GardenClient(name string) (gardenclient.Client, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GardenClient", name)
	ret0, _ := ret[0].(gardenclient.Client)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GardenClient indicates an expected call of GardenClient.
func (mr *MockManagerMockRecorder) GardenClient(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GardenClient", reflect.TypeOf((*MockManager)(nil).GardenClient), name)
}

// Kubeconfig mocks base method.
func (m *MockManager) Kubeconfig(ctx context.Context, t target.Target) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Kubeconfig", ctx, t)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Kubeconfig indicates an expected call of Kubeconfig.
func (mr *MockManagerMockRecorder) Kubeconfig(ctx, t interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Kubeconfig", reflect.TypeOf((*MockManager)(nil).Kubeconfig), ctx, t)
}

// SeedClient mocks base method.
func (m *MockManager) SeedClient(ctx context.Context, t target.Target) (client.Client, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SeedClient", ctx, t)
	ret0, _ := ret[0].(client.Client)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SeedClient indicates an expected call of SeedClient.
func (mr *MockManagerMockRecorder) SeedClient(ctx, t interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SeedClient", reflect.TypeOf((*MockManager)(nil).SeedClient), ctx, t)
}

// ShootClient mocks base method.
func (m *MockManager) ShootClient(ctx context.Context, t target.Target) (client.Client, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ShootClient", ctx, t)
	ret0, _ := ret[0].(client.Client)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ShootClient indicates an expected call of ShootClient.
func (mr *MockManagerMockRecorder) ShootClient(ctx, t interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ShootClient", reflect.TypeOf((*MockManager)(nil).ShootClient), ctx, t)
}

// TargetControlPlane mocks base method.
func (m *MockManager) TargetControlPlane(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TargetControlPlane", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// TargetControlPlane indicates an expected call of TargetControlPlane.
func (mr *MockManagerMockRecorder) TargetControlPlane(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TargetControlPlane", reflect.TypeOf((*MockManager)(nil).TargetControlPlane), ctx)
}

// TargetFlags mocks base method.
func (m *MockManager) TargetFlags() target.TargetFlags {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TargetFlags")
	ret0, _ := ret[0].(target.TargetFlags)
	return ret0
}

// TargetFlags indicates an expected call of TargetFlags.
func (mr *MockManagerMockRecorder) TargetFlags() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TargetFlags", reflect.TypeOf((*MockManager)(nil).TargetFlags))
}

// TargetGarden mocks base method.
func (m *MockManager) TargetGarden(ctx context.Context, name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TargetGarden", ctx, name)
	ret0, _ := ret[0].(error)
	return ret0
}

// TargetGarden indicates an expected call of TargetGarden.
func (mr *MockManagerMockRecorder) TargetGarden(ctx, name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TargetGarden", reflect.TypeOf((*MockManager)(nil).TargetGarden), ctx, name)
}

// TargetMatchPattern mocks base method.
func (m *MockManager) TargetMatchPattern(ctx context.Context, value string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TargetMatchPattern", ctx, value)
	ret0, _ := ret[0].(error)
	return ret0
}

// TargetMatchPattern indicates an expected call of TargetMatchPattern.
func (mr *MockManagerMockRecorder) TargetMatchPattern(ctx, value interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TargetMatchPattern", reflect.TypeOf((*MockManager)(nil).TargetMatchPattern), ctx, value)
}

// TargetProject mocks base method.
func (m *MockManager) TargetProject(ctx context.Context, name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TargetProject", ctx, name)
	ret0, _ := ret[0].(error)
	return ret0
}

// TargetProject indicates an expected call of TargetProject.
func (mr *MockManagerMockRecorder) TargetProject(ctx, name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TargetProject", reflect.TypeOf((*MockManager)(nil).TargetProject), ctx, name)
}

// TargetSeed mocks base method.
func (m *MockManager) TargetSeed(ctx context.Context, name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TargetSeed", ctx, name)
	ret0, _ := ret[0].(error)
	return ret0
}

// TargetSeed indicates an expected call of TargetSeed.
func (mr *MockManagerMockRecorder) TargetSeed(ctx, name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TargetSeed", reflect.TypeOf((*MockManager)(nil).TargetSeed), ctx, name)
}

// TargetShoot mocks base method.
func (m *MockManager) TargetShoot(ctx context.Context, name string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TargetShoot", ctx, name)
	ret0, _ := ret[0].(error)
	return ret0
}

// TargetShoot indicates an expected call of TargetShoot.
func (mr *MockManagerMockRecorder) TargetShoot(ctx, name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TargetShoot", reflect.TypeOf((*MockManager)(nil).TargetShoot), ctx, name)
}

// UnsetTargetControlPlane mocks base method.
func (m *MockManager) UnsetTargetControlPlane() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnsetTargetControlPlane")
	ret0, _ := ret[0].(error)
	return ret0
}

// UnsetTargetControlPlane indicates an expected call of UnsetTargetControlPlane.
func (mr *MockManagerMockRecorder) UnsetTargetControlPlane() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnsetTargetControlPlane", reflect.TypeOf((*MockManager)(nil).UnsetTargetControlPlane))
}

// UnsetTargetGarden mocks base method.
func (m *MockManager) UnsetTargetGarden() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnsetTargetGarden")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UnsetTargetGarden indicates an expected call of UnsetTargetGarden.
func (mr *MockManagerMockRecorder) UnsetTargetGarden() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnsetTargetGarden", reflect.TypeOf((*MockManager)(nil).UnsetTargetGarden))
}

// UnsetTargetProject mocks base method.
func (m *MockManager) UnsetTargetProject() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnsetTargetProject")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UnsetTargetProject indicates an expected call of UnsetTargetProject.
func (mr *MockManagerMockRecorder) UnsetTargetProject() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnsetTargetProject", reflect.TypeOf((*MockManager)(nil).UnsetTargetProject))
}

// UnsetTargetSeed mocks base method.
func (m *MockManager) UnsetTargetSeed() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnsetTargetSeed")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UnsetTargetSeed indicates an expected call of UnsetTargetSeed.
func (mr *MockManagerMockRecorder) UnsetTargetSeed() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnsetTargetSeed", reflect.TypeOf((*MockManager)(nil).UnsetTargetSeed))
}

// UnsetTargetShoot mocks base method.
func (m *MockManager) UnsetTargetShoot() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnsetTargetShoot")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UnsetTargetShoot indicates an expected call of UnsetTargetShoot.
func (mr *MockManagerMockRecorder) UnsetTargetShoot() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnsetTargetShoot", reflect.TypeOf((*MockManager)(nil).UnsetTargetShoot))
}

// WriteKubeconfig mocks base method.
func (m *MockManager) WriteKubeconfig(data []byte) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteKubeconfig", data)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WriteKubeconfig indicates an expected call of WriteKubeconfig.
func (mr *MockManagerMockRecorder) WriteKubeconfig(data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteKubeconfig", reflect.TypeOf((*MockManager)(nil).WriteKubeconfig), data)
}
