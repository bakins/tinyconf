package tinyconf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"syscall"
)

type directoryResource struct {
	Path      string         `json:"path" validate:"required"`
	Owner     *string        `json:"owner"`
	Group     *string        `json:"group"`
	Mode      *os.FileMode   `json:"mode"`
	Recursive bool           `json:"recursive"`
	Notify    notifyResource `json:"notify"`
}

const defaultDirMode = os.FileMode(0o755)

func (d *directoryResource) Run(ctx context.Context) (string, error) {
	userID, groupID, err := getUserAndGroup(d.Owner, d.Group)
	if err != nil {
		return "", err
	}

	var tasks []func() (bool, error)

	dirInfo, err := os.Stat(d.Path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to stat %s %w", d.Path, err)
		}

		mode := defaultDirMode
		if d.Mode != nil {
			mode = *d.Mode
		}

		tasks = append(
			tasks,
			func() (bool, error) {
				if d.Recursive {
					slog.Info("creating directory recursively", "path", d.Path, "mode", mode)
					return true, os.MkdirAll(d.Path, mode)
				}
				slog.Info("creating directory", "path", d.Path, "mode", mode)
				return true, os.Mkdir(d.Path, mode)
			},
			func() (bool, error) {
				if userID == -1 {
					return false, nil
				}
				slog.Info("changing directory owner", "path", d.Path, "uid", userID)
				return true, os.Chown(d.Path, userID, -1)
			},
			func() (bool, error) {
				if groupID == -1 {
					return false, nil
				}
				slog.Info("changing directory group", "path", d.Path, "gid", groupID)
				return true, os.Chown(d.Path, -1, groupID)
			},
		)
	} else {
		if !dirInfo.IsDir() {
			return "", fmt.Errorf("%s is not a directory", d.Path)
		}

		sysStat, ok := dirInfo.Sys().(*syscall.Stat_t)
		if !ok || sysStat == nil {
			return "", fmt.Errorf("unexpected file info returned by stat for %s", d.Path)
		}

		tasks = append(
			tasks,
			func() (bool, error) {
				if userID == -1 || sysStat.Uid == uint32(userID) {
					return false, nil
				}
				slog.Info("changing directory owner", "path", d.Path, "uid", userID)
				return true, os.Chown(d.Path, userID, -1)
			},
			func() (bool, error) {
				if groupID == -1 || sysStat.Gid == uint32(groupID) {
					return false, nil
				}
				slog.Info("changing directory group", "path", d.Path, "gid", groupID)
				return true, os.Chown(d.Path, -1, groupID)
			},
			func() (bool, error) {
				if d.Mode == nil || dirInfo.Mode().Perm() == d.Mode.Perm() {
					return false, nil
				}
				slog.Info("changing directory mode", "path", d.Path, "mode", *d.Mode)
				return true, os.Chmod(d.Path, *d.Mode)
			},
		)
	}

	changed, err := runTasks(tasks)
	if err != nil {
		return "", err
	}

	if changed {
		return d.Notify.Service, nil
	}

	return "", nil
}
