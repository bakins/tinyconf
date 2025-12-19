package tinyconf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
)

type fileResource struct {
	Path     string         `json:"path" validate:"required"`
	Contents *string        `json:"contents"`
	Owner    *string        `json:"owner"`
	Group    *string        `json:"group"`
	Mode     *os.FileMode   `json:"mode"`
	State    *string        `json:"state" validate:"omitempty,oneof=present absent"`
	Notify   notifyResource `json:"notify"`
}

const defaultFileMode = os.FileMode(0o644)

func (f *fileResource) Run(ctx context.Context) (string, error) {
	userID, groupID, err := getUserAndGroup(f.Owner, f.Group)
	if err != nil {
		return "", err
	}

	var tasks []func() (bool, error)

	shouldExist := true
	if f.State != nil && *f.State == "absent" {
		shouldExist = false
	}

	// the if/else's are a bit gnarly here
	// should cllean this up and maybe create smaller helper functions
	fileInfo, err := os.Stat(f.Path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to stat %s %w", f.Path, err)
		} else {
			if !shouldExist {
				return "", nil
			}
		}

		mode := os.FileMode(defaultFileMode)
		if f.Mode != nil {
			mode = *f.Mode
		}

		tasks = append(
			tasks,
			func() (bool, error) {
				var contents string
				if f.Contents != nil {
					contents = *f.Contents
				}
				slog.Info("creating file", "path", f.Path, "mode", mode)
				return true, os.WriteFile(f.Path, []byte(contents), mode)
			},
			// we could/should do group at same time but
			// this makes it a little easier at the expense of an additonal call
			func() (bool, error) {
				if userID == -1 {
					return false, nil
				}
				slog.Info("changing file owner", "path", f.Path, "uid", userID)
				return true, os.Chown(f.Path, userID, -1)
			},
			func() (bool, error) {
				if groupID == -1 {
					return false, nil
				}
				slog.Info("changing file group", "path", f.Path, "gid", groupID)
				return true, os.Chown(f.Path, -1, groupID)
			},
		)
	} else {

		if fileInfo.IsDir() {
			return "", fmt.Errorf("%s is a directory", f.Path)
		}

		if !shouldExist {
			tasks = append(
				tasks,
				func() (bool, error) {
					slog.Info("removing file", "path", f.Path)
					return true, os.Remove(f.Path)
				},
			)
		} else {

			sysStat, ok := fileInfo.Sys().(*syscall.Stat_t)
			if !ok || sysStat == nil {
				return "", fmt.Errorf("unexpected file info returns by stat for %s", f.Path)
			}

			tasks = append(
				tasks,
				func() (bool, error) {
					if userID == -1 || sysStat.Uid == uint32(userID) {
						return false, nil
					}
					slog.Info("changing file owner", "path", f.Path, "uid", userID)
					return true, os.Chown(f.Path, userID, -1)
				},
				func() (bool, error) {
					if groupID == -1 || sysStat.Gid == uint32(groupID) {
						return false, nil
					}
					slog.Info("changing file group", "path", f.Path, "gid", groupID)
					return true, os.Chown(f.Path, -1, groupID)
				},
				func() (bool, error) {
					if f.Mode == nil || fileInfo.Mode().Perm() == f.Mode.Perm() {
						return false, nil
					}

					slog.Info("changing file mode", "path", f.Path, "mode", *f.Mode)
					return true, os.Chmod(f.Path, *f.Mode)
				},
				func() (bool, error) {
					if f.Contents == nil {
						return false, nil
					}

					// this should probably be a checksum
					// as this reads entire file into memory.
					// good enough for this simple string only example
					contents, err := os.ReadFile(f.Path)
					if err != nil {
						return false, fmt.Errorf("failed to read %s %w", f.Path, err)
					}

					if string(contents) == *f.Contents {
						return false, nil
					}

					// attempt to write to tempfile and move into place
					file, err := os.CreateTemp(filepath.Dir(f.Path), ".*.tmp")
					if err != nil {
						return false, fmt.Errorf("failed to create temp file for %s %w", f.Path, err)
					}

					defer func() {
						_ = file.Close()
						_ = os.Remove(file.Name())
					}()

					if _, err := file.WriteString(*f.Contents); err != nil {
						return false, fmt.Errorf("failed to write to temp file for %s %w", f.Path, err)
					}

					if err := file.Close(); err != nil {
						return false, fmt.Errorf("failed to close temp file for %s %w", f.Path, err)
					}

					// need to reread permissions - we could be smarter about this, but brute force is fine for now
					if err := copyPermissions(f.Path, file.Name()); err != nil {
						return false, err
					}

					slog.Info("updating file contents", "path", f.Path)
					if err := os.Rename(file.Name(), f.Path); err != nil {
						return false, fmt.Errorf("failed to rename temp file for %s %w", f.Path, err)
					}

					return true, nil
				},
			)
		}
	}

	changed, err := runTasks(tasks)
	if err != nil {
		return "", err
	}

	if changed {
		return f.Notify.Service, nil
	}

	return "", nil
}
