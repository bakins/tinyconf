package tinyconf

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigFromBytes_ValidFileResource(t *testing.T) {
	yaml := `
resources:
  - type: file
    path: /etc/nginx/nginx.conf
    contents: "server { listen 80; }"
    mode: 0644
`
	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Resources, 1)
	require.Equal(t, "file", cfg.Resources[0].Type)
	require.NotNil(t, cfg.Resources[0].File)
	require.Equal(t, "/etc/nginx/nginx.conf", cfg.Resources[0].File.Path)
	require.NotNil(t, cfg.Resources[0].File.Contents)
	require.Equal(t, "server { listen 80; }", *cfg.Resources[0].File.Contents)
}

func TestConfigFromBytes_ValidDirectoryResource(t *testing.T) {
	yaml := `
resources:
  - type: directory
    path: /var/www/html
    mode: 0755
    recursive: true
`
	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Resources, 1)
	require.Equal(t, "directory", cfg.Resources[0].Type)
	require.NotNil(t, cfg.Resources[0].Directory)
	require.Equal(t, "/var/www/html", cfg.Resources[0].Directory.Path)
	require.True(t, cfg.Resources[0].Directory.Recursive)
}

func TestConfigFromBytes_ValidServiceResource(t *testing.T) {
	yaml := `
resources:
  - type: service
    name: nginx
    state: running
`

	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Resources, 1)
	require.Equal(t, "service", cfg.Resources[0].Type)
	require.NotNil(t, cfg.Resources[0].Service)
	require.Equal(t, "nginx", cfg.Resources[0].Service.Name)
	require.Equal(t, "running", cfg.Resources[0].Service.State)
}

func TestConfigFromBytes_MixedResources(t *testing.T) {
	yaml := `
resources:
  - type: file
    path: /etc/nginx/nginx.conf
    contents: "config"
  - type: directory
    path: /var/www
    mode: 0755
  - type: service
    name: nginx
    state: running
`

	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Resources, 3)

	require.Equal(t, "file", cfg.Resources[0].Type)
	require.Equal(t, "directory", cfg.Resources[1].Type)
	require.Equal(t, "service", cfg.Resources[2].Type)
}

func TestConfigFromBytes_WithNotify(t *testing.T) {
	yaml := `
resources:
  - type: file
    path: /etc/nginx/nginx.conf
    contents: "config"
    notify:
      service: nginx
`

	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Resources, 1)
	require.Equal(t, "nginx", cfg.Resources[0].File.Notify.Service)
}

func TestConfigFromBytes_FileWithOwnerAndGroup(t *testing.T) {
	yaml := `
resources:
  - type: file
    path: /tmp/test.txt
    owner: nobody
    group: nogroup
    mode: 0600
`

	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Resources[0].File.Owner)
	require.Equal(t, "nobody", *cfg.Resources[0].File.Owner)
	require.NotNil(t, cfg.Resources[0].File.Group)
	require.Equal(t, "nogroup", *cfg.Resources[0].File.Group)
}

func TestConfigFromBytes_EmptyConfig(t *testing.T) {
	yaml := `
resources: []
`

	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Empty(t, cfg.Resources)
}

func TestConfigFromBytes_InvalidYAML(t *testing.T) {
	yaml := `
resources:
  - type: file
    path: /test
	this is not valid yaml
`

	_, err := configFromBytes([]byte(yaml))
	require.Error(t, err)
}

func TestConfigFromBytes_MissingType(t *testing.T) {
	yaml := `
resources:
  - path: /etc/test
`

	_, err := configFromBytes([]byte(yaml))
	require.Error(t, err)
}

func TestConfigFromBytes_InvalidType(t *testing.T) {
	yaml := `
resources:
  - type: invalid
    path: /test
`

	_, err := configFromBytes([]byte(yaml))
	require.Error(t, err)
}

func TestConfigFromBytes_MissingRequiredField(t *testing.T) {
	yaml := `
resources:
  - type: file
`

	_, err := configFromBytes([]byte(yaml))
	require.Error(t, err)
}

func TestConfigFromBytes_ServiceInvalidState(t *testing.T) {
	yaml := `
resources:
  - type: service
    name: nginx
    state: invalid
`

	_, err := configFromBytes([]byte(yaml))
	require.Error(t, err)
}

func TestConfigFromBytes_ServiceStoppedState(t *testing.T) {
	yaml := `
resources:
  - type: service
    name: nginx
    state: stopped
`

	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.Equal(t, "stopped", cfg.Resources[0].Service.State)
}

func TestConfigFromBytes_ComplexConfig(t *testing.T) {
	yaml := `
resources:
  - type: directory
    path: /etc/nginx/conf.d
    mode: 0755
    recursive: true

  - type: file
    path: /etc/nginx/nginx.conf
    contents: |
      user www-data;
      worker_processes auto;
    mode: 0644
    notify:
      service: nginx

  - type: file
    path: /etc/nginx/conf.d/default.conf
    contents: |
      server {
        listen 80;
        server_name localhost;
      }
    mode: 0644
    notify:
      service: nginx

  - type: service
    name: nginx
    state: running
`

	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Resources, 4)

	require.Equal(t, "directory", cfg.Resources[0].Type)
	require.Equal(t, "file", cfg.Resources[1].Type)
	require.Equal(t, "file", cfg.Resources[2].Type)
	require.Equal(t, "service", cfg.Resources[3].Type)
}

func TestConfigFromBytes_ToRunner(t *testing.T) {
	yaml := `
resources:
  - type: file
    path: /tmp/test.txt
  - type: directory
    path: /tmp/testdir
  - type: service
    name: test
    state: running
`

	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.Len(t, cfg.Resources, 3)

	for i, res := range cfg.Resources {
		runner, err := res.toRunner()
		require.NoError(t, err, "resource %d should convert to runner", i)
		require.NotNil(t, runner, "runner %d should not be nil", i)
	}
}

// TestConfigFromBytes_MultilineContents tests parsing file with multiline contents
func TestConfigFromBytes_MultilineContents(t *testing.T) {
	yaml := `
resources:
  - type: file
    path: /etc/test.conf
    contents: |
      line 1
      line 2
      line 3
`

	cfg, err := configFromBytes([]byte(yaml))
	require.NoError(t, err)
	require.NotNil(t, cfg.Resources[0].File.Contents)
	require.Contains(t, *cfg.Resources[0].File.Contents, "line 1")
	require.Contains(t, *cfg.Resources[0].File.Contents, "line 2")
	require.Contains(t, *cfg.Resources[0].File.Contents, "line 3")
}

func TestConfigFromFile(t *testing.T) {
	yaml := `
resources:
  - type: file
    path: /tmp/test.txt
    contents: "hello"
`

	tmpfile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	require.NoError(t, err)
	defer tmpfile.Close()

	_, err = tmpfile.Write([]byte(yaml))
	require.NoError(t, err)

	cfg, err := configFromFile(tmpfile.Name())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Len(t, cfg.Resources, 1)
	require.Equal(t, "file", cfg.Resources[0].Type)
}

func TestConfigFromFile_NonExistent(t *testing.T) {
	_, err := configFromFile("/nonexistent/path/to/file.yaml")
	require.Error(t, err)
}

func TestConfigFromFile_InvalidYAML(t *testing.T) {
	tmpfile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	require.NoError(t, err)
	defer tmpfile.Close()

	_, err = tmpfile.Write([]byte("not: [valid yaml"))
	require.NoError(t, err)

	_, err = configFromFile(tmpfile.Name())
	require.Error(t, err)
}
