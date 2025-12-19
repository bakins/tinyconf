package tinyconf

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

type serviceNotifier interface {
	Restart(context.Context, string) error
}

// this is not idempotent - caller should dedup services
func notifyServices(ctx context.Context, notifier serviceNotifier, services []string) error {
	if isNil(notifier) {
		notifier = &systemdServiceManager{}
	}

	for _, service := range services {
		slog.Info("restarting service", "name", service)
		if err := notifier.Restart(ctx, service); err != nil {
			return err
		}
	}

	return nil
}

type serviceResource struct {
	Name    string         `json:"name" validate:"required"`
	State   string         `json:"state" validate:"required,oneof=running stopped"`
	Notify  notifyResource `json:"notify"`
	manager serviceManager
}

// for testing
type serviceManager interface {
	IsRunning(context.Context, string) (bool, error)
	Start(context.Context, string) error
	Stop(context.Context, string) error
}

func (s *serviceResource) Run(ctx context.Context) (string, error) {
	if isNil(s.manager) {
		s.manager = &systemdServiceManager{}
	}

	tasks := []func() (bool, error){
		func() (bool, error) {
			isRunning, err := s.manager.IsRunning(ctx, s.Name)
			if err != nil {
				return false, fmt.Errorf("failed to get status for %s %w", s.Name, err)
			}

			switch s.State {
			case "running":
				if isRunning {
					return false, nil
				}

				slog.Info("starting service", "name", s.Name)
				return true, s.manager.Start(ctx, s.Name)
			case "stopped":
				if !isRunning {
					return false, nil
				}

				slog.Info("stopping service", "name", s.Name)
				return true, s.manager.Stop(ctx, s.Name)
			default:
				// validation should catch this, but in case
				return false, fmt.Errorf("unexpected service state %s", s.State)
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

type systemdServiceManager struct{}

func (s *systemdServiceManager) IsRunning(ctx context.Context, service string) (bool, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", service)
	output, err := cmd.Output()
	if err != nil {
		// systemctl is-active returns non-zero exit code when service is not active
		// Check if it's just inactive vs an actual error
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Exit code 3 means inactive/stopped, which is expected
			if exitErr.ExitCode() == 3 {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check service %s status: %w", service, err)
	}

	// Output is "active" when running, "inactive" when stopped
	state := strings.TrimSpace(string(output))
	return state == "active", nil
}

func (s *systemdServiceManager) Start(ctx context.Context, service string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "start", service)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start service %s: (output: %s) %w", service, string(output), err)
	}
	return nil
}

func (s *systemdServiceManager) Stop(ctx context.Context, service string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "stop", service)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop service %s: (output: %s) %w", service, string(output), err)
	}
	return nil
}

func (s *systemdServiceManager) Restart(ctx context.Context, service string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "restart", service)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart service %s: (output: %s) %w", service, string(output), err)
	}
	return nil
}
