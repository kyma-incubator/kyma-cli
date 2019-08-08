package octopus

import (
	"fmt"

	oct "github.com/kyma-incubator/octopus/pkg/apis/testing/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockedOctopusRestClient struct {
	testDefs   *oct.TestDefinitionList
	testSuites *oct.ClusterTestSuiteList
}

func NewMockedOctopusRestClient(testDefs *oct.TestDefinitionList, testSuites *oct.ClusterTestSuiteList) *MockedOctopusRestClient {
	return &MockedOctopusRestClient{
		testDefs:   testDefs,
		testSuites: testSuites,
	}
}

func (m *MockedOctopusRestClient) ListTestDefinitions(opts metav1.ListOptions) (result *oct.TestDefinitionList, err error) {
	return m.testDefs, nil
}

func (m *MockedOctopusRestClient) ListTestSuites(opts metav1.ListOptions) (result *oct.ClusterTestSuiteList, err error) {
	return m.testSuites, nil
}

func (m *MockedOctopusRestClient) CreateTestSuite(cts *oct.ClusterTestSuite) (result *oct.ClusterTestSuite, err error) {
	m.testSuites.Items = append(m.testSuites.Items, *cts)
	return cts, nil
}

func (m *MockedOctopusRestClient) DeleteTestSuite(name string, options metav1.DeleteOptions) error {
	for i := 0; i < len(m.testSuites.Items); i++ {
		if m.testSuites.Items[i].GetName() == name {
			m.testSuites.Items = append(m.testSuites.Items[i:],
				m.testSuites.Items[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("test not found")
}

func (m *MockedOctopusRestClient) GetTestSuite(name string, options metav1.GetOptions) (result *oct.ClusterTestSuite, err error) {
	for i := 0; i < len(m.testSuites.Items); i++ {
		if m.testSuites.Items[i].GetName() == name {
			return &m.testSuites.Items[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}
