package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubexms/kubexms/pkg/connector" // Assuming this is the correct path
)

// UserExists checks if a user exists on the remote system.
func (r *Runner) UserExists(ctx context.Context, username string) (bool, error) {
	if r.Conn == nil {
		return false, fmt.Errorf("runner has no valid connector")
	}
	if strings.TrimSpace(username) == "" {
		return false, fmt.Errorf("username cannot be empty")
	}
	// `id -u <username>` is a common way to check. It exits 0 if user exists, non-zero otherwise.
	// Redirecting stderr to /dev/null to suppress "no such user" messages from the output of Check.
	// Check itself handles the interpretation of exit codes.
	cmd := fmt.Sprintf("id -u %s > /dev/null 2>&1", username)
	// Sudo is typically not required to check if a user exists.
	return r.Check(ctx, cmd, false)
}

// GroupExists checks if a group exists on the remote system.
func (r *Runner) GroupExists(ctx context.Context, groupname string) (bool, error) {
	if r.Conn == nil {
		return false, fmt.Errorf("runner has no valid connector")
	}
	if strings.TrimSpace(groupname) == "" {
		return false, fmt.Errorf("groupname cannot be empty")
	}
	// `getent group <groupname>` is a reliable way. Exits 0 if found.
	cmd := fmt.Sprintf("getent group %s > /dev/null 2>&1", groupname)
	// Sudo is not required.
	return r.Check(ctx, cmd, false)
}

// AddUser adds a new user to the remote system.
// Common options:
// - username: The name of the user.
// - group: The primary group for the user. If empty, often defaults (e.g. to a group named after user).
// - shell: The login shell for the user. If empty, often defaults (e.g. /bin/sh or /bin/bash).
// This function assumes useradd command is available.
func (r *Runner) AddUser(ctx context.Context, username, group, shell string, homeDir string, createHome bool, systemUser bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("username cannot be empty for AddUser")
	}

	cmdParts := []string{"useradd"}
	if systemUser {
		cmdParts = append(cmdParts, "-r") // Create a system user
	}

	if createHome {
		cmdParts = append(cmdParts, "-m") // Create home directory
		if homeDir != "" {
			cmdParts = append(cmdParts, "-d", homeDir)
		}
	} else {
		cmdParts = append(cmdParts, "-M") // Do not create home directory
		if homeDir != "" { // Still possible to specify homedir path even if not creating it
			cmdParts = append(cmdParts, "-d", homeDir)
		}
	}


	if group != "" {
		cmdParts = append(cmdParts, "-g", group)
	}
	if shell != "" {
		cmdParts = append(cmdParts, "-s", shell)
	}

	cmdParts = append(cmdParts, username)
	cmd := strings.Join(cmdParts, " ")

	// Adding a user almost always requires sudo.
	_, stderr, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to add user %s: %w (stderr: %s)", username, err, string(stderr))
	}
	return nil
}

// AddGroup adds a new group to the remote system.
// This function assumes groupadd command is available.
func (r *Runner) AddGroup(ctx context.Context, groupname string, systemGroup bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if strings.TrimSpace(groupname) == "" {
		return fmt.Errorf("groupname cannot be empty for AddGroup")
	}

	cmdParts := []string{"groupadd"}
	if systemGroup {
		cmdParts = append(cmdParts, "-r") // Create a system group
	}
	cmdParts = append(cmdParts, groupname)
	cmd := strings.Join(cmdParts, " ")

	// Adding a group almost always requires sudo.
	_, stderr, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to add group %s: %w (stderr: %s)", groupname, err, string(stderr))
	}
	return nil
}

// TODO: Implement other user/group management functions as needed:
// - DeleteUser(ctx context.Context, username string, removeHome bool) error
// - DeleteGroup(ctx context.Context, groupname string) error
// - AddUserToGroup(ctx context.Context, username, groupname string) error
// - SetUserPassword(ctx context.Context, username, hashedPassword string) error
// - GetUserUID(ctx context.Context, username string) (string, error)
// - GetUserGID(ctx context.Context, username string) (string, error)
