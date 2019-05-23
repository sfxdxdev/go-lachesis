// Code generated by MockGen. DO NOT EDIT.
// Source: events.go

// Package sorting is a generated GoMock package.
package sorting

import (
	hash "github.com/Fantom-foundation/go-lachesis/src/hash"
	inter "github.com/Fantom-foundation/go-lachesis/src/inter"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockSource is a mock of Source interface
type MockSource struct {
	ctrl     *gomock.Controller
	recorder *MockSourceMockRecorder
}

// MockSourceMockRecorder is the mock recorder for MockSource
type MockSourceMockRecorder struct {
	mock *MockSource
}

// NewMockSource creates a new mock instance
func NewMockSource(ctrl *gomock.Controller) *MockSource {
	mock := &MockSource{ctrl: ctrl}
	mock.recorder = &MockSourceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockSource) EXPECT() *MockSourceMockRecorder {
	return m.recorder
}

// GetEvent mocks base method
func (m *MockSource) GetEvent(arg0 hash.Event) *inter.Event {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEvent", arg0)
	ret0, _ := ret[0].(*inter.Event)
	return ret0
}

// GetEvent indicates an expected call of GetEvent
func (mr *MockSourceMockRecorder) GetEvent(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEvent", reflect.TypeOf((*MockSource)(nil).GetEvent), arg0)
}

// SetEvent mocks base method
func (m *MockSource) SetEvent(e *inter.Event) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetEvent", e)
}

// SetEvent indicates an expected call of SetEvent
func (mr *MockSourceMockRecorder) SetEvent(e interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetEvent", reflect.TypeOf((*MockSource)(nil).SetEvent), e)
}
