package installation

import (
	"errors"
	"testing"
	"time"

	installSDK "github.com/kyma-incubator/hydroform/install/installation"
	k8sMocks "github.com/kyma-project/cli/internal/kube/mocks"
	"github.com/kyma-project/cli/pkg/installation/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	fakeIstio "istio.io/client-go/pkg/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestUpgradeKyma(t *testing.T) {
	t.Parallel()
	// prepare mocks
	kymaMock := k8sMocks.KymaKube{}
	iServiceMock := mocks.Service{}

	// fake k8s with installer pod running and post installation resources
	k8sMock := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metaV1.ObjectMeta{Name: "kyma-installer", Namespace: "kyma-installer", Labels: map[string]string{"name": "kyma-installer"}},
			Status:     v1.PodStatus{Phase: v1.PodRunning},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "Installer",
						Image: "fake-registry/installer:1.15.0",
					},
				},
			},
		},
		&v1.Secret{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "admin-user",
				Namespace: "kyma-system",
			},
			Data: map[string][]byte{
				"email":    []byte("admin@fake.com"),
				"password": []byte("1234-super-secure"),
			},
		},
	)

	// fake istio vService
	istioMock := fakeIstio.NewSimpleClientset(
		&v1alpha3.VirtualService{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      "console-web",
				Namespace: "kyma-system",
			},
			Spec: networkingv1alpha3.VirtualService{
				Hosts: []string{"fake-console-url"},
			},
		},
	)

	kymaMock.On("Static").Return(k8sMock)
	kymaMock.On("Istio").Return(istioMock)
	kymaMock.On("RestConfig", mock.Anything).Return(&rest.Config{Host: "fake-kubeconfig-host"})
	kymaMock.On("WaitPodStatusByLabel", "kyma-installer", "name", "kyma-installer", v1.PodRunning).Return(nil)

	i := &Installation{
		K8s:     &kymaMock,
		Service: &iServiceMock,
		Options: &Options{
			NoWait:           false,
			NonInteractive:   true,
			Timeout:          10 * time.Minute,
			Domain:           "irrelevant",
			TLSCert:          "fake-cert",
			TLSKey:           "fake-key",
			Password:         "fake-password",
			OverrideConfigs:  nil,
			ComponentsConfig: "",
			IsLocal:          false,
			Source:           "1.15.1",
		},
	}

	// Happy path
	iServiceMock.On("CheckInstallationState", mock.Anything).Return(installSDK.InstallationState{State: "Installed"}, nil).Times(3)
	iServiceMock.On("TriggerUpgrade", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	r, err := i.UpgradeKyma()
	require.NoError(t, err)
	require.NotEmpty(t, r)

	// Installation in progress
	i.Options.NoWait = true // no need to wait for upgrade in all test cases from here on
	iServiceMock.On("CheckInstallationState", mock.Anything).Return(installSDK.InstallationState{State: "InProgress"}, nil).Once()

	r, err = i.UpgradeKyma()
	require.NoError(t, err)
	require.Empty(t, r)

	// No Kyma on cluster
	iServiceMock.On("CheckInstallationState", mock.Anything).Return(installSDK.InstallationState{State: installSDK.NoInstallationState}, nil).Once()

	r, err = i.UpgradeKyma()
	require.Error(t, err)
	require.Empty(t, r)

	// Error getting installation status
	iServiceMock.On("CheckInstallationState", mock.Anything).Return(installSDK.InstallationState{}, errors.New("installation is hiding from us")).Once()

	r, err = i.UpgradeKyma()
	require.Error(t, err)
	require.Empty(t, r)

	// Empty installation status
	iServiceMock.On("CheckInstallationState", mock.Anything).Return(installSDK.InstallationState{}, nil).Once()

	r, err = i.UpgradeKyma()
	require.Error(t, err)
	require.Empty(t, r)
}
