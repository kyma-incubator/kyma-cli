package run

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	oct "github.com/kyma-incubator/octopus/pkg/apis/testing/v1alpha1"
	"github.com/kyma-project/cli/cmd/kyma/test"
	"github.com/kyma-project/cli/internal/cli"
	"github.com/kyma-project/cli/internal/kube"
	"github.com/kyma-project/cli/pkg/api/octopus"
	"github.com/kyma-project/cli/pkg/step"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

type command struct {
	opts *options
	cli.Command
}

func NewCmd(o *options) *cobra.Command {
	cmd := command{
		Command: cli.Command{Options: o.Options},
		opts:    o,
	}

	cobraCmd := &cobra.Command{
		Use:   "run <test-definition-1> <test-defintion-2> ... <test-definition-N>",
		Short: "Runs tests on a Kyma cluster.",
		Long: `Use this command to run tests on a Kyma cluster.

Remarks: 
If you don't provide any specific test definitions, all available test definitions will be added to the newly created test suite.
To execute all test defintions, run ` + "`kyma test run -n example-test`" + `.

`,
		RunE:    func(_ *cobra.Command, args []string) error { return cmd.Run(args) },
		Aliases: []string{"r"},
	}

	cobraCmd.Flags().StringVarP(&o.Name, "name", "n", "", `Specifies the name of the new test suite. If you don't specify the value for the "-n" flag, the name of the test suite will be autogenerated.`)
	cobraCmd.Flags().Int64VarP(&o.ExecutionCount, "count", "c", 1, `Defines how many times every test should be executed. "count" and "max-retries" flags are mutually exclusive.`)
	cobraCmd.Flags().Int64VarP(&o.MaxRetries, "max-retries", "", 0, `Defines how many times a given test is retried when it fails. A suite is marked with a "succeeded" status even if some tests failed at first and then finally succeeded. The default value of 0 means that there are no retries of a given test.`)
	cobraCmd.Flags().Int64VarP(&o.Concurrency, "concurrency", "", 1, "Specifies the number of tests to be executed in parallel.")
	cobraCmd.Flags().DurationVar(&o.Timeout, "timeout", 0, `Maximum time during which the test suite is being watched, zero means infinite. Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".`)
	cobraCmd.Flags().BoolVarP(&o.Watch, "watch", "w", o.Watch, "Watch the status of the test suite until it finishes or the defined `--timeout` occurs.")
	return cobraCmd
}

func (cmd *command) Run(args []string) error {
	var err error
	if cmd.opts.Watch {
		if cmd.K8s, err = kube.NewFromConfigWithTimeout("", cmd.KubeconfigPath, cmd.opts.Timeout); err != nil {
			return errors.Wrapf(err, "Could not initialize the Kubernetes client with %d timeout. Make sure your kubeconfig is valid.", cmd.opts.Timeout)
		}
	} else {
		if cmd.K8s, err = kube.NewFromConfig("", cmd.KubeconfigPath); err != nil {
			return errors.Wrap(err, "Could not initialize the Kubernetes client. Make sure your kubeconfig is valid.")
		}
	}

	var testSuiteName string
	if len(cmd.opts.Name) > 0 {
		testSuiteName = cmd.opts.Name
	} else {
		rand.Seed(time.Now().UTC().UnixNano())
		rnd := rand.Int31()
		testSuiteName = fmt.Sprintf("test-%d", rnd)
	}

	tNotExists, err := verifyIfTestNotExists(testSuiteName, cmd.K8s.Octopus())
	if err != nil {
		return err
	}
	if !tNotExists {
		return fmt.Errorf("Test suite '%s' already exists", testSuiteName)
	}

	clusterTestDefs, err := cmd.K8s.Octopus().ListTestDefinitions(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Unable to get the list of test definitions")
	}

	var testDefToApply []oct.TestDefinition
	if len(args) == 0 {
		testDefToApply = clusterTestDefs.Items
	} else {
		if testDefToApply, err = matchTestDefinitionNames(args,
			clusterTestDefs.Items); err != nil {
			return err
		}
	}

	testResource := generateTestsResource(testSuiteName,
		cmd.opts.ExecutionCount, cmd.opts.MaxRetries,
		cmd.opts.Concurrency, testDefToApply)

	if _, err := cmd.K8s.Octopus().CreateTestSuite(testResource); err != nil {
		return err
	}
	fmt.Printf("- Test suite '%s' successfully created\r\n", testSuiteName)

	waitStep := cmd.NewStep("Waiting for test suite to finish")
	err = waitForTestSuite(cmd.K8s.Octopus(), testResource.Name, clusterTestSuiteCompleted(waitStep), cmd.opts.Timeout)
	if err != nil {
		waitStep.Failure()
		return err
	}

	return nil
}

func matchTestDefinitionNames(testNames []string,
	testDefs []oct.TestDefinition) ([]oct.TestDefinition, error) {
	result := []oct.TestDefinition{}
	for _, tName := range testNames {
		found := false
		for _, tDef := range testDefs {
			if strings.EqualFold(tName, tDef.GetName()) {
				found = true
				result = append(result, tDef)
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("Test defintion '%s' not found in the list of cluster test definitions", tName)
		}
	}
	return result, nil
}

func generateTestsResource(testName string, numberOfExecutions,
	maxRetries, concurrency int64,
	testDefinitions []oct.TestDefinition) *oct.ClusterTestSuite {

	octTestDefs := test.NewTestSuite(testName)
	matchNames := []oct.TestDefReference{}
	for _, td := range testDefinitions {
		matchNames = append(matchNames, oct.TestDefReference{
			Name:      td.GetName(),
			Namespace: td.GetNamespace(),
		})
	}
	octTestDefs.Spec.MaxRetries = maxRetries
	octTestDefs.Spec.Concurrency = concurrency
	octTestDefs.Spec.Count = numberOfExecutions
	octTestDefs.Spec.Selectors.MatchNames = matchNames

	return octTestDefs
}

func listTestSuiteNames(cli octopus.OctopusInterface) ([]string, error) {
	suites, err := cli.ListTestSuites(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Unable to list test suites")
	}

	var result = make([]string, len(suites.Items))
	for i := 0; i < len(suites.Items); i++ {
		result[i] = suites.Items[i].GetName()
	}
	return result, nil
}

func verifyIfTestNotExists(suiteName string,
	cli octopus.OctopusInterface) (bool, error) {
	tests, err := listTestSuiteNames(cli)
	if err != nil {
		return false, err
	}
	for _, t := range tests {
		if t == suiteName {
			return false, nil
		}
	}
	return true, nil
}

// waitForTestSuite watches the given test suite until the exitCondition is true
func waitForTestSuite(cli octopus.OctopusInterface, name string, exitCondition watchtools.ConditionFunc, timeout time.Duration) error {
	ctx, cancel := context.WithCancel(context.Background())
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	preconditionFunc := func(store cache.Store) (bool, error) {
		_, exists, err := store.Get(&metav1.ObjectMeta{Name: name})
		if err != nil {
			return true, err
		}
		if !exists {
			// We need to make sure we see the object in the cache before we start waiting for events
			// or we would be waiting for the timeout if such object didn't exist.
			return true, apierrors.NewNotFound(oct.Resource("clustertestsuites"), name)
		}

		return false, nil
	}

	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return cli.ListTestSuites(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return cli.WatchTestSuite(options)
		},
	}

	// TODO(mszostok): use the `interrupt.New(nil, cancel)` func from the
	//  `k8s.io/kubectl/pkg/util/interrupt` pkg when switching to k8s 1.16
	_, err := watchtools.UntilWithSync(ctx, lw, &oct.ClusterTestSuite{}, preconditionFunc, func(ev watch.Event) (bool, error) {
		return exitCondition(ev)
	})

	return err
}

// clusterTestSuiteCompleted returns true if the suite has run to completion, false if the suite has not yet
// reached running state, or an error in any other case.
func clusterTestSuiteCompleted(statusReporter step.Step) func(event watch.Event) (bool, error) {
	return func(event watch.Event) (bool, error) {
		switch t := event.Type; t {
		case watch.Added, watch.Modified:
			switch ts := event.Object.(type) {
			case *oct.ClusterTestSuite:
				for _, cond := range ts.Status.Conditions {
					if cond.Status == oct.StatusTrue {
						switch cond.Type {
						case oct.SuiteSucceeded:
							statusReporter.Successf("Test suite '%s' execution succeeded", ts.Name)
							return true, nil
						case oct.SuiteError:
							statusReporter.Failuref("Test suite '%s' execution errored", ts.Name)
							return true, nil
						case oct.SuiteFailed:
							statusReporter.Failuref("Test suite '%s' execution failed", ts.Name)
							return true, nil
						}
					}
					statusReporter.Status(testResultStatistic(ts))
				}
			}
		case watch.Deleted:
			// We need to abort to avoid cases of recreation and not to silently watch the wrong (new) object
			return false, apierrors.NewNotFound(oct.Resource("clustertestsuites"), "")
		default:
			return true, fmt.Errorf("internal error: unexpected event %#v", event)
		}

		return false, nil
	}
}

func testResultStatistic(ts *oct.ClusterTestSuite) string {
	var (
		succeeded = 0
		failed    = 0
		skipped   = 0
	)
	for _, t := range ts.Status.Results {
		switch t.Status {
		case oct.TestFailed:
			failed++
		case oct.TestSucceeded:
			succeeded++
		case oct.TestSkipped:
			skipped++
		}
	}

	finished := failed + succeeded + skipped
	all := len(ts.Status.Results)

	return fmt.Sprintf("%d out of %d test(s) have finished (Succeeded: %d, Failed: %d, Skipped: %d)...",
		finished, all, succeeded, failed, skipped)
}
