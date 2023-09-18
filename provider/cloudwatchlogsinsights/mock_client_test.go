// Code generated by MockGen. DO NOT EDIT.
// Source: cloudwatchlogsinsights_test.go

// Package cloudwatchlogsinsights_test is a generated GoMock package.
package cloudwatchlogsinsights_test

import (
	context "context"
	reflect "reflect"

	cloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	gomock "go.uber.org/mock/gomock"
)

// MockCloudwatchLogsClient is a mock of CloudwatchLogsClient interface.
type MockCloudwatchLogsClient struct {
	ctrl     *gomock.Controller
	recorder *MockCloudwatchLogsClientMockRecorder
}

// MockCloudwatchLogsClientMockRecorder is the mock recorder for MockCloudwatchLogsClient.
type MockCloudwatchLogsClientMockRecorder struct {
	mock *MockCloudwatchLogsClient
}

// NewMockCloudwatchLogsClient creates a new mock instance.
func NewMockCloudwatchLogsClient(ctrl *gomock.Controller) *MockCloudwatchLogsClient {
	mock := &MockCloudwatchLogsClient{ctrl: ctrl}
	mock.recorder = &MockCloudwatchLogsClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCloudwatchLogsClient) EXPECT() *MockCloudwatchLogsClientMockRecorder {
	return m.recorder
}

// GetQueryResults mocks base method.
func (m *MockCloudwatchLogsClient) GetQueryResults(ctx context.Context, params *cloudwatchlogs.GetQueryResultsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetQueryResultsOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, params}
	for _, a := range optFns {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetQueryResults", varargs...)
	ret0, _ := ret[0].(*cloudwatchlogs.GetQueryResultsOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetQueryResults indicates an expected call of GetQueryResults.
func (mr *MockCloudwatchLogsClientMockRecorder) GetQueryResults(ctx, params interface{}, optFns ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, params}, optFns...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetQueryResults", reflect.TypeOf((*MockCloudwatchLogsClient)(nil).GetQueryResults), varargs...)
}

// StartQuery mocks base method.
func (m *MockCloudwatchLogsClient) StartQuery(ctx context.Context, params *cloudwatchlogs.StartQueryInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartQueryOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, params}
	for _, a := range optFns {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "StartQuery", varargs...)
	ret0, _ := ret[0].(*cloudwatchlogs.StartQueryOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartQuery indicates an expected call of StartQuery.
func (mr *MockCloudwatchLogsClientMockRecorder) StartQuery(ctx, params interface{}, optFns ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, params}, optFns...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartQuery", reflect.TypeOf((*MockCloudwatchLogsClient)(nil).StartQuery), varargs...)
}

// StopQuery mocks base method.
func (m *MockCloudwatchLogsClient) StopQuery(ctx context.Context, params *cloudwatchlogs.StopQueryInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StopQueryOutput, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{ctx, params}
	for _, a := range optFns {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "StopQuery", varargs...)
	ret0, _ := ret[0].(*cloudwatchlogs.StopQueryOutput)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StopQuery indicates an expected call of StopQuery.
func (mr *MockCloudwatchLogsClientMockRecorder) StopQuery(ctx, params interface{}, optFns ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{ctx, params}, optFns...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopQuery", reflect.TypeOf((*MockCloudwatchLogsClient)(nil).StopQuery), varargs...)
}