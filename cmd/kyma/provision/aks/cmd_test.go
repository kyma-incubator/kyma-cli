package aks

import (
	"testing"

	"github.com/kyma-incubator/hydroform/provision/types"
	"github.com/kyma-project/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

// TestProvisionAKSFlags ensures that the provided command flags are stored in the options.
func TestProvisionAKSFlags(t *testing.T) {
	t.Parallel()
	o := NewOptions(&cli.Options{})
	c := NewCmd(o)

	// test default flag values
	require.Equal(t, "", o.Name, "Default value for the name flag not as expected.")
	require.Equal(t, "", o.Project, "Default value for the project flag not as expected.")
	require.Equal(t, "", o.CredentialsFile, "Default value for the credentials flag not as expected.")
	require.Equal(t, defaultKubernetesVersion, o.KubernetesVersion, "Default value for the kube-version flag not as expected.")
	require.Equal(t, "westeurope", o.Location, "Default value for the location flag not as expected.")
	require.Equal(t, "Standard_D4_v3", o.MachineType, "Default value for the type flag not as expected.")
	require.Equal(t, 50, o.DiskSizeGB, "Default value for the disk-size flag not as expected.")
	require.Equal(t, 3, o.NodeCount, "Default value for the nodes flag not as expected.")
	// Temporary disable flag. To be enabled when hydroform supports TF modules
	//require.Empty(t, o.Extra, "Default value for the extra flag not as expected.")
	require.Equal(t, uint(3), o.Attempts, "Default value for the attempts flag not as expected.")

	// test passing flags
	err := c.ParseFlags([]string{
		"-n", "my-cluster",
		"-p", "my-resource-group",
		"-c", "/my/credentials/file",
		"-k", "1.19.0",
		"-l", "us-central1-c",
		"-t", "quantum-computer",
		"--disk-size", "2000",
		"--nodes", "7",
		// Temporary disable flag. To be enabled when hydroform supports TF modules
		//"--extra", "VAR1=VALUE1,VAR2=VALUE2",
		"--attempts", "2",
	})
	require.NoError(t, err, "Parsing flags should not return an error")
	require.Equal(t, "my-cluster", o.Name, "The parsed value for the name flag not as expected.")
	require.Equal(t, "my-resource-group", o.Project, "The parsed value for the project flag not as expected.")
	require.Equal(t, "/my/credentials/file", o.CredentialsFile, "The parsed value for the credentials flag not as expected.")
	require.Equal(t, "1.19.0", o.KubernetesVersion, "The parsed value for the kube-version flag not as expected.")
	require.Equal(t, "us-central1-c", o.Location, "The parsed value for the location flag not as expected.")
	require.Equal(t, "quantum-computer", o.MachineType, "The parsed value for the type flag not as expected.")
	require.Equal(t, 2000, o.DiskSizeGB, "The parsed value for the disk-size flag not as expected.")
	require.Equal(t, 7, o.NodeCount, "The parsed value for the nodes flag not as expected.")
	// Temporary disable flag. To be enabled when hydroform supports TF modules
	//require.Equal(t, []string{"VAR1=VALUE1", "VAR2=VALUE2"}, o.Extra, "The parsed value for the extra flag not as expected.")
	require.Equal(t, uint(2), o.Attempts, "The parsed value for the attempts flag not as expected.")
}

func TestProvisionAKSSubcommands(t *testing.T) {
	t.Parallel()
	o := NewOptions(&cli.Options{})
	c := NewCmd(o)

	sub := c.Commands()

	require.Equal(t, 0, len(sub), "Number of provision aks subcommands not as expected")
}

func TestNewCluster(t *testing.T) {
	t.Parallel()
	o := &Options{
		Name:              "mega-cluster",
		KubernetesVersion: "1.19.0",
		Location:          "north-pole",
		MachineType:       "HAL",
		DiskSizeGB:        9000,
		NodeCount:         3,
	}
	cmd := newAksCmd(o)
	c := cmd.NewCluster()
	require.Equal(t, o.Name, c.Name, "Cluster name not as expected.")
	require.Equal(t, o.KubernetesVersion, c.KubernetesVersion, "Cluster Kubernetes version not as expected.")
	require.Equal(t, o.Location, c.Location, "Cluster location not as expected.")
	require.Equal(t, o.MachineType, c.MachineType, "Cluster machine type not as expected.")
	require.Equal(t, o.DiskSizeGB, c.DiskSizeGB, "Cluster disk size not as expected.")
	require.Equal(t, o.NodeCount, c.NodeCount, "Cluster number of nodes not as expected.")
}

func TestNewProvider(t *testing.T) {
	o := &Options{
		Project:         "cool-project",
		CredentialsFile: "/path/to/credentials",
		Extra:           []string{"VAR1=VALUE1", "VAR2=VALUE2"},
	}
	c := newAksCmd(o)
	p, err := c.NewProvider()
	require.NoError(t, err)

	require.Equal(t, types.Azure, p.Type, "Provider type not as expected.")
	require.Equal(t, o.Project, p.ProjectName, "Provider project name not as expected.")
	require.Equal(t, o.CredentialsFile, p.CredentialsFilePath, "Provider credentials file path not as expected.")
	require.Equal(t, map[string]interface{}{"VAR1": "VALUE1", "VAR2": "VALUE2"}, p.CustomConfigurations, "Provider extra configurations not as expected.")
}
