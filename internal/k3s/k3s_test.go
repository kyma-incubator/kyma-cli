package k3s

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Place this folder at the beginning of PATH env-var to ensure this
// mock-script will be used instead of a locally installed k3d tool.
func init() {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	os.Setenv("PATH", fmt.Sprintf("%s:%s", filepath.Dir(ex)+string(os.PathSeparator)+"mock", os.Getenv("PATH")))
}

// function to verify output of k3d tool
type testFunc func(output string, err error)

func TestRunCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cmd      []string
		verifyer testFunc
	}{
		{
			cmd: []string{"help"},
			verifyer: testFunc(func(output string, err error) {
				if !strings.Contains(output, "--help") {
					require.Fail(t, fmt.Sprintf("Expected string '--help' is missing in k3d output: %s", output))
				}
			}),
		},
		{
			cmd: []string{"cluster", "list"},
			verifyer: testFunc(func(output string, err error) {
				if !strings.Contains(output, "kyma-cluster") {
					require.Fail(t, fmt.Sprintf("Expected string 'kyma-cluster' is missing in k3d output: %s", output))
				}
			}),
		},
		{
			cmd: []string{"cluster", "xyz"},
			verifyer: testFunc(func(output string, err error) {
				require.NotEmpty(t, err, "Error object expected")
			}),
		},
	}

	for testID, testCase := range tests {
		output, err := RunCmd(false, 5*time.Second, testCase.cmd...)
		if testCase.verifyer == nil {
			require.Fail(t, fmt.Sprintf("Verifyer function missing for test #'%d'", testID))
		}
		testCase.verifyer(output, err)
	}

}

func TestCheckVersion(t *testing.T) {
	err := CheckVersion(false)
	if err != nil {
		require.Fail(t, fmt.Sprintf("k3d version check failed: %s", err))
	}
}

func TestInitialize(t *testing.T) {
	err := Initialize(false)
	if err != nil {
		require.Fail(t, fmt.Sprintf("k3d initialize: %s", err))
	}
}
