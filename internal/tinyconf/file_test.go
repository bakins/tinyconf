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

func TestFileResource_Run_CreateNewFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")
	contents := "hello world"

	f := &fileResource{
		Path:     filePath,
		Contents: &contents,
	}

	service, err := f.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, contents, string(data))

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	require.Equal(t, defaultFileMode, info.Mode())
}

func TestFileResource_Run_CreateNewFileWithMode(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")
	contents := "hello"
	mode := os.FileMode(0o600)

	f := &fileResource{
		Path:     filePath,
		Contents: &contents,
		Mode:     &mode,
	}

	_, err := f.Run(t.Context())
	require.NoError(t, err)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	require.Equal(t, mode, info.Mode().Perm())
}

func TestFileResource_Run_CreateNewFileWithOwner(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root privileges")
	}

	filePath := filepath.Join(t.TempDir(), "test.txt")
	contents := "hello"
	owner := "nobody"

	f := &fileResource{
		Path:     filePath,
		Contents: &contents,
		Owner:    &owner,
	}

	_, err := f.Run(t.Context())
	require.NoError(t, err)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	u, err := user.Lookup(owner)
	require.NoError(t, err)
	expectedUID, err := strconv.Atoi(u.Uid)
	require.NoError(t, err)

	require.Equal(t, expectedUID, int(stat.Uid))
}

func TestFileResource_Run_CreateNewFileWithGroup(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping test that requires root privileges")
	}

	filePath := filepath.Join(t.TempDir(), "test.txt")
	contents := "hello"
	group := "nobody"

	f := &fileResource{
		Path:     filePath,
		Contents: &contents,
		Group:    &group,
	}

	_, err := f.Run(t.Context())
	require.NoError(t, err)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	g, err := user.LookupGroup(group)
	require.NoError(t, err)
	expectedGID, err := strconv.Atoi(g.Gid)
	require.NoError(t, err)

	require.Equal(t, expectedGID, int(stat.Gid))
}

func TestFileResource_Run_CreateEmptyFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "empty.txt")

	f := &fileResource{
		Path: filePath,
	}

	_, err := f.Run(t.Context())
	require.NoError(t, err)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Empty(t, data)
}

func TestFileResource_Run_UpdateExistingFileContents(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	initialContents := "initial"
	err := os.WriteFile(filePath, []byte(initialContents), 0o644)
	require.NoError(t, err)

	newContents := "updated contents"
	f := &fileResource{
		Path:     filePath,
		Contents: &newContents,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := f.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test-service", service)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, newContents, string(data))
}

func TestFileResource_Run_UpdateExistingFileMode(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	err := os.WriteFile(filePath, []byte("hello"), 0o644)
	require.NoError(t, err)

	newMode := os.FileMode(0o600)
	f := &fileResource{
		Path: filePath,
		Mode: &newMode,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := f.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test-service", service)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestFileResource_Run_NoChangesNeeded(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	contents := "hello world"
	mode := os.FileMode(0o644)

	err := os.WriteFile(filePath, []byte(contents), mode)
	require.NoError(t, err)

	f := &fileResource{
		Path:     filePath,
		Contents: &contents,
		Mode:     &mode,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := f.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)
}

func TestFileResource_Run_ErrorWhenPathIsDirectory(t *testing.T) {
	f := &fileResource{
		Path: t.TempDir(),
	}

	_, err := f.Run(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "is a directory")
}

func TestFileResource_Run_ErrorInvalidUser(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")
	invalidUser := "thisuserdoesnotexist12345"

	f := &fileResource{
		Path:  filePath,
		Owner: &invalidUser,
	}

	_, err := f.Run(t.Context())
	require.Error(t, err)
}

func TestFileResource_Run_ErrorInvalidGroup(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")
	invalidGroup := "thisgroupdoesnotexist12345"

	f := &fileResource{
		Path:  filePath,
		Group: &invalidGroup,
	}

	_, err := f.Run(t.Context())
	require.Error(t, err)
}

func TestFileResource_Run_MultipleUpdates(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	err := os.WriteFile(filePath, []byte("initial"), 0o644)
	require.NoError(t, err)

	newContents := "updated"
	newMode := os.FileMode(0o600)
	f := &fileResource{
		Path:     filePath,
		Contents: &newContents,
		Mode:     &newMode,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := f.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test-service", service)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, newContents, string(data))

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode())
}

func TestFileResource_Run_PreservesPermissionsOnContentUpdate(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	initialMode := os.FileMode(0o600)
	err := os.WriteFile(filePath, []byte("initial"), initialMode)
	require.NoError(t, err)

	initialInfo, err := os.Stat(filePath)
	require.NoError(t, err)
	initialStat := initialInfo.Sys().(*syscall.Stat_t)

	newContents := "updated"
	f := &fileResource{
		Path:     filePath,
		Contents: &newContents,
	}

	_, err = f.Run(t.Context())
	require.NoError(t, err)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	stat := info.Sys().(*syscall.Stat_t)

	require.Equal(t, initialMode, info.Mode())
	require.Equal(t, initialStat.Uid, stat.Uid)
	require.Equal(t, initialStat.Gid, stat.Gid)
}

func TestFileResource_Run_RunMultipleTimes(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	contents := "hello"
	f := &fileResource{
		Path:     filePath,
		Contents: &contents,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	ctx := t.Context()

	service1, err := f.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, "test-service", service1)

	service2, err := f.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service2)

	service3, err := f.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service3)
}

func TestFileResource_Run_EmptyOwnerAndGroup(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	emptyOwner := ""
	emptyGroup := ""

	f := &fileResource{
		Path:  filePath,
		Owner: &emptyOwner,
		Group: &emptyGroup,
	}

	_, err := f.Run(t.Context())
	require.NoError(t, err)

	stat, err := os.Stat(filePath)
	require.NoError(t, err)

	sysStat, ok := stat.Sys().(*syscall.Stat_t)
	require.True(t, ok)

	currentUser, err := user.Current()
	require.NoError(t, err)

	uid, err := strconv.Atoi(currentUser.Uid)
	require.NoError(t, err)
	require.Equal(t, uint32(uid), sysStat.Uid)
}

func TestFileResource_Run_ContentUpdateWithTempFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	err := os.WriteFile(filePath, []byte("initial"), 0o644)
	require.NoError(t, err)

	newContents := "new contents that are longer than the original"
	f := &fileResource{
		Path:     filePath,
		Contents: &newContents,
	}

	_, err = f.Run(t.Context())
	require.NoError(t, err)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, newContents, string(data))

	files, err := os.ReadDir(t.TempDir())
	require.NoError(t, err)

	for _, file := range files {
		require.NotEqual(t, ".tmp", filepath.Ext(file.Name()), "found leftover temp file: %s", file.Name())
	}
}

func TestFileResource_Run_RemoveExistingFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	err := os.WriteFile(filePath, []byte("content"), 0o644)
	require.NoError(t, err)

	absent := "absent"
	f := &fileResource{
		Path:  filePath,
		State: &absent,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := f.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, "test-service", service)

	_, err = os.Stat(filePath)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func TestFileResource_Run_RemoveNonExistentFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "nonexistent.txt")

	absent := "absent"
	f := &fileResource{
		Path:  filePath,
		State: &absent,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	service, err := f.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)
}

func TestFileResource_Run_RemoveMultipleTimes(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")

	err := os.WriteFile(filePath, []byte("content"), 0o644)
	require.NoError(t, err)

	absent := "absent"
	f := &fileResource{
		Path:  filePath,
		State: &absent,
		Notify: notifyResource{
			Service: "test-service",
		},
	}

	ctx := t.Context()

	service1, err := f.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, "test-service", service1)

	_, err = os.Stat(filePath)
	require.True(t, os.IsNotExist(err))

	service2, err := f.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service2)

	service3, err := f.Run(ctx)
	require.NoError(t, err)
	require.Empty(t, service3)
}

func TestFileResource_Run_StatePresentExplicit(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")
	contents := "hello"
	present := "present"

	f := &fileResource{
		Path:     filePath,
		Contents: &contents,
		State:    &present,
	}

	service, err := f.Run(t.Context())
	require.NoError(t, err)
	require.Empty(t, service)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, contents, string(data))
}

func TestFileResource_Run_CreateAndRemoveCycle(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "test.txt")
	contents := "content"

	f := &fileResource{
		Path:     filePath,
		Contents: &contents,
	}

	ctx := t.Context()

	_, err := f.Run(ctx)
	require.NoError(t, err)

	_, err = os.Stat(filePath)
	require.NoError(t, err)

	absent := "absent"
	f.State = &absent

	_, err = f.Run(ctx)
	require.NoError(t, err)

	_, err = os.Stat(filePath)
	require.True(t, os.IsNotExist(err))

	f.State = nil

	_, err = f.Run(ctx)
	require.NoError(t, err)

	_, err = os.Stat(filePath)
	require.NoError(t, err)
}
