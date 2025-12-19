package tinyconf

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// track state of services and
// how many calls for stop/start we get
// could use https://pkg.go.dev/github.com/stretchr/testify/mock
// but that's a bit much for this, but this is inspired
// by that and https://github.com/vektra/mockery
type mockServiceManager struct {
	services       map[string]bool // service name -> running state
	isRunningErr   error
	startErr       error
	stopErr        error
	startCalled    []string
	stopCalled     []string
	isRunningCalls []string
}

func newMockServiceManager() *mockServiceManager {
	return &mockServiceManager{
		services: make(map[string]bool),
	}
}

func (m *mockServiceManager) IsRunning(ctx context.Context, service string) (bool, error) {
	m.isRunningCalls = append(m.isRunningCalls, service)
	if m.isRunningErr != nil {
		return false, m.isRunningErr
	}
	return m.services[service], nil
}

func (m *mockServiceManager) Start(ctx context.Context, service string) error {
	m.startCalled = append(m.startCalled, service)
	if m.startErr != nil {
		return m.startErr
	}
	m.services[service] = true
	return nil
}

func (m *mockServiceManager) Stop(ctx context.Context, service string) error {
	m.stopCalled = append(m.stopCalled, service)
	if m.stopErr != nil {
		return m.stopErr
	}
	m.services[service] = false
	return nil
}

func TestServiceResource_Run_StartStoppedService(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = false

	s := &serviceResource{
		Name:    "nginx",
		State:   "running",
		manager: mock,
	}

	service, err := s.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	require.True(t, mock.services["nginx"])
	require.Contains(t, mock.startCalled, "nginx")
	require.Contains(t, mock.isRunningCalls, "nginx")
}

func TestServiceResource_Run_StopRunningService(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = true

	s := &serviceResource{
		Name:    "nginx",
		State:   "stopped",
		manager: mock,
	}

	service, err := s.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	require.False(t, mock.services["nginx"])
	require.Contains(t, mock.stopCalled, "nginx")
	require.Contains(t, mock.isRunningCalls, "nginx")
}

func TestServiceResource_Run_ServiceAlreadyRunning(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = true

	s := &serviceResource{
		Name:    "nginx",
		State:   "running",
		manager: mock,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := s.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	require.Empty(t, mock.startCalled)
	require.Empty(t, mock.stopCalled)
	require.Contains(t, mock.isRunningCalls, "nginx")
}

func TestServiceResource_Run_ServiceAlreadyStopped(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = false

	s := &serviceResource{
		Name:    "nginx",
		State:   "stopped",
		manager: mock,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := s.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	require.Empty(t, mock.startCalled)
	require.Empty(t, mock.stopCalled)
	require.Contains(t, mock.isRunningCalls, "nginx")
}

func TestServiceResource_Run_WithNotification(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = false

	s := &serviceResource{
		Name:  "nginx",
		State: "running",
		Notify: notifyResource{
			Service: "my-service",
		},
		manager: mock,
	}

	service, err := s.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "my-service", service)

	require.True(t, mock.services["nginx"])
}

func TestServiceResource_Run_ErrorIsRunning(t *testing.T) {
	mock := newMockServiceManager()
	mock.isRunningErr = errors.New("failed to check status")

	s := &serviceResource{
		Name:    "nginx",
		State:   "running",
		manager: mock,
	}

	_, err := s.Run(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get status")
}

func TestServiceResource_Run_ErrorStart(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = false
	mock.startErr = errors.New("failed to start service")

	s := &serviceResource{
		Name:    "nginx",
		State:   "running",
		manager: mock,
	}

	_, err := s.Run(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to start service")
}

func TestServiceResource_Run_ErrorStop(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = true
	mock.stopErr = errors.New("failed to stop service")

	s := &serviceResource{
		Name:    "nginx",
		State:   "stopped",
		manager: mock,
	}

	_, err := s.Run(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to stop service")
}

func TestServiceResource_Run_MultipleServices(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = false
	mock.services["mysql"] = true

	nginx := &serviceResource{
		Name:    "nginx",
		State:   "running",
		manager: mock,
	}

	mysql := &serviceResource{
		Name:    "mysql",
		State:   "stopped",
		manager: mock,
	}

	ctx := t.Context()

	_, err := nginx.Run(ctx)
	require.NoError(t, err)
	require.True(t, mock.services["nginx"])

	_, err = mysql.Run(ctx)
	require.NoError(t, err)
	require.False(t, mock.services["mysql"])

	require.Contains(t, mock.startCalled, "nginx")
	require.Contains(t, mock.stopCalled, "mysql")
}

func TestServiceResource_Run_RunMultipleTimes(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = false

	s := &serviceResource{
		Name:  "nginx",
		State: "running",
		Notify: notifyResource{
			Service: "test-service",
		},
		manager: mock,
	}

	ctx := t.Context()

	// First run - should start service
	service1, err := s.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, "test-service", service1)
	require.True(t, mock.services["nginx"])

	// Second run - should be idempotent
	service2, err := s.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service2)
	require.True(t, mock.services["nginx"])

	// Third run - should still be idempotent
	service3, err := s.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service3)
	require.True(t, mock.services["nginx"])

	// Should have called start only once
	require.Len(t, mock.startCalled, 1)
}

func TestServiceResource_Run_StartAndStopCycle(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = false

	s := &serviceResource{
		Name:    "nginx",
		manager: mock,
	}

	ctx := t.Context()

	s.State = "running"
	service, err := s.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service)
	require.True(t, mock.services["nginx"])

	s.State = "stopped"
	service, err = s.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service)
	require.False(t, mock.services["nginx"])

	s.State = "running"
	service, err = s.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service)
	require.True(t, mock.services["nginx"])

	require.Len(t, mock.startCalled, 2)
	require.Len(t, mock.stopCalled, 1)
}

func TestServiceResource_Run_StopWithNotification(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = true

	s := &serviceResource{
		Name:  "nginx",
		State: "stopped",
		Notify: notifyResource{
			Service: "monitor-service",
		},
		manager: mock,
	}

	service, err := s.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "monitor-service", service)
	require.False(t, mock.services["nginx"])
}

func TestServiceResource_Run_NoNotifyOnNoChange(t *testing.T) {
	mock := newMockServiceManager()
	mock.services["nginx"] = true

	s := &serviceResource{
		Name:  "nginx",
		State: "running",
		Notify: notifyResource{
			Service: "should-not-notify",
		},
		manager: mock,
	}

	service, err := s.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)
}
