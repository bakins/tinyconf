# tinyconf

`tinyconf` is a simple host configuration management experiment.

Inspired by [ansible](https://github.com/ansible/ansible), sort of.

##

## Known Issues and Limitations

- Currently, if you start or stop a service in a run and another resource notifies it, it will be always be restarted
