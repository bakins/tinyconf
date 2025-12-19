package tinyconf

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"os/user"
	"reflect"
	"slices"
	"strconv"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/go-playground/validator/v10"
	"sigs.k8s.io/yaml"
)

// should be only call in main.go
func Run() {
	var cli struct {
		ConfigFile string `arg:"" type:"existingfile"`
	}

	kong.Parse(&cli)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cli.ConfigFile); err != nil {
		cancel()

		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, filename string) error {
	cfg, err := configFromFile(filename)
	if err != nil {
		return err
	}

	runners, err := cfg.getRunners()
	if err != nil {
		return err
	}

	services, err := runRunners(ctx, runners)
	if err != nil {
		return err
	}

	return notifyServices(ctx, &systemdServiceManager{}, services)
}

type config struct {
	Resources []resource `json:"resources"`
}

type resource struct {
	Type      string             `json:"type" validate:"required,oneof=file directory service"`
	File      *fileResource      `json:",inline"`
	Directory *directoryResource `json:",inline"`
	Service   *serviceResource   `json:",inline"`
	Package   *packageResource   `json:",inline"`
}

// handle all the supported types
func (r *resource) UnmarshalJSON(data []byte) error {
	var typeOnly struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &typeOnly); err != nil {
		return err
	}

	r.Type = typeOnly.Type

	switch r.Type {
	case "file":
		r.File = &fileResource{}
		return json.Unmarshal(data, r.File)
	case "directory":
		r.Directory = &directoryResource{}
		return json.Unmarshal(data, r.Directory)
	case "service":
		r.Service = &serviceResource{}
		return json.Unmarshal(data, r.Service)
	case "package":
		r.Package = &packageResource{}
		return json.Unmarshal(data, r.Package)
	default:
		// should be caught by validation...
		return fmt.Errorf("unknown resource type: %s", r.Type)
	}
}

// helper when building out the run tree
func (r *resource) toRunner() (runner, error) {
	switch r.Type {
	case "file":
		return r.File, nil
	case "directory":
		return r.Directory, nil
	case "service":
		return r.Service, nil
	case "package":
		return r.Package, nil
	default:
		return nil, fmt.Errorf("unknown resource type: %s", r.Type)
	}
}

func (cfg *config) getRunners() ([]runner, error) {
	var out []runner

	for _, r := range cfg.Resources {
		run, err := r.toRunner()
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}

	return out, nil
}

// poorly named, but it does run the runners
// returns services to notify
func runRunners(ctx context.Context, runners []runner) ([]string, error) {
	var out []string

	for _, r := range runners {
		service, err := r.Run(ctx)
		if err != nil {
			return nil, err
		}

		if service != "" {
			// we want order to somewhat matter (sure, why not)
			// otherwise we could use a map, but this is fine for now
			if !slices.Contains(out, service) {
				out = append(out, service)
			}
		}
	}

	return out, nil
}

func configFromBytes(input []byte) (*config, error) {
	var cfg config
	if err := yaml.Unmarshal(input, &cfg); err != nil {
		return nil, err
	}

	v := validator.New(validator.WithRequiredStructEnabled())
	if err := v.Struct(&cfg); err != nil {
		return nil, err
	}

	// this is a bit gross because of how the validator works
	for i, res := range cfg.Resources {
		var err error
		switch res.Type {
		case "file":
			err = v.Struct(res.File)
		case "directory":
			err = v.Struct(res.Directory)
		case "service":
			err = v.Struct(res.Service)
		}
		if err != nil {
			return nil, fmt.Errorf("resource %d validation failed: %w", i, err)
		}
	}

	return &cfg, nil
}

func configFromFile(filename string) (*config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return configFromBytes(data)
}

// for now, we only support notifying a service
// to restart. The service does not need to be defined
// as a resource. For now, we assume, for better or worse, the caller
// knows what they are doing.
type notifyResource struct {
	Service string
}

type runner interface {
	// returns the service to notify if any
	Run(ctx context.Context) (string, error)
}

func getUserAndGroup(username *string, groupname *string) (int, int, error) {
	userID := -1
	if username != nil && *username != "" {
		u, err := user.Lookup(*username)
		if err != nil {
			return 0, 0, fmt.Errorf("unble to determine uid for user %s %w", *username, err)
		}
		id, err := strconv.Atoi(u.Uid)
		// should never happen, but just in case
		if err != nil {
			return 0, 0, fmt.Errorf("unexpected uid for user %s %s %w", *username, u.Uid, err)
		}
		userID = id
	}

	groupID := -1
	if groupname != nil && *groupname != "" {
		g, err := user.LookupGroup(*groupname)
		if err != nil {
			return 0, 0, fmt.Errorf("unble to determine gid for group %s %w", *groupname, err)
		}
		id, err := strconv.Atoi(g.Gid)
		// should never happen, but just in case
		if err != nil {
			return 0, 0, fmt.Errorf("unexpected gid for group %s %s %w", *groupname, g.Gid, err)
		}
		groupID = id
	}

	return userID, groupID, nil
}

func runTasks(tasks []func() (bool, error)) (bool, error) {
	changed := false
	for _, task := range tasks {
		c, err := task()
		if c {
			changed = c
		}
		if err != nil {
			return changed, err
		}
	}

	return changed, nil
}

func copyPermissions(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	stat, ok := srcInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("operating system does not support extracting UID/GID in this manner")
	}
	uid := int(stat.Uid)
	gid := int(stat.Gid)

	if err := os.Chown(dst, uid, gid); err != nil {
		return fmt.Errorf("failed to change ownership of destination file %s %w", dst, err)
	}

	if err := os.Chmod(dst, os.FileMode(stat.Mode)); err != nil {
		return fmt.Errorf("failed to chmod of %s %w", dst, err)
	}

	return nil
}

// helper that tries to better check for nil interfaces
// i always have to look this up
func isNil(i any) bool {
	if i == nil {
		return true
	}

	v := reflect.ValueOf(i)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Chan, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}
