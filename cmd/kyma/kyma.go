package main

import (
	"fmt"
	"os"

	"github.com/kyma-incubator/kymactl/pkg/kyma/cmd"
	"github.com/kyma-incubator/kymactl/pkg/kyma/core"
)

func main() {
	command := cmd.NewKymaCmd(core.NewOptions())

	err := command.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
