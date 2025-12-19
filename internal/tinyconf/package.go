package tinyconf

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
)

// TODO: support version
type packageResource struct {
	Name    string         `json:"name" validate:"required"`
	State   string         `json:"state" validate:"required,oneof=installed absent"`
	Notify  notifyResource `json:"notify"`
	manager packageManager
}

type packageManager interface {
	IsInstalled(context.Context, string) (bool, error)
	Install(context.Context, string) error
	Uninstall(context.Context, string) error
}

func (s *packageResource) Run(ctx context.Context) (string, error) {
	if isNil(s.manager) {
		s.manager = &aptPackageManager{}
	}

	tasks := []func() (bool, error){
		func() (bool, error) {
			isInstalled, err := s.manager.IsInstalled(ctx, s.Name)
			if err != nil {
				return false, fmt.Errorf("failed to get status for %s %w", s.Name, err)
			}

			switch s.State {
			case "installed":
				if isInstalled {
					return false, nil
				}

				slog.Info("installing package", "name", s.Name)
				return true, s.manager.Install(ctx, s.Name)
			case "absent":
				if !isInstalled {
					return false, nil
				}
				slog.Info("uninstalling package", "name", s.Name)
				return true, s.manager.Uninstall(ctx, s.Name)
			default:
				// validation should catch this, but in case
				return false, fmt.Errorf("unexpected package state %s", s.State)
			}
		},
	}

	// use runTasks in case we add some debugging/logging/etc
	changed, err := runTasks(tasks)
	if err != nil {
		return "", err
	}

	if changed {
		return s.Notify.Service, nil
	}

	return "", nil
}

type aptPackageManager struct{}

func (a *aptPackageManager) IsInstalled(ctx context.Context, packageName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "dpkg", "-s", packageName)
	if err := cmd.Run(); err != nil {
		// dpkg -s returns non-zero exit code when package is not installed
		if exitErr, ok := err.(*exec.ExitError); ok {
			// exit code 1 means package not installed
			if exitErr.ExitCode() == 1 {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check package %s status: %w", packageName, err)
	}

	return true, nil
}

func (a *aptPackageManager) Install(ctx context.Context, packageName string) error {
	// TODO: have an option whether to run update or not? for now
	// always run it when we have to install something
	// could check timestamp of /var/lib/apt/periodic/update-success-stamp

	updateCmd := exec.CommandContext(ctx, "apt", "update")
	updateCmd.Env = []string{"DEBIAN_FRONTEND=noninteractive"}
	// for now, we won't consider this fatal
	_ = updateCmd.Run()

	cmd := exec.CommandContext(ctx, "apt", "install", "-y", packageName)
	cmd.Env = []string{"DEBIAN_FRONTEND=noninteractive"}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install package %s (output: %s): %w", packageName, string(output), err)
	}

	return nil
}

func (a *aptPackageManager) Uninstall(ctx context.Context, packageName string) error {
	// purge??
	cmd := exec.CommandContext(ctx, "apt", "remove", "-y", packageName)
	cmd.Env = []string{"DEBIAN_FRONTEND=noninteractive"}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to uninstall package %s (output: %s): %w", packageName, string(output), err)
	}

	return nil
}
