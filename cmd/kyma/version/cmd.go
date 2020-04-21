package version

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyma-project/cli/internal/kube"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"
)

//Version contains the cli binary version injected by the build system
var Version string

type command struct {
	opts *Options
}

//NewVersionCmd creates a new version command
func NewCmd(o *Options) *cobra.Command {
	c := &command{
		opts: o,
	}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Displays the version of Kyma CLI and the connected Kyma cluster.",
		Long: `Use this command to print the version of Kyma CLI and the version of the Kyma cluster the current kubeconfig points to.
`,
		RunE: func(_ *cobra.Command, _ []string) error { return c.Run() },
	}
	cmd.Flags().BoolVarP(&o.Client, "client", "c", false, "Client version only (no server required)")
	return cmd
}

//Run runs the command
func (c command) Run() error {
	version := Version
	if version == "" {
		version = "N/A"
	}
	fmt.Printf("Kyma CLI version: %s\n", version)

	if !c.opts.Client {
		k8s, err := kube.NewFromConfigWithTimeout("", c.opts.KubeconfigPath, 2*time.Second)
		if err != nil {
			return errors.Wrap(err, "Could not initialize the Kubernetes client. Make sure your kubeconfig is valid")
		}

		version, err := KymaVersion(c.opts.Verbose, k8s)
		if err != nil {
			fmt.Printf("Unable to get Kyma cluster version due to error: %s. Check if your cluster is available and has Kyma installed\r\n", err.Error())
			return nil
		}
		fmt.Printf("Kyma cluster version: %s\n", version)
	}

	return nil
}

//KymaVersion determines the version of kyma installed in the cluster sccessible via the provided kubernetes client
func KymaVersion(verbose bool, k8s kube.KymaKube) (string, error) {
	pods, err := k8s.Static().CoreV1().Pods("kyma-installer").List(metav1.ListOptions{LabelSelector: "name=kyma-installer"})
	if err != nil {
		return "", err
	}

	if len(pods.Items) == 0 {
		return "N/A", nil
	}

	imageParts := strings.Split(pods.Items[0].Spec.Containers[0].Image, ":")
	if len(imageParts) < 2 {
		return "N/A", nil
	}

	return imageParts[1], nil
}
