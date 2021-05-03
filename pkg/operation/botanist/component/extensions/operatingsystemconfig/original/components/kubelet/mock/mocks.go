// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/kubelet (interfaces: ConfigCodec)

// Package kubelet is a generated GoMock package.
package kubelet

import (
	reflect "reflect"

	v1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gomock "github.com/golang/mock/gomock"
	v1beta1 "k8s.io/kubelet/config/v1beta1"
)

// MockConfigCodec is a mock of ConfigCodec interface.
type MockConfigCodec struct {
	ctrl     *gomock.Controller
	recorder *MockConfigCodecMockRecorder
}

// MockConfigCodecMockRecorder is the mock recorder for MockConfigCodec.
type MockConfigCodecMockRecorder struct {
	mock *MockConfigCodec
}

// NewMockConfigCodec creates a new mock instance.
func NewMockConfigCodec(ctrl *gomock.Controller) *MockConfigCodec {
	mock := &MockConfigCodec{ctrl: ctrl}
	mock.recorder = &MockConfigCodecMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConfigCodec) EXPECT() *MockConfigCodecMockRecorder {
	return m.recorder
}

// Decode mocks base method.
func (m *MockConfigCodec) Decode(arg0 *v1alpha1.FileContentInline) (*v1beta1.KubeletConfiguration, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Decode", arg0)
	ret0, _ := ret[0].(*v1beta1.KubeletConfiguration)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Decode indicates an expected call of Decode.
func (mr *MockConfigCodecMockRecorder) Decode(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Decode", reflect.TypeOf((*MockConfigCodec)(nil).Decode), arg0)
}

// Encode mocks base method.
func (m *MockConfigCodec) Encode(arg0 *v1beta1.KubeletConfiguration, arg1 string) (*v1alpha1.FileContentInline, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Encode", arg0, arg1)
	ret0, _ := ret[0].(*v1alpha1.FileContentInline)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Encode indicates an expected call of Encode.
func (mr *MockConfigCodecMockRecorder) Encode(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Encode", reflect.TypeOf((*MockConfigCodec)(nil).Encode), arg0, arg1)
}
