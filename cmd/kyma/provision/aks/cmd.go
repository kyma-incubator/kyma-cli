package aks

import (
	"github.com/spf13/cobra"
)

var defaultKubernetesVersion string = "1.19.11"

func NewCmd(o *Options) *cobra.Command {
	c := newAksCmd(o)

	cmd := &cobra.Command{
		Use:   "aks",
		Short: "Provisions an Azure Kubernetes Service (AKS) cluster on Azure.",
		Long: `Use this command to provision an AKS cluster on Azure for Kyma installation. Use the flags to specify cluster details. 
	NOTE: To provision and access the provisioned cluster, make sure you get authenticated by using the Azure CLI. To do so,run ` + "`az login`" + ` and log in with your Azure credentials.`,

		RunE: func(_ *cobra.Command, _ []string) error { return c.Run() },
	}

	cmd.Flags().StringVarP(&o.Name, "name", "n", "", "Name of the AKS cluster to provision. (required)")
	cmd.Flags().StringVarP(&o.Project, "project", "p", "", "Name of the Azure Resource Group where you provision the AKS cluster. (required)")
	cmd.Flags().StringVarP(&o.CredentialsFile, "credentials", "c", "", "Path to the TOML file containing the Azure Subscription ID (SUBSCRIPTION_ID), Tenant ID (TENANT_ID), Client ID (CLIENT_ID) and Client Secret (CLIENT_SECRET). (required)")
	cmd.Flags().StringVarP(&o.KubernetesVersion, "kube-version", "k", defaultKubernetesVersion, "Kubernetes version of the cluster.")
	cmd.Flags().StringVarP(&o.Location, "location", "l", "westeurope", "Region (e.g. westeurope) of the cluster.")
	cmd.Flags().StringVarP(&o.MachineType, "type", "t", "Standard_D4_v3", "Machine type used for the cluster.")
	cmd.Flags().IntVar(&o.DiskSizeGB, "disk-size", 50, "Disk size (in GB) of the cluster.")
	cmd.Flags().IntVar(&o.NodeCount, "nodes", 3, "Number of cluster nodes.")
	// Temporary disabled flag. To be enabled when hydroform supports TF modules
	//cmd.Flags().StringSliceVarP(&o.Extra, "extra", "e", nil, "Provide one or more arguments of the form NAME=VALUE to add extra configurations.")
	cmd.Flags().UintVar(&o.Attempts, "attempts", 3, "Maximum number of attempts to provision the cluster.")

	return cmd
}
