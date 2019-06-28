package cmd

import (
	"github.com/kyma-project/cli/pkg/kyma/cmd/install"
	"github.com/kyma-project/cli/pkg/kyma/cmd/provision/minikube"
	"github.com/kyma-project/cli/pkg/kyma/cmd/uninstall"
	"github.com/kyma-project/cli/pkg/kyma/cmd/version"

	"github.com/kyma-project/cli/pkg/kyma/cmd/provision"
	"github.com/kyma-project/cli/pkg/kyma/core"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

//NewKymaCmd creates a new kyma CLI command
func NewKymaCmd(o *core.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kyma",
		Short: "Controls a Kyma cluster.",
		Long: `Kyma is a flexible and easy way to connect and extend enterprise applications in a cloud-native world.
kyma CLI controls a Kyma cluster.

Find more information at: https://github.com/kyma-project/cli
`,
		// Affects children as well
		SilenceErrors: false,
		SilenceUsage:  true,
	}

	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "verbose output")
	cmd.PersistentFlags().BoolVar(&o.NonInteractive, "non-interactive", false, "Do not use spinners")
	cmd.PersistentFlags().StringVar(&o.KubeconfigPath, "kubeconfig", clientcmd.RecommendedHomeFile, "Path to kubeconfig")

	provisionCmd := provision.NewCmd()
	provisionCmd.AddCommand(minikube.NewCmd(minikube.NewOptions(o)))

	cmd.AddCommand(
		version.NewCmd(version.NewOptions(o)),
		NewCompletionCmd(),
		install.NewCmd(install.NewOptions(o)),
		uninstall.NewCmd(uninstall.NewOptions(o)),
		provisionCmd,
	)

	return cmd
}
