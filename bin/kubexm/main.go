package main

import (
	"os"

	"github.com/mensylisir/kubexm/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
