# tinyconf

`tinyconf` is a simple host configuration management experiment.

Inspired by [ansible](https://github.com/ansible/ansible), sort of.

## Example

Install apache and ensure it is running.

```yaml
---
resources:
  - type: package
    name: apache2
    state: installed
  - type: service
    name: apache2
    state: running
```

Note: `tinyconf` is idempotent, so actions only happen if needed.

## Status

`tinyconf` is pretty simple. It supports a small set of resources and restarting services.

Only tested on ubuntu.

Dependencies are not supported. It runs resources in the order they are in a config file - that's it.

## Architecture

The architecture is fairly simple. No dependency graph is used, just a list of resources.

It takes a list of resources form the configuration and creates a list of tasks for each resource.

### Example

We want to create a file with specific contents:

```yaml
resources:
  - type: file
    path: /tmp/myfile.txt
    owner: www-data
    mode: 0644
    contents: hello world
    notify:
      service: apache2
```

A list of tasks like the following may be produced:

- if the file doesn't exist, create it
- make sure the owner is `www-data`
- make sure the filemode is `0644`
- ensure the contents match the configuration

It will run these tasks in order and stop running if any have an error.

If any of these tasks make a change, then `apache2` will be added to the list of services
to restart

### Ordering

Resources are ran in the order they are in the `resources` list in the config file.
An error on any resource will stop the entire run.

### Service Restarts

If any resource changes and it has a `notify` filed, then that service will be added to the list
of services to restart. The service does not have to exist in the resource list - no validation is done, but this allows one to notify services not being managed by `tinyconf`.

Services are restarted in the order that notifications are sent. Services are only restarted once
per `tinyconf` run.

## Installation

The easiest way to install is to download a precompiled binary for your platform
from https://github.com/bakins/tinyconf/releases

You can also build `tinyconf` using [Go](https://go.dev/dl/) 1.25+

```shell
go build -o ./bin/tinyconf ./cmd/tinyconf
```

You can crosscompile it as well. For example, to build a linux-x86_64 binary on a Mac arm64 laptop:

```shell
GOOS=linux GOARCH=amd64 go build -o ./bin/tinyconf ./cmd/tinyconf/
```

## Usage

```bash
$ tinyconf /path/to/resources/file.yaml
```

You can see some examples in [./examples](./examples)

### Resources

All resources must be in the key `resources` in the configration file.

Each resources has a `type` field.

In general, if a field is not set on a resource, then `tinyconf` does not change that attribute.
For example, if `owner` is not set on a `file`, then `tinyconf` will not ensure the onwer is any particular user.

#### file 

Manage a file.

When a file already exists, `file` will use temporary files, and atomically rename into place to
avoid partial writes.

```yaml
- type: file
  # filesystem path of the file
  path: /path/to/file
  # file owner - username as a string
  owner: root
  # file owner - group name as a string
  group: root
  # permissions mode in the form 0644, 755, etc
  # defaults to 0644 for new files with no mode set
  mode: 0644
  # present or absent - defaults to present. when absent, ensures the file does not exist
  state: present
  # file contents as a string
  contents: |
    use some yaml, I guess
```

#### directory

Manage a single directory

```yaml
- type: directory
  # filesystem path of the directory
  path: /path/to/directory
  # directory owner - username as a string
  owner: root
  # directory owner - group name as a string
  group: root
  # permissions mode in the form 0644, 755, etc
  # defaults to 0755 for new directory with no mode set
  mode: 0644
  # if set, attempt to create parent directories. defaults to
  # only the named directory
  recursive: true
```

#### package

Install/uninstall packages.

```yaml
- type: package
  name: apache2
  # installed or absent - if absent, attempt to ensure the package is not installed
  state: installed
```

#### service

Start/stop services using systemctl.

```yaml
- type: services
  name: apache2
  # running or stopped.
  state: running
```

### Notifications

`file`, `directory`, and `package` support the `notify` directive.

```yaml
- type: file
  path: /tmp/tmp.txt
  notify:
    service: service-name
```

The only supported notification target is `service` - which tries to restart the service.

## Known Issues and Limitations

- If you start or stop a service in a run and another resource notifies it, it will be always be restarted
- `apt update` is ran every time before installing a package. A future version should hav an option for this and/or check how old the package cache is.
- `state: stopped` for a service will still fail if the service does not exist.

