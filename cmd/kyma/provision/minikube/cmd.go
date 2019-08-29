package minikube

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/cli/internal/cli"
	"github.com/kyma-project/cli/internal/kube"
	"github.com/kyma-project/cli/internal/minikube"
	"github.com/kyma-project/cli/internal/step"
	"github.com/spf13/cobra"
)

const (
	kubernetesVersion  string = "1.12.5"
	bootstrapper       string = "kubeadm"
	vmDriverHyperkit   string = "hyperkit"
	vmDriverHyperv     string = "hyperv"
	vmDriverNone       string = "none"
	vmDriverVirtualBox string = "virtualbox"
	sleep                     = 10 * time.Second
)

var (
	drivers = []string{
		"virtualbox",
		"vmwarefusion",
		"kvm",
		"xhyve",
		vmDriverHyperv,
		vmDriverHyperkit,
		"kvm2",
		"none",
	}
	ErrMinikubeRunning = errors.New("Minikube already running")
)

type command struct {
	opts *options
	cli.Command
}

//NewCmd creates a new minikube command
func NewCmd(o *options) *cobra.Command {

	c := command{
		Command: cli.Command{Options: o.Options},
		opts:    o,
	}

	cmd := &cobra.Command{
		Use:     "minikube",
		Short:   "Provisions Minikube",
		Long:    `Provisions Minikube for Kyma installation`,
		RunE:    func(_ *cobra.Command, _ []string) error { return c.Run() },
		Aliases: []string{"m"},
	}

	cmd.Flags().StringVar(&o.VMDriver, "vm-driver", defaultVMDriver, "VMDriver to use. Possible values: "+strings.Join(drivers, ","))
	cmd.Flags().StringVar(&o.HypervVirtualSwitch, "hypervVirtualSwitch", "", "Name of the hyperv switch, required if --vm-driver=hyperv")
	cmd.Flags().StringVar(&o.DiskSize, "disk-size", "30g", "Disk size to use")
	cmd.Flags().StringVar(&o.Memory, "memory", "8192", "Memory to use")
	cmd.Flags().StringVar(&o.CPUS, "cpus", "4", "CPUs to use")
	return cmd
}

//Run runs the command
func (c *command) Run() error {
	s := c.NewStep("Checking requirements")
	if err := c.checkRequirements(s); err != nil {
		s.Failure()
		return err
	}
	s.Successf("Requirements verified")

	s.LogInfof("Preparing Minikube using vm-driver '%s'", c.opts.VMDriver)

	s = c.NewStep("Checking Minikube status")
	err := c.checkIfMinikubeIsInitialized(s)
	switch err {
	case ErrMinikubeRunning, nil:
		break
	default:
		s.Failure()
		return err
	}
	s.Successf("Minikube status verified")

	s = c.NewStep(fmt.Sprintf("Initializing Minikube config"))
	err = c.initializeMinikubeConfig()
	if err != nil {
		s.Failure()
		return err
	}
	s.Successf("Minikube config initialized")

	s = c.NewStep(fmt.Sprintf("Create Minikube instance"))
	s.Status("Start Minikube")
	err = c.startMinikube()
	if err != nil {
		s.Failure()
		return err
	}

	s.Status("Wait for Minikube to be up and running")
	err = c.waitForMinikubeToBeUp(s)
	if err != nil {
		s.Failure()
		return err
	}

	// K8s client needs to be created here because before the kubeconfig is not ready to use
	if c.K8s, err = kube.NewFromConfig("", c.KubeconfigPath); err != nil {
		return errors.Wrap(err, "Could not initialize the Kubernetes client. Make sure your kubeconfig is valid")
	}

	s.Status("Create default cluster role")
	err = c.createClusterRoleBinding()
	if err != nil {
		s.Failure()
		return err
	}

	s.Status("Wait for kube-dns to be up and running")
	err = c.K8s.WaitPodStatusByLabel("kube-system", "k8s-app", "kube-dns", corev1.PodRunning)
	if err != nil {
		s.Failure()
		return err
	}
	s.Successf("Minikube up and running")

	s = c.NewStep(fmt.Sprintf("Adjusting Minikube cluster"))
	s.Status("Increase fs.inotify.max_user_instances")
	err = c.increaseFsInotifyMaxUserInstances()
	if err != nil {
		s.Failure()
		return err
	}
	s.Status("Enable metrics server")
	err = c.enableMetricsServer()
	if err != nil {
		s.Failure()
		return err
	}
	s.Successf("Adjustments finished")

	s = c.NewStep(fmt.Sprintf("Creating cluster info ConfigMap"))
	err = c.createClusterInfoConfigMap()
	if err != nil {
		s.Failure()
		return err
	}
	s.Successf("ConfigMap created")

	err = c.printSummary()
	if err != nil {
		return err
	}

	return nil
}

func (c *command) checkRequirements(s step.Step) error {
	if !driverSupported(c.opts.VMDriver) {
		s.Failure()
		return fmt.Errorf("Specified VMDriver '%s' is not supported by Minikube", c.opts.VMDriver)
	}
	if c.opts.VMDriver == vmDriverHyperv && c.opts.HypervVirtualSwitch == "" {
		s.Failure()
		return fmt.Errorf("Specified VMDriver '%s' requires the --hypervVirtualSwitch option", vmDriverHyperv)
	}

	versionWarning, err := minikube.CheckVersion(c.opts.Verbose)
	if err != nil {
		s.Failure()
		return err
	}
	if versionWarning != "" {
		s.LogError(versionWarning)
	}

	return nil
}

func (c *command) checkIfMinikubeIsInitialized(s step.Step) error {
	statusText, _ := minikube.RunCmd(c.opts.Verbose, "status", "-b", bootstrapper, "--format", "{{.Host}}")

	if strings.TrimSpace(statusText) != "" {
		var answer bool
		if !c.opts.NonInteractive {
			answer = s.PromptYesNo("Do you want to remove the existing Minikube cluster? ")
		}
		if c.opts.NonInteractive || answer {
			_, err := minikube.RunCmd(c.opts.Verbose, "delete")
			if err != nil {
				return err
			}
		} else {
			return ErrMinikubeRunning
		}
	}
	return nil
}

func (c *command) initializeMinikubeConfig() error {
	// Disable default nginx ingress controller
	_, err := minikube.RunCmd(c.opts.Verbose, "config", "unset", "ingress")
	if err != nil {
		return err
	}
	return nil
}

func (c *command) startMinikube() error {
	startCmd := []string{"start",
		"--memory", c.opts.Memory,
		"--cpus", c.opts.CPUS,
		"--extra-config=apiserver.authorization-mode=RBAC",
		"--extra-config=apiserver.cors-allowed-origins='http://*'",
		"--extra-config=apiserver.enable-admission-plugins=DefaultStorageClass,LimitRanger,MutatingAdmissionWebhook,NamespaceExists,NamespaceLifecycle,ResourceQuota,ServiceAccount,ValidatingAdmissionWebhook",
		"--kubernetes-version=v" + kubernetesVersion,
		"--vm-driver", c.opts.VMDriver,
		"--disk-size", c.opts.DiskSize,
		"-b", bootstrapper,
	}

	if c.opts.VMDriver == vmDriverHyperv {
		startCmd = append(startCmd, "--hyperv-virtual-switch="+c.opts.HypervVirtualSwitch)

	}
	_, err := minikube.RunCmd(c.opts.Verbose, startCmd...)
	if err != nil {
		return err
	}
	return nil
}

// fixes https://github.com/kyma-project/kyma/issues/1986
func (c *command) createClusterRoleBinding() error {
	var err error
	bs, err := c.K8s.Static().RbacV1().ClusterRoleBindings().List(metav1.ListOptions{LabelSelector: "app=kyma"})
	if err != nil {
		return err
	}
	if len(bs.Items) == 0 {
		_, err = c.K8s.Static().RbacV1().ClusterRoleBindings().Create(&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "default-sa-cluster-admin",
				Labels: map[string]string{"app": "kyma"},
			},
			RoleRef: rbacv1.RoleRef{
				Name: "cluster-admin",
				Kind: "ClusterRole",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Namespace: "kube-system",
					Name:      "default",
				},
			},
		})
	}
	return err
}

func (c *command) waitForMinikubeToBeUp(step step.Step) error {
	for {
		statusText, err := minikube.RunCmd(c.opts.Verbose, "status", "-b="+bootstrapper, "--format", "'{{.Host}}'")
		if err != nil {
			return err
		}
		step.Status(statusText)

		if strings.TrimSpace(statusText) == "Running" {
			break
		}
		time.Sleep(sleep)
	}

	for {
		statusText, err := minikube.RunCmd(c.opts.Verbose, "status", "-b="+bootstrapper, "--format", "'{{.Kubelet}}'")
		if err != nil {
			return err
		}
		step.Status(statusText)

		if strings.TrimSpace(statusText) == "Running" {
			break
		}
		time.Sleep(sleep)
	}

	return nil
}

// Default value of 128 is not enough to perform “kubectl log -f” from pods, hence increased to 524288
func (c *command) increaseFsInotifyMaxUserInstances() error {
	if c.opts.VMDriver != vmDriverNone {
		_, err := minikube.RunCmd(c.opts.Verbose, "ssh", "--", "sudo sysctl -w fs.inotify.max_user_instances=524288")
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *command) enableMetricsServer() error {
	_, err := minikube.RunCmd(c.opts.Verbose, "addons", "enable", "metrics-server")
	if err != nil {
		return err
	}
	return nil
}

func (c *command) printSummary() error {
	fmt.Println()
	fmt.Println("Minikube cluster installed")
	clusterInfo, err := minikube.RunCmd(c.opts.Verbose, "status", "-b="+bootstrapper)
	if err != nil {
		fmt.Printf("Cannot show cluster-info because of '%s", err)
	} else {
		fmt.Println(clusterInfo)
	}

	fmt.Println("Happy Minikube-ing! :)")
	return nil
}

func driverSupported(driver string) bool {
	for _, element := range drivers {
		if element == driver {
			return true
		}
	}
	return false
}

func (c *command) createClusterInfoConfigMap() error {
	minikubeIP := c.getMinikubeIP()

	_, err := c.K8s.Static().CoreV1().ConfigMaps("kube-system").Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "kyma-cluster-info",
			Labels: map[string]string{"app": "kyma"},
		},
		Data: map[string]string{
			"provider":      "minikube",
			"isLocal":       "true",
			"localIP":       minikubeIP,
			"localVMDriver": c.opts.VMDriver,
		},
	})

	return err
}

func (c *command) getMinikubeIP() string {
	minikubeIP, err := minikube.RunCmd(c.opts.Verbose, "ip")
	if err != nil {
		c.CurrentStep.LogInfo("Unable to perform 'minikube ip' command. IP won't be passed to Kyma")
		return ""
	}
	return strings.TrimSpace(minikubeIP)
}
