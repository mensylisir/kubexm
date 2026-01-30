# AGENTS.md - Connector Package

**Generated:** 2026-01-22
**Commit:** kubexm codebase

## OVERVIEW
Abstraction layer for remote host connections (SSH) and local execution. Provides `Connector` interface for command execution, file transfer, and OS detection. Uses factory pattern to create SSH or local connectors based on host address.

## STRUCTURE
```
pkg/connector/
├── interface.go           # Connector, Host, Factory interfaces + types
├── factory.go             # Factory implementation (SSH vs Local selection)
├── local.go               # LocalConnector (local execution, no SSH)
├── host_impl.go           # Host implementation (host abstraction)
├── errors.go              # CommandError, ConnectionError types
└── *_test.go              # Test files
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| **Remote execution** | `interface.go` | `Connector.Exec()`, `ExecOptions` |
| **File transfer** | `interface.go` | `Upload()`, `Download()`, `CopyContent()` |
| **SSH connection** | `factory.go` | `NewSSHConnector()`, bastion/proxy support |
| **Local operations** | `local.go` | `LocalConnector` - no SSH needed |
| **Host abstraction** | `host_impl.go` | `Host` interface with address, user, roles |
| **Error handling** | `errors.go` | `CommandError` with exit codes |

## CONVENTIONS
- **Interface-first design**: Define `Connector`, `Host`, `Factory` interfaces first
- **Factory pattern**: `NewFactory()` returns `Factory` interface
- **SSH/Local detection**: `NewConnectorForHost()` checks "localhost" or "127.0.0.1"
- **Context-based operations**: All methods accept `context.Context`
- **Options structs**: Behavior controlled via `*Options` pointers (nullable)
- **Error wrapping**: Use `fmt.Errorf("...: %w", err)` pattern
- **Interface validation**: `var _ Connector = (*LocalConnector)(nil)` compile-time check

## ANTI-PATTERNS
- **Don't expose SSH details** → Hide in `ConnectionCfg`, use `BastionCfg`/`ProxyCfg`
- **Don't hardcode shell commands** → Use `shellEscape()` for safety
- **Don't ignore sudo passwords** → Pass via stdin for sudo commands
- **Don't skip context cancellation** → Use `exec.CommandContext()` for Exec
- **Don't use global state** → Pass logger via `logger.Get()` singleton

## UNIQUE STYLES
- **Dual connector**: SSH for remote hosts, LocalConnector for localhost
- **Bastion host support**: `BastionCfg` for jump host connections
- **Base64 private keys**: Keys stored encoded in configs
- **Command retry logic**: `ExecOptions.Retries` with configurable delay
- **Streaming output**: `ExecOptions.Stream` for real-time command output
- **Cross-platform OS detection**: Linux/Darwin/Windows support in `GetOS()`
- **Sudo file operations**: Staged writes via temp files for privilege escalation

## NOTES
- `Connector` interface has 30+ methods - use convenience aliases (`Run`, `Read`, `Write`)
- `Host` interface uses getter/setter pattern (not struct fields)
- SSH uses `golang.org/x/crypto/ssh` package
- LocalConnector uses native `os`/`exec` packages
- All file operations support sudo via temporary file staging
- `CommandError` captures stdout/stderr/exit code for debugging
