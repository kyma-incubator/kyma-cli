package install

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/kyma-project/cli/internal/nice"
	"github.com/kyma-project/cli/internal/step"

	"github.com/kyma-project/cli/internal/cmd/version"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/cli/internal/kube"

	"github.com/kyma-project/cli/internal/trust"

	"github.com/kyma-project/cli/internal/cli"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/kyma-project/cli/internal/helm"
	"github.com/kyma-project/cli/internal/minikube"
	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/storage/memory"
	yaml "gopkg.in/yaml.v2"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
)

type command struct {
	opts *Options
	cli.Command
}

type clusterInfo struct {
	isLocal       bool
	provider      string
	profile       string
	localIP       string
	localVMDriver string
}

const (
	sleep                  = 10 * time.Second
	releaseSrcUrlPattern   = "https://raw.githubusercontent.com/kyma-project/kyma/%s/%s"
	releaseResourcePattern = "https://raw.githubusercontent.com/kyma-project/kyma/%s/installation/resources/%s"
	registryImagePattern   = "eu.gcr.io/kyma-project/kyma-installer:%s"
	localDomain            = "kyma.local"
)

//NewCmd creates a new kyma command
func NewCmd(o *Options) *cobra.Command {

	cmd := command{
		Command: cli.Command{Options: o.Options},
		opts:    o,
	}

	cobraCmd := &cobra.Command{
		Use:   "install",
		Short: "Installs Kyma on a running Kubernetes cluster.",
		Long: `Use this command to install Kyma on a running Kubernetes cluster.

### Detailed description

Before you use the command, make sure your setup meets the following prerequisites:

* Kyma is not installed.
* Kubernetes cluster is available with your kubeconfig file already pointing to it.
* Helm binary is available (optional).

Here are the installation steps:

The standard installation uses the minimal configuration. The system performs the following steps:
1. Fetches the ` + "`tiller.yaml`" + ` file from the ` + "`/installation/resources`" + ` directory and deploys it to the cluster.
2. Deploys and configures the Kyma Installer. At this point, steps differ depending on the installation type.
    <div tabs name="installation">
    <details>
    <summary>
    From release
    </summary>

    When you install Kyma locally from release, the system:
    1. Fetches the latest or specified release along with configuration.
    2. Deploys the Kyma Installer on the cluster.
    3. Applies downloaded or defined configuration.
    4. Applies overrides, if applicable.
    5. Sets the admin password.
    6. Patches the Minikube IP.
    </details>
    <details>
    <summary>
    From sources
    </summary>
    
    When you install Kyma locally from sources, the system:
    1. Fetches the configuration yaml files from the local sources.
    2. Builds the Kyma Installer image.
    3. Deploys the Kyma Installer and applies the fetched configuration.
    4. Applies overrides, if applicable.
    5. Sets the admin password.
    6. Patches the Minikube IP.
    </details>
    </div>
3. Configures Helm. If installed, Helm is automatically configured using certificates from Tiller. This step is optional.
4. Runs Kyma installation until the ` + "**installed**" + ` status confirms the successful installation.
	> **NOTE**: You can override the standard installation settings using the ` + "`--override`" + ` flag.

### Usage
`,
		RunE:    func(_ *cobra.Command, _ []string) error { return cmd.Run() },
		Aliases: []string{"i"},
	}

	cobraCmd.Flags().BoolVarP(&o.NoWait, "noWait", "n", false, "Determines if the command should wait for the Kyma installation to complete.")
	cobraCmd.Flags().StringVarP(&o.Domain, "domain", "d", localDomain, "Specifies the domain used for installation.")
	cobraCmd.Flags().StringVarP(&o.TLSCert, "tlsCert", "", "", "Specifies the TLS certificate for the domain used for installation.")
	cobraCmd.Flags().StringVarP(&o.TLSKey, "tlsKey", "", "", "Specifies the TLS key for the domain used for installation.")
	cobraCmd.Flags().StringVarP(&o.Source, "source", "s", DefaultKymaVersion, "Specifies the installation source. To use the specific release, write kyma install --source=1.3.0. To use the latest master, write kyma install --source=latest. To use the local sources, write kyma install --source=local. To use the remote image, write kyma install --source=user/my-kyma-installer:v1.4.0")
	cobraCmd.Flags().StringVarP(&o.LocalSrcPath, "src-path", "", "", "Specifies the absolute path to local sources.")
	cobraCmd.Flags().DurationVarP(&o.Timeout, "timeout", "", 30*time.Minute, "Time-out after which CLI stops watching the installation progress")
	cobraCmd.Flags().StringVarP(&o.Password, "password", "p", "", "Specifies the predefined cluster password.")
	cobraCmd.Flags().VarP(&o.OverrideConfigs, "override", "o", "Specifies the path to a yaml file with parameters to override.")
	cobraCmd.Flags().Bool("help", false, "Displays help for the command.")

	return cobraCmd
}

//Run runs the command
func (cmd *command) Run() error {
	var err error
	if cmd.K8s, err = kube.NewFromConfig("", cmd.KubeconfigPath); err != nil {
		return errors.Wrap(err, "Could not initialize the Kubernetes client. Make sure your kubeconfig is valid")
	}

	s := cmd.NewStep("Validating configuration")
	if err := cmd.validateFlags(); err != nil {
		s.Failure()
		return err
	}
	s.Successf("Configuration validated")

	s = cmd.NewStep("Checking installation source")
	if cmd.opts.Local {
		s.LogInfof("Installing Kyma from local path: '%s'", cmd.opts.LocalSrcPath)
	} else {
		if cmd.opts.ReleaseVersion != cmd.opts.ConfigVersion {
			s.LogInfof("Using the installation configuration from '%s'", cmd.opts.ConfigVersion)
		}
		if cmd.opts.RemoteImage != "" {
			s.LogInfof("Installing Kyma with installer image '%s'. ", cmd.opts.RemoteImage)
		} else {
			s.LogInfof("Installing Kyma in version '%s'. ", cmd.opts.ReleaseVersion)
		}
	}
	s.Successf("Installation source checked")

	s = cmd.NewStep("Installing Tiller")
	if err := cmd.installTiller(); err != nil {
		s.Failure()
		return err
	}
	s.Successf("Tiller deployed")

	s = cmd.NewStep("Reading cluster info from ConfigMap")
	clusterConfig, err := cmd.getClusterInfoFromConfigMap()
	if err != nil {
		s.Failure()
		return err
	}
	s.Successf("Cluster info read")

	s = cmd.NewStep("Loading installation files")
	resources, err := cmd.loadAndConfigureInstallationFiles(clusterConfig)
	if err != nil {
		s.Failure()
		return err
	}
	s.Successf("Installation files loaded")

	s = cmd.NewStep("Deploying Kyma Installer")
	if err := cmd.installInstaller(resources); err != nil {
		s.Failure()
		return err
	}
	s.Successf("Kyma Installer deployed")

	if clusterConfig.isLocal {
		s = cmd.NewStep("Adding Minikube IP to the overrides")
		err := cmd.patchMinikubeIP(clusterConfig.localIP)
		if err != nil {
			s.Failure()
			return err
		}
		s.Successf("Minikube IP added")
	} else {
		if cmd.opts.Domain != localDomain {
			s = cmd.NewStep("Creating own domain ConfigMap")
			err := cmd.createOwnDomainConfigMap()
			if err != nil {
				s.Failure()
				return err
			}
			s.Successf("ConfigMap created")
		}
	}

	s = cmd.NewStep("Configuring Helm")
	if err := cmd.configureHelm(); err != nil {
		s.Failure()
		return err
	}
	s.Successf("Helm configured")

	s = cmd.NewStep("Requesting Kyma Installer to install Kyma")
	if err := cmd.activateInstaller(); err != nil {
		s.Failure()
		return err
	}
	s.Successf("Kyma Installer is installing Kyma")

	if !cmd.opts.NoWait {
		if err := cmd.waitForInstaller(); err != nil {
			return err
		}
	}
	if err := cmd.importCertificate(trust.NewCertifier(cmd.K8s)); err != nil {
		// certificate import errors do not mean installation failed
		cmd.CurrentStep.LogError(err.Error())
	}

	if clusterConfig.isLocal {
		s = cmd.NewStep("Adding domains to /etc/hosts")
		err = cmd.addDevDomainsToEtcHosts(s, clusterConfig)
		if err != nil {
			s.Failure()
			return err
		}
		s.Successf("Domains added")
	}

	if err := cmd.printSummary(); err != nil {
		return err
	}
	return nil
}

func (cmd *command) isSemVer(s string) bool {
	_, err := semver.NewVersion(s)
	return err == nil
}

func (cmd *command) isDockerImage(s string) bool {
	return len(strings.Split(s, "/")) > 1
}

func (cmd *command) validateFlags() error {
	switch {
	//Install from local sources
	case strings.ToLower(cmd.opts.Source) == "local":
		cmd.opts.Local = true
		cmd.opts.ReleaseVersion = DefaultKymaVersion
		cmd.opts.ConfigVersion = DefaultKymaVersion
		if cmd.opts.LocalSrcPath == "" {
			goPath := os.Getenv("GOPATH")
			if goPath == "" {
				return fmt.Errorf("No 'src-path' configured and no applicable default found. Check if you exported a GOPATH")
			}
			cmd.opts.LocalSrcPath = filepath.Join(goPath, "src", "github.com", "kyma-project", "kyma")
		}
		if _, err := os.Stat(cmd.opts.LocalSrcPath); err != nil {
			return fmt.Errorf("Configured 'src-path=%s' does not exist. Check if you configured a valid path", cmd.opts.LocalSrcPath)
		}
		if _, err := os.Stat(filepath.Join(cmd.opts.LocalSrcPath, "installation", "resources")); err != nil {
			return fmt.Errorf("Configured 'src-path=%s' does not seem to point to a Kyma repository. Check if your repository contains the 'installation/resources' folder.", cmd.opts.LocalSrcPath)
		}
		break

	//Install the latest version (latest master)
	case strings.ToLower(cmd.opts.Source) == "latest":
		latest, err := cmd.getMasterHash()
		if err != nil {
			return fmt.Errorf("Unable to get latest version of kyma: %s", err.Error())
		}
		cmd.opts.ReleaseVersion = fmt.Sprintf("master-%s", latest)
		cmd.opts.ConfigVersion = "master"
		cmd.opts.RegistryTemplate = registryImagePattern
		break

	//Install the specific version from release (ex: 1.3.0)
	case cmd.isSemVer(cmd.opts.Source):
		cmd.opts.ReleaseVersion = cmd.opts.Source
		cmd.opts.ConfigVersion = cmd.opts.Source
		cmd.opts.RegistryTemplate = registryImagePattern
		break

	//Install the kyma with the specific installer image (docker image URL)
	case cmd.isDockerImage(cmd.opts.Source):
		cmd.opts.RemoteImage = cmd.opts.Source
		cmd.opts.ConfigVersion = DefaultKymaVersion
		break
	default:
		return fmt.Errorf("Failed to parse the source flag. It can take one of the following: 'local', 'latest', release version (e.g. 1.4.1), or installer image")
	}

	// If one of the --domain, --tlsKey, or --tlsCert is specified, the others must be specified as well (XOR logic used below)
	if (cmd.opts.Domain != localDomain || cmd.opts.TLSKey != "" || cmd.opts.TLSCert != "") &&
		!(cmd.opts.Domain != localDomain && cmd.opts.TLSKey != "" && cmd.opts.TLSCert != "") {
		return errors.New("You specified one of the --domain, --tlsKey, or --tlsCert without specifying the others. They must be specified together")
	}

	return nil
}

func (cmd *command) installTiller() error {
	deployed, err := cmd.K8s.IsPodDeployedByLabel("kube-system", "name", "tiller")
	if err != nil {
		return err

	}
	if !deployed {
		_, err = cmd.Kubectl().RunCmd("apply", "-f", cmd.releaseSrcFile("/installation/resources/tiller.yaml"))
		if err != nil {
			return err
		}
	}
	return cmd.K8s.WaitPodStatusByLabel("kube-system", "name", "tiller", corev1.PodRunning)
}

func (cmd *command) configureHelm() error {
	helmHome, err := helm.Home()
	if err != nil {
		return err
	}

	if helmHome == "" {
		cmd.CurrentStep.LogInfof("Helm not installed")
		return nil
	}

	// Wait for the job that generates the helm secret to finish
	for {
		j, err := cmd.K8s.Static().BatchV1().Jobs("kyma-installer").Get("helm-certs-job", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if j.Status.Succeeded == 1 {
			break
		} else if j.Status.Failed == 1 {
			return errors.New("Could not generate the Helm certificate.")
		}
		time.Sleep(1 * time.Second)
	}

	secret, err := cmd.K8s.Static().CoreV1().Secrets("kyma-installer").Get("helm-secret", metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(helmHome, "ca.pem"), secret.Data["global.helm.ca.crt"], 0644)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(helmHome, "cert.pem"), secret.Data["global.helm.tls.crt"], 0644)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(helmHome, "key.pem"), secret.Data["global.helm.tls.key"], 0644)
	if err != nil {
		return err
	}
	return nil
}

func (cmd *command) installInstaller(resources []map[string]interface{}) error {
	deployed, err := cmd.K8s.IsPodDeployedByLabel("kyma-installer", "name", "kyma-installer")
	if err != nil {
		return err
	}

	if !deployed {
		_, err := cmd.Kubectl().RunApplyCmd(resources)
		if err != nil {
			return err
		}

		err = cmd.applyOverrideFiles()
		if err != nil {
			return err
		}

		err = cmd.setAdminPassword()
		if err != nil {
			return err
		}
	}
	return cmd.K8s.WaitPodStatusByLabel("kyma-installer", "name", "kyma-installer", corev1.PodRunning)
}

func (cmd *command) downloadFile(path string) (io.ReadCloser, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(path)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (cmd *command) getMasterHash() (string, error) {
	ctx, timeoutF := context.WithTimeout(context.Background(), 1*time.Minute)
	defer timeoutF()
	r, err := git.CloneContext(ctx, memory.NewStorage(), nil,
		&git.CloneOptions{
			Depth: 1,
			URL:   "https://github.com/kyma-project/kyma",
		})
	if err != nil {
		return "", err
	}
	h, err := r.Head()
	if err != nil {
		return "", err
	}

	return h.Hash().String()[:8], nil
}

func (cmd *command) buildDockerImageString(template string, version string) string {
	return fmt.Sprintf(template, version)
}

func (cmd *command) replaceDockerImageURL(resources []map[string]interface{}, imageURL string) ([]map[string]interface{}, error) {
	for _, config := range resources {
		kind, ok := config["kind"]
		if !ok {
			continue
		}

		if kind != "Deployment" {
			continue
		}

		spec, ok := config["spec"].(map[interface{}]interface{})
		if !ok {
			continue
		}

		template, ok := spec["template"].(map[interface{}]interface{})
		if !ok {
			continue
		}

		spec, ok = template["spec"].(map[interface{}]interface{})
		if !ok {
			continue
		}

		if accName, ok := spec["serviceAccountName"]; !ok {
			continue
		} else {
			if accName != "kyma-installer" {
				continue
			}
		}

		containers, ok := spec["containers"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range containers {
			container := c.(map[interface{}]interface{})
			cName, ok := container["name"]
			if !ok {
				continue
			}

			if cName != "kyma-installer-container" {
				continue
			}

			if _, ok := container["image"]; !ok {
				continue
			}
			container["image"] = imageURL
			return resources, nil
		}
	}
	return nil, errors.New("unable to find 'image' field for kyma installer 'Deployment'")
}

func (cmd *command) loadAndConfigureInstallationFiles(clusterInfo clusterInfo) ([]map[string]interface{}, error) {
	var installationFiles []string
	if clusterInfo.isLocal {
		installationFiles = []string{"installer-local.yaml", "installer-config-local.yaml.tpl", "installer-cr.yaml.tpl"}
	} else {
		installationFiles = []string{"installer.yaml", "installer-cr-cluster.yaml.tpl"}
	}

	resources, err := cmd.loadInstallationResourceFiles(installationFiles, cmd.opts.Local)
	if err != nil {
		return nil, err
	}

	err = cmd.removeActionLabel(resources)
	if err != nil {
		return nil, err
	}

	if cmd.opts.Local {
		imageName, err := cmd.findInstallerImageName(resources)
		if err != nil {
			return nil, err
		}

		err = cmd.buildKymaInstaller(imageName, clusterInfo)
		if err != nil {
			return nil, err
		}
	} else {
		if cmd.opts.RemoteImage != "" {
			resources, err = cmd.replaceDockerImageURL(resources,
				cmd.opts.RemoteImage)
			if err != nil {
				return nil, err
			}
		} else {
			resources, err = cmd.replaceDockerImageURL(resources,
				cmd.buildDockerImageString(cmd.opts.RegistryTemplate, cmd.opts.ReleaseVersion))
			if err != nil {
				return nil, err
			}
		}
	}

	return resources, nil
}

func (cmd *command) findInstallerImageName(resources []map[string]interface{}) (string, error) {
	for _, res := range resources {
		if res["kind"] == "Deployment" {
			var deployment struct {
				Metadata struct {
					Name string
				}
				Spec struct {
					Template struct {
						Spec struct {
							Containers []struct {
								Image string
							}
						}
					}
				}
			}

			err := mapstructure.Decode(res, &deployment)
			if err != nil {
				return "", err
			}

			if deployment.Metadata.Name == "kyma-installer" {
				return deployment.Spec.Template.Spec.Containers[0].Image, nil
			}
		}
	}
	return "", errors.New("'kyma-installer' deployment is missing")
}

func (cmd *command) removeActionLabel(acc []map[string]interface{}) error {
	for _, config := range acc {
		kind, ok := config["kind"]
		if !ok {
			continue
		}

		if kind != "Installation" {
			continue
		}

		meta, ok := config["metadata"].(map[interface{}]interface{})
		if !ok {
			return errors.New("Installation contains no METADATA section")
		}

		labels, ok := meta["labels"].(map[interface{}]interface{})
		if !ok {
			return errors.New("Installation contains no LABELS section")
		}

		_, ok = labels["action"].(string)
		if !ok {
			return nil
		}

		delete(labels, "action")

	}
	return nil
}

func (cmd *command) loadInstallationResourceFiles(resourcePaths []string, fromLocalSources bool) ([]map[string]interface{}, error) {

	var err error
	resources := make([]map[string]interface{}, 0)

	for _, resourcePath := range resourcePaths {

		var yamlReader io.ReadCloser

		if !fromLocalSources {
			yamlReader, err = cmd.downloadFile(cmd.releaseFile(resourcePath))
			if err != nil {
				return nil, err
			}
		} else {
			path := filepath.Join(cmd.opts.LocalSrcPath, "installation",
				"resources", resourcePath)
			yamlReader, err = os.Open(path)
			if err != nil {
				return nil, err
			}
		}

		dec := yaml.NewDecoder(yamlReader)
		for {
			m := make(map[string]interface{})
			err := dec.Decode(m)
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}
			resources = append(resources, m)
		}

		yamlReader.Close()
	}

	return resources, nil
}

func (cmd *command) buildKymaInstaller(imageName string, clusterInfo clusterInfo) error {
	dc, err := minikube.DockerClient(cmd.opts.Verbose, clusterInfo.profile)
	if err != nil {
		return err
	}

	var args []docker.BuildArg
	return dc.BuildImage(docker.BuildImageOptions{
		Name:         strings.TrimSpace(string(imageName)),
		Dockerfile:   filepath.Join("tools", "kyma-installer", "kyma.Dockerfile"),
		OutputStream: ioutil.Discard,
		ContextDir:   filepath.Join(cmd.opts.LocalSrcPath),
		BuildArgs:    args,
	})
}

func (cmd *command) activateInstaller() error {
	status, err := cmd.Kubectl().RunCmd("get", "installation/kyma-installation", "-o", "jsonpath='{.status.state}'")
	if err != nil {
		return err
	}
	if status == "InProgress" {
		return nil
	}

	_, err = cmd.Kubectl().RunCmd("label", "installation/kyma-installation", "action=install")
	if err != nil {
		return err
	}
	return nil
}

func (cmd *command) applyOverrideFiles() error {
	oFiles := cmd.opts.OverrideConfigs.Len()
	if oFiles == 0 {
		return nil
	}

	for _, file := range cmd.opts.OverrideConfigs {
		oFile, err := os.Open(file)
		if err != nil {
			fmt.Printf("unable to open file: %s. error: %s\n",
				file, err.Error())
			continue
		}
		rawData, err := ioutil.ReadAll(oFile)
		if err != nil {
			fmt.Printf("unable to read data from file: %s. error: %s\n",
				file, err.Error())
			continue
		}

		configs := strings.Split(string(rawData), "---")

		for _, c := range configs {
			if c == "" {
				continue
			}

			cfg := make(map[interface{}]interface{})
			err = yaml.Unmarshal([]byte(c), &cfg)
			if err != nil {
				fmt.Printf("unable to parse file data: %s. error: %s\n",
					file, err.Error())
				continue
			}

			kind, ok := cfg["kind"].(string)
			if !ok {
				fmt.Printf("unable to retrieve the kind of config. file: %s\n", file)
				continue
			}

			meta, ok := cfg["metadata"].(map[interface{}]interface{})
			if !ok {
				fmt.Printf("unable to get metadata from config. file: %s\n", file)
				continue
			}

			namespace, ok := meta["namespace"].(string)
			if !ok {
				fmt.Printf("unable to get Namespace from config. file: %s\n", file)
				continue
			}

			name, ok := meta["name"].(string)
			if !ok {
				fmt.Printf("unable to get name from config. file: %s\n", file)
				continue
			}

			if err := cmd.checkIfResourcePresent(namespace, kind, name); err != nil {
				if strings.Contains(err.Error(), "not found") {
					if err := cmd.applyResourceFile(file); err != nil {
						fmt.Printf(
							"unable to apply file %s. error: %s\n", file, err.Error())
						continue

					}
					continue
				} else {
					fmt.Printf("unable to check if resource is installed. error: %s\n", err.Error())
					continue
				}
			}

			_, err := cmd.Kubectl().RunCmd("-n",
				strings.ToLower(namespace),
				"patch",
				kind,
				strings.ToLower(name),
				"--type=merge",
				"-p",
				c)
			if err != nil {
				fmt.Printf("unable to override values. File: %s. Error: %s\n", file, err.Error())
				continue
			}
		}

	}

	return nil
}

func (cmd *command) checkIfResourcePresent(namespace, kind, name string) error {
	_, err := cmd.Kubectl().RunCmd("-n", namespace, "get", kind, name)
	return err
}

func (cmd *command) applyResourceFile(filepath string) error {
	_, err := cmd.Kubectl().RunCmd("apply", "-f", filepath)
	return err
}

func (cmd *command) setAdminPassword() error {
	if cmd.opts.Password == "" {
		return nil
	}
	encPass := base64.StdEncoding.EncodeToString([]byte(cmd.opts.Password))
	_, err := cmd.K8s.Static().CoreV1().ConfigMaps("kyma-installer").Patch("installation-config-overrides", types.JSONPatchType,
		[]byte(fmt.Sprintf("[{\"op\": \"replace\", \"path\": \"/data/global.adminPassword\", \"value\": \"%s\"}]", encPass)))
	if err != nil {
		err = errors.Wrap(err, "Error setting admin password")
	}
	return err
}

func (cmd *command) printSummary() error {
	v, err := version.KymaVersion(cmd.opts.Verbose, cmd.K8s)
	if err != nil {
		return err
	}

	adm, err := cmd.K8s.Static().CoreV1().Secrets("kyma-system").Get("admin-user", metav1.GetOptions{})
	if err != nil {
		return err
	}

	var consoleURL string
	vs, err := cmd.K8s.Istio().NetworkingV1alpha3().VirtualServices("kyma-system").Get("core-console", metav1.GetOptions{})
	switch {
	case apiErrors.IsNotFound(err):
		consoleURL = "not installed"
	case err != nil:
		return err
	case vs != nil && vs.Spec != nil && len(vs.Spec.Hosts) > 0:
		consoleURL = fmt.Sprintf("https://%s", vs.Spec.Hosts[0])
	default:
		return errors.New("Console host could not be obtained.")
	}

	fmt.Println()
	nice.PrintKyma()
	fmt.Print(" is installed in version:\t")
	nice.PrintImportant(v)

	nice.PrintKyma()
	fmt.Print(" is running at:\t\t")
	nice.PrintImportant(cmd.K8s.Config().Host)

	nice.PrintKyma()
	fmt.Print(" console:\t\t\t")
	nice.PrintImportantf(consoleURL)

	nice.PrintKyma()
	fmt.Print(" admin email:\t\t")
	nice.PrintImportant(string(adm.Data["email"]))

	if cmd.opts.Password == "" || cmd.opts.NonInteractive {
		nice.PrintKyma()
		fmt.Printf(" admin password:\t\t")
		nice.PrintImportant(string(adm.Data["password"]))
	}

	if cmd.opts.Domain != localDomain {
		fmt.Printf("\nTo access the console, configure DNS for the cluster load balancer: ")
		nice.PrintImportant("https://kyma-project.io/docs/#installation-use-your-own-domain--provider-domain--gke--configure-dns-for-the-cluster-load-balancer")
	}

	fmt.Printf("\nHappy ")
	nice.PrintKyma()
	fmt.Printf("-ing! :)\n\n")

	return nil
}

func (cmd *command) waitForInstaller() error {
	currentDesc := ""
	cmd.NewStep("Waiting for installation to start")

	status, err := cmd.Kubectl().RunCmd("get", "installation/kyma-installation", "-o", "jsonpath='{.status.state}'")
	if err != nil {
		return err
	}
	if status == "Installed" {
		return nil
	}

	var timeout <-chan time.Time
	var errorOccured bool
	if cmd.opts.Timeout > 0 {
		timeout = time.After(cmd.opts.Timeout)
	}

	for {
		select {
		case <-timeout:
			cmd.CurrentStep.Failure()
			cmd.printInstallationErrorLog()
			return errors.New("Timeout reached while waiting for installation to complete")
		default:
			status, desc, err := cmd.getInstallationStatus()
			if err != nil {
				// A timeout when asking for the status can happen if the cluster is under high load while installing Kyma.
				// But it should not make the CLI stop waiting immediately.
				if strings.Contains("operation timed out", err.Error()) {
					cmd.CurrentStep.LogError("Could not get the status, retrying...")
				} else {
					return err
				}
			}

			switch status {
			case "Installed":
				cmd.CurrentStep.Success()
				return nil

			case "Error":
				if !errorOccured {
					errorOccured = true
					cmd.CurrentStep.LogErrorf("%s failed, which may be OK. Will retry later...", desc)
					cmd.CurrentStep.LogInfo("To fetch the error logs from the installer, run: kubectl get installation kyma-installation -o go-template --template='{{- range .status.errorLog }}{{printf \"%s:\\n %s\\n\" .component .log}}{{- end}}'")
					cmd.CurrentStep.LogInfo("To fetch the application logs from the installer, run: kubectl logs -n kyma-installer -l name=kyma-installer")
				}

			case "InProgress":
				errorOccured = false
				// only do something if the description has changed
				if desc != currentDesc {
					cmd.CurrentStep.Success()
					cmd.CurrentStep = cmd.opts.NewStep(fmt.Sprintf(desc))
					currentDesc = desc
				}

			case "":
				cmd.CurrentStep.LogInfo("Failed to get the installation status. Will retry later...")

			default:
				cmd.CurrentStep.Failure()
				fmt.Printf("Unexpected status: %s\n", status)
				os.Exit(1)
			}
			time.Sleep(sleep)
		}
	}
}

func (cmd *command) getInstallationStatus() (status string, desc string, err error) {
	status, err = cmd.Kubectl().RunCmd("get", "installation/kyma-installation", "-o", "jsonpath='{.status.state}'")
	if err != nil {
		return
	}
	desc, err = cmd.Kubectl().RunCmd("get", "installation/kyma-installation", "-o", "jsonpath='{.status.description}'")
	return
}

func (cmd *command) printInstallationErrorLog() error {
	logs, err := cmd.Kubectl().RunCmd("get", "installation", "kyma-installation", "-o", "go-template", "--template={{- range .status.errorLog -}}{{printf \"%s:\n %s [%s]\n\" .component .log .occurrences}}{{- end}}")
	if err != nil {
		return err
	}
	fmt.Println(logs)
	return nil
}

func (cmd *command) releaseSrcFile(path string) string {
	return fmt.Sprintf(releaseSrcUrlPattern, cmd.opts.ConfigVersion, path)
}

func (cmd *command) releaseFile(path string) string {
	return fmt.Sprintf(releaseResourcePattern, cmd.opts.ConfigVersion, path)
}

func (cmd *command) importCertificate(ca trust.Certifier) error {
	if !cmd.opts.NoWait {
		// get cert from cluster
		cert, err := ca.Certificate()
		if err != nil {
			return err
		}

		tmpFile, err := ioutil.TempFile(os.TempDir(), "kyma-*.crt")
		if err != nil {
			return errors.Wrap(err, "Cannot create temporary file for Kyma certificate")
		}
		defer os.Remove(tmpFile.Name())

		if _, err = tmpFile.Write(cert); err != nil {
			return errors.Wrap(err, "Failed to write the kyma certificate")
		}
		if err := tmpFile.Close(); err != nil {
			return err
		}

		if err := ca.StoreCertificate(tmpFile.Name(), cmd.CurrentStep); err != nil {
			return err
		}
		cmd.CurrentStep.Successf("Kyma root certificate imported")

	} else {
		cmd.CurrentStep.LogError(ca.Instructions())
	}
	return nil
}

func (cmd *command) addDevDomainsToEtcHosts(s step.Step, clusterInfo clusterInfo) error {
	hostnames := ""

	vsList, err := cmd.K8s.Istio().NetworkingV1alpha3().VirtualServices("kyma-system").List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, v := range vsList.Items {
		for _, host := range v.Spec.Hosts {
			hostnames = hostnames + " " + host
		}
	}

	hostAlias := "127.0.0.1" + hostnames

	if clusterInfo.localVMDriver != "none" {
		_, err := minikube.RunCmd(cmd.opts.Verbose, clusterInfo.profile, "ssh", "sudo /bin/sh -c 'echo \""+hostAlias+"\" >> /etc/hosts'")
		if err != nil {
			return err
		}
	}

	hostAlias = strings.Trim(clusterInfo.localIP, "\n") + hostnames

	return addDevDomainsToEtcHostsOSSpecific(cmd.opts.Domain, s, hostAlias)
}

func (cmd *command) getClusterInfoFromConfigMap() (clusterInfo, error) {
	cm, err := cmd.K8s.Static().CoreV1().ConfigMaps("kube-system").Get("kyma-cluster-info", metav1.GetOptions{})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return clusterInfo{}, nil
		}
		return clusterInfo{}, err
	}

	isLocal, err := strconv.ParseBool(cm.Data["isLocal"])
	if err != nil {
		isLocal = false
	}

	clusterConfig := clusterInfo{
		isLocal:       isLocal,
		provider:      cm.Data["provider"],
		profile:       cm.Data["profile"],
		localIP:       cm.Data["localIP"],
		localVMDriver: cm.Data["localVMDriver"],
	}

	return clusterConfig, nil
}

func (cmd *command) patchMinikubeIP(minikubeIP string) error {
	if _, err := cmd.K8s.Static().CoreV1().ConfigMaps("kyma-installer").Get("installation-config-overrides", metav1.GetOptions{}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			cmd.CurrentStep.LogInfof("Resource '%s' not found, won't be patched", "configmap/installation-config-overrides")
		} else {
			return err
		}
	}

	if minikubeIP != "" {
		_, err := cmd.K8s.Static().CoreV1().ConfigMaps("kyma-installer").Patch("installation-config-overrides", types.JSONPatchType,
			[]byte(fmt.Sprintf("[{\"op\": \"replace\", \"path\": \"/data/global.minikubeIP\", \"value\": \"%s\"}]", minikubeIP)))
		if err != nil {
			return err
		}
	}
	return nil
}

func (cmd *command) createOwnDomainConfigMap() error {
	_, err := cmd.K8s.Static().CoreV1().ConfigMaps("kyma-installer").Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "owndomain-overrides",
			Labels: map[string]string{"installer": "overrides"},
		},
		Data: map[string]string{
			"global.domainName": cmd.opts.Domain,
			"global.tlsCrt":     cmd.opts.TLSCert,
			"global.tlsKey":     cmd.opts.TLSKey,
		},
	})

	return err
}
