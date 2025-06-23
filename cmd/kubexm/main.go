package main

import (
	"os"
	"github.com/mensylisir/kubexm/cmd/kubexm/cmd" // Adjusted path
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
