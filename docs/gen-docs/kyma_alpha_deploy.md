---
title: kyma alpha deploy
---

Deploys Kyma on a running Kubernetes cluster.

## Synopsis

Use this command to deploy Kyma on a running Kubernetes cluster.

```bash
kyma alpha deploy [flags]
```

## Options

```bash
      --cancel-timeout duration   Time after which the workers' context is canceled. Pending worker goroutines (if any) may continue if blocked by a Helm client. (default 15m0s)
  -c, --components string         Path to the components file. (default "workspace/installation/resources/components.yaml")
  -d, --domain string             Domain used for installation. (default "local.kyma.dev")
      --helm-timeout duration     Timeout for the underlying Helm client. (default 6m0s)
  -p, --profile string            Kyma deployment profile. Supported profiles are: "evaluation", "production".
      --quit-timeout duration     Time after which the deployment is aborted. Worker goroutines may still be working in the background. This value must be greater than the value for cancel-timeout. (default 20m0s)
  -s, --source string             Installation source.
                                  	- To use a specific release, write "kyma alpha deploy --source=1.17.1".
                                  	- To use the master branch, write "kyma alpha deploy --source=master".
                                  	- To use a commit, write "kyma alpha deploy --source=34edf09a".
                                  	- To use a pull request, write "kyma alpha deploy --source=PR-9486".
                                  	- To use the local sources, write "kyma alpha deploy --source=local". (default "master")
      --tls-crt string            TLS certificate file for the domain used for installation.
      --tls-key string            TLS key file for the domain used for installation.
      --value strings             Set a configuration value (e.g. --value component.key='the value').
  -f, --values-file string        Path to a JSON or YAML file with configuration values.
      --workers-count int         Number of parallel workers used for the deployment. (default 4)
  -w, --workspace string          Path used to download Kyma sources. (default "workspace")
```

## Options inherited from parent commands

```bash
      --ci                  Enables the CI mode to run on CI/CD systems. It avoids any user interaction (such as no dialog prompts) and ensures that logs are formatted properly in log files (such as no spinners for CLI steps).
  -h, --help                Displays help for the command.
      --kubeconfig string   Specifies the path to the kubeconfig file. By default, Kyma CLI uses the KUBECONFIG environment variable or "/$HOME/.kube/config" if the variable is not set.
      --non-interactive     Enables the non-interactive shell mode.
  -v, --verbose             Displays details of actions triggered by the command.
```

## See also

* [kyma alpha](#kyma-alpha-kyma-alpha)	 - Executes the commands in the alpha testing stage.

