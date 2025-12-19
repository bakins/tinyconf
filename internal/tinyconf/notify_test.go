package tinyconf

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockServiceNotifier struct {
	restartCalled []string
	restartErr    error
}

func newMockServiceNotifier() *mockServiceNotifier {
	return &mockServiceNotifier{
		restartCalled: make([]string, 0),
	}
}

func (m *mockServiceNotifier) Restart(ctx context.Context, service string) error {
	m.restartCalled = append(m.restartCalled, service)
	if m.restartErr != nil {
		return m.restartErr
	}
	return nil
}

func TestNotifyServices_EmptyList(t *testing.T) {
	mock := newMockServiceNotifier()
	services := []string{}

	err := notifyServices(t.Context(), mock, services)
	require.NoError(t, err)
	require.Empty(t, mock.restartCalled)
}

func TestNotifyServices_SingleService(t *testing.T) {
	mock := newMockServiceNotifier()
	services := []string{"nginx"}

	err := notifyServices(t.Context(), mock, services)
	require.NoError(t, err)
	require.Len(t, mock.restartCalled, 1)
	require.Contains(t, mock.restartCalled, "nginx")
}

func TestNotifyServices_MultipleServices(t *testing.T) {
	mock := newMockServiceNotifier()
	services := []string{"nginx", "mysql", "redis"}

	err := notifyServices(t.Context(), mock, services)
	require.NoError(t, err)
	require.Len(t, mock.restartCalled, 3)
	require.Equal(t, []string{"nginx", "mysql", "redis"}, mock.restartCalled)
}

func TestNotifyServices_PreservesOrder(t *testing.T) {
	mock := newMockServiceNotifier()
	services := []string{"service1", "service2", "service3"}

	err := notifyServices(t.Context(), mock, services)
	require.NoError(t, err)
	require.Equal(t, services, mock.restartCalled)
}

func TestNotifyServices_ErrorOnRestart(t *testing.T) {
	mock := newMockServiceNotifier()
	mock.restartErr = errors.New("failed to restart service")
	services := []string{"nginx"}

	err := notifyServices(t.Context(), mock, services)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to restart service")
	require.Len(t, mock.restartCalled, 1)
}

func TestNotifyServices_StopsOnFirstError(t *testing.T) {
	mock := newMockServiceNotifier()
	mock.restartErr = errors.New("restart failed")
	services := []string{"nginx", "mysql", "redis"}

	err := notifyServices(t.Context(), mock, services)
	require.Error(t, err)
	// Should only have tried to restart the first service
	require.Len(t, mock.restartCalled, 1)
	require.Equal(t, "nginx", mock.restartCalled[0])
}

func TestNotifyServices_DuplicateServices(t *testing.T) {
	mock := newMockServiceNotifier()
	// The caller should deduplicate, but notifyServices will restart each
	// TODO: should we dedup in notifyServices?
	services := []string{"nginx", "nginx", "mysql"}

	err := notifyServices(t.Context(), mock, services)
	require.NoError(t, err)
	require.Len(t, mock.restartCalled, 3)
	require.Equal(t, []string{"nginx", "nginx", "mysql"}, mock.restartCalled)
}

func TestNotifyServices_ErrorContainsServiceInfo(t *testing.T) {
	mock := newMockServiceNotifier()
	mock.restartErr = errors.New("connection refused")
	services := []string{"critical-service"}

	err := notifyServices(t.Context(), mock, services)
	require.Error(t, err)
	require.Contains(t, err.Error(), "refused")
}

func TestNotifyServices_MultipleCalls(t *testing.T) {
	mock := newMockServiceNotifier()

	err := notifyServices(t.Context(), mock, []string{"nginx"})
	require.NoError(t, err)

	err = notifyServices(t.Context(), mock, []string{"mysql"})
	require.NoError(t, err)

	err = notifyServices(t.Context(), mock, []string{"redis"})
	require.NoError(t, err)

	// All calls should have been recorded
	require.Len(t, mock.restartCalled, 3)
	require.Equal(t, []string{"nginx", "mysql", "redis"}, mock.restartCalled)
}
