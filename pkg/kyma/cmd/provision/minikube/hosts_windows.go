// +build windows

package minikube

import (
	"fmt"
	"strings"

	"github.com/kyma-project/cli/internal/step"
)

func addDevDomainsToEtcHostsOSSpecific(o *MinikubeOptions, s step.Step, hostAlias string) error {

	s.LogErrorf("Please add these lines to your " + hostsFile + " file:")
	hostsArray := strings.Split(hostAlias, " ")
	ip := hostsArray[0]
	hostsArray = hostsArray[1:]
	for len(hostsArray) > 0 {
		chunkLen := 7 // max hosts per line
		if len(hostsArray) < chunkLen {
			chunkLen = len(hostsArray)
		}
		fmt.Printf("%s %s\n", ip, strings.Join(hostsArray[:chunkLen], " "))
		hostsArray = hostsArray[chunkLen:]
	}
	return nil
}
