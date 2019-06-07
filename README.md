# Kyma CLI

## Overview

Kyma CLI is a command line tool which supports [Kyma](https://github.com/kyma-project/kyma) developers. It provides a set of commands you can use to install Kyma. 

## Prerequisites

Kyma CLI requires the following software:
- [kubectl](https://github.com/kubernetes/kubectl) 
- [Minikube](https://github.com/kubernetes/minikube) 

## Installation

For the installation instructions, see the [release page](https://github.com/kyma-project/cli/releases).

## Usage

### Commands

Kyma CLI comes with a set of commands:

- `version` shows the Kyma cluster version and the Kyma CLI version.
- `provision minikube` initializes Minikube on a new cluster. It replaces the `minikube.sh` script. 
- `install` installs Kyma to a cluster based on the current release. It replaces the `ìnstaller.sh` and `is-installed.sh` script. 
- `uninstall` uninstalls all Kyma-related resources from a cluster.
- `completion` generates and shows the bash completion script.
- `help` displays and explains the usage of a given command.


### Use Kyma CLI

Use the following syntax to run the commands from your terminal:

```
kyma {COMMAND} {FLAGS}
```
where:

* **{COMMAND}** specifies the operation you want to perform.
* **{FLAGS}** specify optional flags. For example, use `-v` or `--verbose` for additional information on performed operations.

Example:

```
kyma install --verbose
```

Further usage examples include:

* Install Kyma with Minikube on Mac:

    ```bash
    kyma provision minikube
    kyma install
    ```

* Install Kyma with Minikube on Windows:

    ```bash
    kyma provision minikube
    # follow instructions to add hosts
    kyma install
    ```

* Install Kyma with Minikube on Windows using HyperV:

    ```bash
    kyma provision minikube --vm-driver hyperv --hypervVirtualSwitch {YOUR_SWITCH_NAME}
    # follow instructions to add hosts
    kyma install
    ```

### Kyma CLI as a kubectl plugin

> **NOTE**: To use Kyma CLI as a kubectl plugin, use Kubernetes version 1.12.0 or higher.

A plugin is a standalone executable file with a name prefixed with `kubectl-` .To use the plugin, perform the following steps:

1. Rename the `kyma` binary to `kubectl-kyma` and place it anywhere in your **{PATH}**:

```bash
sudo mv ./kyma /usr/local/bin/kubectl-kyma
```

2. Run `kubectl plugin list` command to see your plugin on the list of all available plugins.

3. Invoke your plugin as a kubectl command:

```bash
$ kubectl kyma version
Kyma CLI version: v0.6.1
Kyma cluster version: 1.0.0
```

For more information on extending kubectl with plugins, read [Kubernetes documentation](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/).
