# Kymactl

## Overview

A command line tool to support developers of and with Kyma

## Available Commands

- `version`: Shows the kyma cluster version and the kymactl version. The kymactl version is set at compile time passing it to the go linker as a flag:

    ```bash
    go build -o kymactl -ldflags "-X github.com/kyma-incubator/kymactl/pkg/kymactl/cmd.Version=1.5.0"
    ```
- `install cluster minikube`: Initializes minikube with a new cluster (replaces the `minikube.sh` script) 
- `install kyma`: Installs kyma to a cluster based on a release (replaces the `ìnstaller.sh` and `is-installed.sh` script)
- `uninstall kyma`: Uninstalls all kyma related resources from a cluster
- `completion`: Output shell completion code for bash.
- `help`: Displays usage for the given command (e.g. `kymactl help`, `kymactl help status`, etc...)

## kymactl as a kubectl plugin

To follow this section a kubectl version of 1.12.0 or later is required.

A plugin is nothing more than a standalone executable file, whose name begins with kubectl- . To install a plugin, simply move this executable file to anywhere on your PATH.

Rename a `kymactl` binary to `kubectl-kyma` and place it anywhere in your PATH:

```bash
sudo mv ./kymactl /usr/local/bin/kubectl-kyma
```

Run `kubectl plugin list` command and you will see your plugin in the list of available plugins.

You may now invoke your plugin as a kubectl command:

```bash
$ kubectl kyma status
Kyma is running!
```

To know more about extending kubectl with plugins read [kubernetes documentation](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/).

## Roadmap
- fix adding minikube domain to /etc/hosts
- use latest release automically
- remove orphaned minikube domain entries from /etc/hosts
- install optional kyma module
- uninstall optional kyma module
- update kyma to newer release
- list available releases
- install gke cluster
- execute acceptance tests against kyma cluster
  