package tinyconf

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirectoryResource_Run_CreateNewDirectory(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")

	d := &directoryResource{
		Path: dirPath,
	}

	service, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	require.Equal(t, defaultDirMode, info.Mode().Perm())
}

func TestDirectoryResource_Run_CreateNewDirectoryWithMode(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")
	mode := os.FileMode(0o700)

	d := &directoryResource{
		Path: dirPath,
		Mode: &mode,
	}

	_, err := d.Run(t.Context())
	require.NoError(t, err)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	require.Equal(t, mode, info.Mode().Perm())
}

func TestDirectoryResource_Run_CreateRecursively(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "parent", "child", "grandchild")

	d := &directoryResource{
		Path:      dirPath,
		Recursive: true,
	}

	service, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestDirectoryResource_Run_CreateNonRecursiveFailsWithoutParent(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "nonexistent", "child")

	d := &directoryResource{
		Path:      dirPath,
		Recursive: false,
	}

	_, err := d.Run(t.Context())
	require.Error(t, err)
}

func TestDirectoryResource_Run_CreateWithOwner(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root privileges")
	}

	dirPath := filepath.Join(t.TempDir(), "testdir")
	owner := "nobody"

	d := &directoryResource{
		Path:  dirPath,
		Owner: &owner,
	}

	_, err := d.Run(t.Context())
	require.NoError(t, err)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)

	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	u, err := user.Lookup(owner)
	require.NoError(t, err)
	expectedUID, err := strconv.Atoi(u.Uid)
	require.NoError(t, err)

	require.Equal(t, expectedUID, int(stat.Uid))
}

func TestDirectoryResource_Run_CreateWithGroup(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root privileges")
	}

	dirPath := filepath.Join(t.TempDir(), "testdir")
	group := "nobody"

	d := &directoryResource{
		Path:  dirPath,
		Group: &group,
	}

	_, err := d.Run(t.Context())
	require.NoError(t, err)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)

	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	g, err := user.LookupGroup(group)
	require.NoError(t, err)
	expectedGID, err := strconv.Atoi(g.Gid)
	require.NoError(t, err)

	require.Equal(t, expectedGID, int(stat.Gid))
}

func TestDirectoryResource_Run_UpdateExistingDirectoryMode(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")

	err := os.Mkdir(dirPath, 0o755)
	require.NoError(t, err)

	newMode := os.FileMode(0o700)
	d := &directoryResource{
		Path: dirPath,
		Mode: &newMode,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test-service", service)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestDirectoryResource_Run_NoChangesNeeded(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")
	mode := os.FileMode(0o755)

	err := os.Mkdir(dirPath, mode)
	require.NoError(t, err)

	d := &directoryResource{
		Path: dirPath,
		Mode: &mode,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)
}

func TestDirectoryResource_Run_ErrorWhenPathIsFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "testfile")
	err := os.WriteFile(filePath, []byte("test"), 0o644)
	require.NoError(t, err)

	d := &directoryResource{
		Path: filePath,
	}

	_, err = d.Run(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not a directory")
}

func TestDirectoryResource_Run_ErrorInvalidUser(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")
	invalidUser := "thisuserdoesnotexist12345"

	d := &directoryResource{
		Path:  dirPath,
		Owner: &invalidUser,
	}

	_, err := d.Run(t.Context())
	require.Error(t, err)
}

func TestDirectoryResource_Run_ErrorInvalidGroup(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")
	invalidGroup := "thisgroupdoesnotexist12345"

	d := &directoryResource{
		Path:  dirPath,
		Group: &invalidGroup,
	}

	_, err := d.Run(t.Context())
	require.Error(t, err)
}

func TestDirectoryResource_Run_MultipleUpdates(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")

	err := os.Mkdir(dirPath, 0o755)
	require.NoError(t, err)

	newMode := os.FileMode(0o700)
	d := &directoryResource{
		Path: dirPath,
		Mode: &newMode,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test-service", service)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestDirectoryResource_Run_RunMultipleTimes(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")

	d := &directoryResource{
		Path: dirPath,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	ctx := t.Context()

	service1, err := d.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, "test-service", service1)

	service2, err := d.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service2)

	service3, err := d.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service3)
}

func TestDirectoryResource_Run_EmptyOwnerAndGroup(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")

	emptyOwner := ""
	emptyGroup := ""

	d := &directoryResource{
		Path:  dirPath,
		Owner: &emptyOwner,
		Group: &emptyGroup,
	}

	_, err := d.Run(t.Context())
	require.NoError(t, err)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestDirectoryResource_Run_RecursiveWithMode(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "parent", "child", "grandchild")
	mode := os.FileMode(0o700)

	d := &directoryResource{
		Path:      dirPath,
		Mode:      &mode,
		Recursive: true,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test-service", service)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	require.Equal(t, mode, info.Mode().Perm())
}

func TestDirectoryResource_Run_UpdateOnlyMode(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")

	err := os.Mkdir(dirPath, 0o755)
	require.NoError(t, err)

	initialInfo, err := os.Stat(dirPath)
	require.NoError(t, err)

	initialStat, ok := initialInfo.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	newMode := os.FileMode(0o700)
	d := &directoryResource{
		Path: dirPath,
		Mode: &newMode,
	}

	_, err = d.Run(t.Context())
	require.NoError(t, err)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)

	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	require.Equal(t, newMode, info.Mode().Perm())
	require.Equal(t, initialStat.Uid, stat.Uid)
	require.Equal(t, initialStat.Gid, stat.Gid)
}

func TestDirectoryResource_Run_CreateWithNotification(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "testdir")

	d := &directoryResource{
		Path: dirPath,
		Notify: notifyResource{
			Service: "my-service",
		},
	}

	service, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "my-service", service)

	info, err := os.Stat(dirPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestDirectoryResource_Run_RecursiveWithExistingParent(t *testing.T) {
	parentPath := filepath.Join(t.TempDir(), "parent")
	err := os.Mkdir(parentPath, 0o755)
	require.NoError(t, err)

	childPath := filepath.Join(parentPath, "child")

	d := &directoryResource{
		Path:      childPath,
		Recursive: true,
	}

	service, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	info, err := os.Stat(childPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}
