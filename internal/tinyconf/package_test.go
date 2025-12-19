package tinyconf

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// package is very similar to service
// copy/paste search/replace :)

type mockPackageManager struct {
	packages         map[string]bool // package name -> installed state
	isInstalledErr   error
	installErr       error
	uninstallErr     error
	installCalled    []string
	uninstallCalled  []string
	isInstalledCalls []string
}

func newMockPackageManager() *mockPackageManager {
	return &mockPackageManager{
		packages: make(map[string]bool),
	}
}

func (m *mockPackageManager) IsInstalled(ctx context.Context, packageName string) (bool, error) {
	m.isInstalledCalls = append(m.isInstalledCalls, packageName)
	if m.isInstalledErr != nil {
		return false, m.isInstalledErr
	}
	return m.packages[packageName], nil
}

func (m *mockPackageManager) Install(ctx context.Context, packageName string) error {
	m.installCalled = append(m.installCalled, packageName)
	if m.installErr != nil {
		return m.installErr
	}
	m.packages[packageName] = true
	return nil
}

func (m *mockPackageManager) Uninstall(ctx context.Context, packageName string) error {
	m.uninstallCalled = append(m.uninstallCalled, packageName)
	if m.uninstallErr != nil {
		return m.uninstallErr
	}
	m.packages[packageName] = false
	return nil
}

func TestPackageResource_Run_InstallAbsentPackage(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = false

	p := &packageResource{
		Name:    "nginx",
		State:   "installed",
		manager: mock,
	}

	service, err := p.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	require.True(t, mock.packages["nginx"])
	require.Contains(t, mock.installCalled, "nginx")
	require.Contains(t, mock.isInstalledCalls, "nginx")
}

func TestPackageResource_Run_UninstallInstalledPackage(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = true

	p := &packageResource{
		Name:    "nginx",
		State:   "absent",
		manager: mock,
	}

	service, err := p.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	require.False(t, mock.packages["nginx"])
	require.Contains(t, mock.uninstallCalled, "nginx")
	require.Contains(t, mock.isInstalledCalls, "nginx")
}

func TestPackageResource_Run_PackageAlreadyInstalled(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = true

	p := &packageResource{
		Name:  "nginx",
		State: "installed",
		Notify: notifyResource{
			Service: "test-service",
		},
		manager: mock,
	}

	service, err := p.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	require.Empty(t, mock.installCalled)
	require.Empty(t, mock.uninstallCalled)
	require.Contains(t, mock.isInstalledCalls, "nginx")
}

func TestPackageResource_Run_PackageAlreadyAbsent(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = false

	p := &packageResource{
		Name:  "nginx",
		State: "absent",
		Notify: notifyResource{
			Service: "test-service",
		},
		manager: mock,
	}

	service, err := p.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	require.Empty(t, mock.installCalled)
	require.Empty(t, mock.uninstallCalled)
	require.Contains(t, mock.isInstalledCalls, "nginx")
}

func TestPackageResource_Run_WithNotification(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = false

	p := &packageResource{
		Name:  "nginx",
		State: "installed",
		Notify: notifyResource{
			Service: "my-service",
		},
		manager: mock,
	}

	service, err := p.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "my-service", service)

	require.True(t, mock.packages["nginx"])
}

func TestPackageResource_Run_ErrorIsInstalled(t *testing.T) {
	mock := newMockPackageManager()
	mock.isInstalledErr = errors.New("failed to check package status")

	p := &packageResource{
		Name:    "nginx",
		State:   "installed",
		manager: mock,
	}

	_, err := p.Run(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get status")
}

func TestPackageResource_Run_ErrorInstall(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = false
	mock.installErr = errors.New("failed to install package")

	p := &packageResource{
		Name:    "nginx",
		State:   "installed",
		manager: mock,
	}

	_, err := p.Run(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to install package")
}

func TestPackageResource_Run_ErrorUninstall(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = true
	mock.uninstallErr = errors.New("failed to uninstall package")

	p := &packageResource{
		Name:    "nginx",
		State:   "absent",
		manager: mock,
	}

	_, err := p.Run(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to uninstall package")
}

func TestPackageResource_Run_MultiplePackages(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = false
	mock.packages["mysql"] = true

	nginx := &packageResource{
		Name:    "nginx",
		State:   "installed",
		manager: mock,
	}

	mysql := &packageResource{
		Name:    "mysql",
		State:   "absent",
		manager: mock,
	}

	ctx := t.Context()

	_, err := nginx.Run(ctx)
	require.NoError(t, err)
	require.True(t, mock.packages["nginx"])

	_, err = mysql.Run(ctx)
	require.NoError(t, err)
	require.False(t, mock.packages["mysql"])

	require.Contains(t, mock.installCalled, "nginx")
	require.Contains(t, mock.uninstallCalled, "mysql")
}

func TestPackageResource_Run_RunMultipleTimes(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = false

	p := &packageResource{
		Name:  "nginx",
		State: "installed",
		Notify: notifyResource{
			Service: "test-service",
		},
		manager: mock,
	}

	ctx := t.Context()

	service1, err := p.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, "test-service", service1)
	require.True(t, mock.packages["nginx"])

	service2, err := p.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service2)
	require.True(t, mock.packages["nginx"])

	service3, err := p.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service3)
	require.True(t, mock.packages["nginx"])

	require.Len(t, mock.installCalled, 1)
}

func TestPackageResource_Run_InstallAndUninstallCycle(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = false

	p := &packageResource{
		Name:    "nginx",
		manager: mock,
	}

	ctx := t.Context()

	p.State = "installed"
	service, err := p.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service)
	require.True(t, mock.packages["nginx"])

	p.State = "absent"
	service, err = p.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service)
	require.False(t, mock.packages["nginx"])

	p.State = "installed"
	service, err = p.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service)
	require.True(t, mock.packages["nginx"])

	require.Len(t, mock.installCalled, 2)
	require.Len(t, mock.uninstallCalled, 1)
}

func TestPackageResource_Run_UninstallWithNotification(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = true

	p := &packageResource{
		Name:  "nginx",
		State: "absent",
		Notify: notifyResource{
			Service: "monitor-service",
		},
		manager: mock,
	}

	service, err := p.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "monitor-service", service)
	require.False(t, mock.packages["nginx"])
}

func TestPackageResource_Run_PackageNameVariations(t *testing.T) {
	testCases := []struct {
		name        string
		packageName string
	}{
		{"simple", "nginx"},
		{"with-dash", "nginx-extras"},
		{"with-number", "python3"},
		{"with-colon", "nginx:1.18"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockPackageManager()
			mock.packages[tc.packageName] = false

			p := &packageResource{
				Name:    tc.packageName,
				State:   "installed",
				manager: mock,
			}

			_, err := p.Run(t.Context())
			require.NoError(t, err)
			require.True(t, mock.packages[tc.packageName])
		})
	}
}

func TestPackageResource_Run_ConcurrentCalls(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = false

	p := &packageResource{
		Name:    "nginx",
		State:   "installed",
		manager: mock,
	}

	ctx := t.Context()

	for range 5 {
		_, err := p.Run(ctx)
		require.NoError(t, err)
	}

	require.True(t, mock.packages["nginx"])
}

func TestPackageResource_Run_NoNotifyOnNoChange(t *testing.T) {
	mock := newMockPackageManager()
	mock.packages["nginx"] = true

	p := &packageResource{
		Name:  "nginx",
		State: "installed",
		Notify: notifyResource{
			Service: "should-not-notify",
		},
		manager: mock,
	}

	service, err := p.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)
}

func TestPackageResource_Run_StateTransitions(t *testing.T) {
	testCases := []struct {
		name          string
		initialState  bool
		desiredState  string
		expectChange  bool
		expectInstall bool
		expectNotify  bool
	}{
		{"absent_to_installed", false, "installed", true, true, true},
		{"installed_to_absent", true, "absent", true, false, true},
		{"installed_to_installed", true, "installed", false, true, false},
		{"absent_to_absent", false, "absent", false, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockPackageManager()
			mock.packages["test"] = tc.initialState

			p := &packageResource{
				Name:  "test",
				State: tc.desiredState,
				Notify: notifyResource{
					Service: "notify-service",
				},
				manager: mock,
			}

			service, err := p.Run(t.Context())
			require.NoError(t, err)

			require.Equal(t, tc.expectInstall, mock.packages["test"])

			if tc.expectNotify {
				require.Equal(t, "notify-service", service)
			} else {
				require.Empty(t, service)
			}
		})
	}
}
