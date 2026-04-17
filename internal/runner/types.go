package runner

import (
	"github.com/mensylisir/kubexm/internal/connector"
)

// Re-export connector types needed by upper layers (step/task) that cannot
// import connector directly due to layer isolation constraints.
//
// These are thin aliases — the canonical definitions live in the connector package.
// Step and Task layers must NOT import connector directly; use these instead.

type (
	// Connector is an alias for connector.Connector, exposed so upper layers can
	// reference connector handles without importing connector directly.
	Connector = connector.Connector

	// ExecOptions is an alias for connector.ExecOptions, which is used when
	// calling runner.RunWithOptions().
	ExecOptions = connector.ExecOptions

	// FileTransferOptions is an alias for connector.FileTransferOptions,
	// used when transferring files to/from remote hosts.
	FileTransferOptions = connector.FileTransferOptions

	// StatOptions is an alias for connector.StatOptions,
	// used when checking file existence with options.
	StatOptions = connector.StatOptions

	// LookPathOptions is an alias for connector.LookPathOptions,
	// used when looking up executables with options.
	LookPathOptions = connector.LookPathOptions
)

// CommandError is an alias for connector.CommandError.
// Step files type-assert errors returned by runner methods to check exit codes.
type CommandError = connector.CommandError
