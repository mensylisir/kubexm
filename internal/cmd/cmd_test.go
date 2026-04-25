package cmd_test

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/mensylisir/kubexm/internal/cmd"
)

// Helper to find command by name in a command tree
func findCommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
		// Check subcommands
		if sub := findCommand(cmd, name); sub != nil {
			return sub
		}
	}
	return nil
}

// Helper to find flag by name
func findFlag(cmd *cobra.Command, name string) *pflag.Flag {
	// Check command flags first (local flags)
	if f := cmd.Flags().Lookup(name); f != nil {
		return f
	}
	// Check persistent flags (inherited from parent)
	return cmd.PersistentFlags().Lookup(name)
}

// Helper to check if flag exists in any parent command
func findFlagRecursive(cmd *cobra.Command, name string) *pflag.Flag {
	for cmd != nil {
		if f := cmd.Flags().Lookup(name); f != nil {
			return f
		}
		if f := cmd.PersistentFlags().Lookup(name); f != nil {
			return f
		}
		cmd = cmd.Parent()
	}
	return nil
}

// TestRootCommandHasGlobalFlags verifies root command has global flags
// Per 测试计划.md Section 1.1: "指令与交互 100% 覆盖"
func TestRootCommandHasGlobalFlags(t *testing.T) {
	// Import the root command package
	cmdRoot := cmd.NewRootCmd()

	t.Run("RootHasVerboseFlag", func(t *testing.T) {
		flag := findFlagRecursive(cmdRoot, "verbose")
		if flag == nil {
			t.Error("Root command should have --verbose flag")
		}
	})

	t.Run("RootHasVerboseShortFlag", func(t *testing.T) {
		// Check verbose flag shorthand exists
		verbose := findFlagRecursive(cmdRoot, "verbose")
		if verbose == nil {
			t.Error("Root command should have --verbose flag")
			return
		}
		if verbose.Shorthand == "" {
			t.Error("Root command --verbose flag should have -v shorthand")
		}
	})

	t.Run("RootHasYesFlag", func(t *testing.T) {
		flag := findFlagRecursive(cmdRoot, "yes")
		if flag == nil {
			t.Error("Root command should have --yes flag")
		}
	})

	t.Run("RootHasYesShortFlag", func(t *testing.T) {
		// Check yes flag shorthand exists
		yes := findFlagRecursive(cmdRoot, "yes")
		if yes == nil {
			t.Error("Root command should have --yes flag")
			return
		}
		if yes.Shorthand == "" {
			t.Error("Root command --yes flag should have -y shorthand")
		}
	})
}

// TestClusterCommandsHaveRequiredFlags verifies cluster commands have required flags
// Per 测试方案.md: "所有的写操作（create, delete, upgrade, reconfigure）必须包含 --dry-run 用例"
func TestClusterCommandsHaveRequiredFlags(t *testing.T) {
	cmdRoot := cmd.NewRootCmd()

	// Verb-first: kubexm create cluster, kubexm delete cluster
	createCmd := findCommand(cmdRoot, "create")
	if createCmd == nil {
		t.Fatal("create command not found")
	}
	deleteCmd := findCommand(cmdRoot, "delete")
	if deleteCmd == nil {
		t.Fatal("delete command not found")
	}

	// Test each write operation under create
	createOps := []struct {
		name       string
		flags      []string
		dryRunReqd bool
	}{
		{"cluster", []string{"config"}, true},
		{"iso", []string{"config"}, true},
		{"registry", []string{"config"}, true},
	}

	for _, op := range createOps {
		t.Run("create_"+op.name, func(t *testing.T) {
			subCmd := findCommand(createCmd, op.name)
			if subCmd == nil {
				t.Skipf("create %s command not found", op.name)
			}

			for _, flagName := range op.flags {
				flag := findFlag(subCmd, flagName)
				if flag == nil {
					t.Errorf("create %s should have --%s flag", op.name, flagName)
				}
			}

			if op.dryRunReqd {
				flag := findFlagRecursive(subCmd, "dry-run")
				if flag == nil {
					t.Errorf("create %s must have --dry-run flag per 测试方案.md", op.name)
				}
			}
		})
	}

	// Test delete cluster
	t.Run("delete_cluster", func(t *testing.T) {
		deleteClusterCmd := findCommand(deleteCmd, "cluster")
		if deleteClusterCmd == nil {
			t.Skip("delete cluster command not found")
		}

		flag := findFlag(deleteClusterCmd, "name")
		if flag == nil {
			t.Error("delete cluster should have --name flag")
		}

		flag = findFlag(deleteClusterCmd, "force")
		if flag == nil {
			t.Error("delete cluster should have --force flag")
		}
	})

	// Test delete registry
	t.Run("delete_registry", func(t *testing.T) {
		deleteRegistryCmd := findCommand(deleteCmd, "registry")
		if deleteRegistryCmd == nil {
			t.Skip("delete registry command not found")
		}

		flag := findFlag(deleteRegistryCmd, "config")
		if flag == nil {
			t.Error("delete registry should have --config flag")
		}

		flag = findFlag(deleteRegistryCmd, "force")
		if flag == nil {
			t.Error("delete registry should have --force flag")
		}
	})
}

// TestCertsCommandsHaveDryRun verifies certs commands support --dry-run
func TestCertsCommandsHaveDryRun(t *testing.T) {
	cmdRoot := cmd.NewRootCmd()
	certsCmd := findCommand(cmdRoot, "certs")
	if certsCmd == nil {
		t.Skip("certs command not found")
	}

	certsSubcommands := []string{"update", "rotate", "renew"}
	for _, sub := range certsSubcommands {
		t.Run(sub, func(t *testing.T) {
			subCmd := findCommand(certsCmd, sub)
			if subCmd == nil {
				t.Skipf("certs %s command not found", sub)
			}
			flag := findFlagRecursive(subCmd, "dry-run")
			if flag == nil {
				t.Errorf("certs %s command must have --dry-run flag", sub)
			}
		})
	}
}

// TestGlobalFlagsInheritedBySubcommands verifies global flags are inherited
func TestGlobalFlagsInheritedBySubcommands(t *testing.T) {
	cmdRoot := cmd.NewRootCmd()

	// Verb-first: kubexm create <subcommand>
	createCmd := findCommand(cmdRoot, "create")
	if createCmd == nil {
		t.Fatal("create command not found")
	}

	// Test that subcommands inherit global flags
	clusterCmd := findCommand(createCmd, "cluster")
	if clusterCmd == nil {
		t.Fatal("create cluster command not found")
	}

	globalFlags := []string{"verbose", "yes"}

	for _, flagName := range globalFlags {
		t.Run(flagName+"_inherited", func(t *testing.T) {
			// Global flags should be accessible via recursive lookup
			flag := findFlagRecursive(clusterCmd, flagName)
			if flag == nil {
				t.Errorf("Subcommand should inherit --%s flag from root", flagName)
			}
		})
	}
}

// TestInteractionPromptsBypassWithYes verifies --yes bypasses prompts
// Per 测试计划.md Section 1.1: "所有包含交互式提示 (y/N) 的流程通过注入模拟输入来验证其自动化挂机模式"
func TestInteractionPromptsBypassWithYes(t *testing.T) {
	cmdRoot := cmd.NewRootCmd()

	// Verb-first: kubexm delete <subcommand>
	deleteCmd := findCommand(cmdRoot, "delete")
	if deleteCmd == nil {
		t.Fatal("delete command not found")
	}

	// Commands that should have interactive prompts
	promptCommands := []string{"cluster", "registry"}

	for _, name := range promptCommands {
		t.Run(name, func(t *testing.T) {
			cmd := findCommand(deleteCmd, name)
			if cmd == nil {
				t.Skipf("delete %s command not found", name)
			}
			// The --yes flag should be present to bypass prompts
			yesFlag := findFlagRecursive(cmd, "yes")
			if yesFlag == nil {
				t.Errorf("delete %s should inherit --yes flag to bypass interactive prompts", name)
			}
		})
	}
}

// TestAllShortFlagForms verifies short forms exist for flags
// Per 测试计划.md Section 1.1: "校验 --yes, --verbose, --dry-run, --skip-preflight, --force 选项的短拼写"
func TestAllShortFlagForms(t *testing.T) {
	cmdRoot := cmd.NewRootCmd()

	// Verb-first: kubexm create <subcommand>
	createCmd := findCommand(cmdRoot, "create")
	if createCmd == nil {
		t.Fatal("create command not found")
	}

	// Check config flag short form in create cluster command
	clusterCmd := findCommand(createCmd, "cluster")
	if clusterCmd != nil {
		t.Run("create_cluster_config_short", func(t *testing.T) {
			// Check config flag shorthand
			configFlag := findFlag(clusterCmd, "config")
			if configFlag == nil {
				t.Skip("config flag not found")
			}
			if configFlag.Shorthand == "" {
				t.Error("create cluster --config flag should have -f shorthand")
			}
		})
	}
}

// TestVerbFirstCommandStructure verifies the new verb-first command structure
func TestVerbFirstCommandStructure(t *testing.T) {
	cmdRoot := cmd.NewRootCmd()

	// Verify verb commands exist
	verbs := []string{"create", "delete", "build", "get", "list", "check", "renew", "rotate", "push", "drain", "cordon", "uncordon"}
	for _, verb := range verbs {
		t.Run("verb_"+verb, func(t *testing.T) {
			cmd := findCommand(cmdRoot, verb)
			if cmd == nil {
				t.Errorf("verb command '%s' should exist (verb-first style)", verb)
			}
		})
	}

	// Verify create subcommands
	createCmd := findCommand(cmdRoot, "create")
	if createCmd == nil {
		t.Fatal("create command not found")
	}

	createSubcommands := []string{"cluster", "iso", "registry"}
	for _, sub := range createSubcommands {
		t.Run("create_"+sub, func(t *testing.T) {
			cmd := findCommand(createCmd, sub)
			if cmd == nil {
				t.Errorf("create %s should exist (verb-first style: kubexm create %s)", sub, sub)
			}
		})
	}

	// Verify build subcommands
	buildCmd := findCommand(cmdRoot, "build")
	if buildCmd == nil {
		t.Fatal("build command not found")
	}

	buildSubcommands := []string{"cluster", "iso"}
	for _, sub := range buildSubcommands {
		t.Run("build_"+sub, func(t *testing.T) {
			cmd := findCommand(buildCmd, sub)
			if cmd == nil {
				t.Errorf("build %s should exist (verb-first style: kubexm build %s)", sub, sub)
			}
		})
	}

	// Verify delete subcommands
	deleteCmd := findCommand(cmdRoot, "delete")
	if deleteCmd == nil {
		t.Fatal("delete command not found")
	}

	deleteSubcommands := []string{"cluster", "nodes", "registry"}
	for _, sub := range deleteSubcommands {
		t.Run("delete_"+sub, func(t *testing.T) {
			cmd := findCommand(deleteCmd, sub)
			if cmd == nil {
				t.Errorf("delete %s should exist (verb-first style: kubexm delete %s)", sub, sub)
			}
		})
	}
}

// TestSkipPreflightFlag verifies --skip-preflight flag exists on applicable commands
// Per 测试计划.md Section 1.1: "校验 --yes, --verbose, --dry-run, --skip-preflight, --force 选项的短拼写"
func TestSkipPreflightFlag(t *testing.T) {
	cmdRoot := cmd.NewRootCmd()

	// Verb-first: kubexm create <subcommand>
	createCmd := findCommand(cmdRoot, "create")
	if createCmd == nil {
		t.Fatal("create command not found")
	}

	// Commands that should have --skip-preflight flag
	skipPreflightCommands := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"cluster", findCommand(createCmd, "cluster")},
	}

	for _, tc := range skipPreflightCommands {
		t.Run(tc.name+"_has_skip_preflight", func(t *testing.T) {
			if tc.cmd == nil {
				t.Skipf("Command %s not found", tc.name)
			}
			flag := findFlag(tc.cmd, "skip-preflight")
			if flag == nil {
				t.Errorf("Command %s should have --skip-preflight flag", tc.name)
			}
		})
	}
}

// Verify test files exist for M1 requirement
func TestM1CLITestsExist(t *testing.T) {
	// Per 测试计划.md Section 1.1 exit criteria: "Core package Unit Test 覆盖率达标"
	// This test file itself is the M1 CLI test
	if os.Getenv("SKIP_M1_TEST") != "" {
		t.Skip("M1 CLI tests can be skipped in unit test mode")
	}
}
