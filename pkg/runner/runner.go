package runner

// defaultRunner is the concrete implementation of the Runner interface.
// Its methods are defined across multiple files in this package (e.g., facts.go, command.go, etc.).
type defaultRunner struct{}

// NewRunner creates a new instance of the defaultRunner, which implements the Runner interface.
func NewRunner() Runner {
	return &defaultRunner{}
}

// Note: All substantive method implementations for defaultRunner are now in other files
// within this package (e.g., facts.go, deploy.go, reboot.go, qemu.go, docker.go,
// command.go, file.go, etc.). This file, runner.go, primarily serves to define the
// defaultRunner struct and its constructor NewRunner().
