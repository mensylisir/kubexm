package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// UserExists checks if a user exists on the remote system.
func (r *defaultRunner) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(username) == "" {
		return false, fmt.Errorf("username cannot be empty")
	}
	// Use `id -u` for a more reliable check if user exists.
	// `id <username>` might print info even if user doesn't exist but some other entity does.
	cmd := fmt.Sprintf("id -u %s", username)
	// Sudo is typically not required.
	// r.Check will return true if exit code is 0, false otherwise (unless Check itself errors).
	// `id -u <nonexistent_user>` typically exits 1.
	exists, err := r.Check(ctx, conn, cmd, false)
	if err != nil {
		// This means the Check command itself failed (e.g. `id` command not found, connection error)
		return false, fmt.Errorf("error checking user %s: %w", username, err)
	}
	return exists, nil
}

// GroupExists checks if a group exists on the remote system.
func (r *defaultRunner) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(groupname) == "" {
		return false, fmt.Errorf("groupname cannot be empty")
	}
	cmd := fmt.Sprintf("getent group %s", groupname)
	// Sudo is not required.
	exists, err := r.Check(ctx, conn, cmd, false)
	if err != nil {
		return false, fmt.Errorf("error checking group %s: %w", groupname, err)
	}
	return exists, nil
}

// AddUser adds a new user to the remote system.
func (r *defaultRunner) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("username cannot be empty for AddUser")
	}

	// Check for existence to ensure idempotency
	exists, err := r.UserExists(ctx, conn, username)
	if err != nil {
		return fmt.Errorf("failed to check if user %s exists before adding: %w", username, err)
	}
	if exists {
		return nil // User already exists
	}

	cmdParts := []string{"useradd"}
	if systemUser {
		cmdParts = append(cmdParts, "-r")
	}

	if createHome {
		cmdParts = append(cmdParts, "-m")
		if homeDir != "" {
			cmdParts = append(cmdParts, "-d", homeDir)
		}
	} else {
		cmdParts = append(cmdParts, "-M")
		if homeDir != "" {
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

	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		// Stderr is included in the error by RunWithOptions if it's a CommandError
		return fmt.Errorf("failed to add user %s: %w", username, err)
	}
	return nil
}

// AddGroup adds a new group to the remote system.
func (r *defaultRunner) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(groupname) == "" {
		return fmt.Errorf("groupname cannot be empty for AddGroup")
	}

	// Check for existence to ensure idempotency
	exists, err := r.GroupExists(ctx, conn, groupname)
	if err != nil {
		return fmt.Errorf("failed to check if group %s exists before adding: %w", groupname, err)
	}
	if exists {
		return nil // Group already exists
	}

	cmdParts := []string{"groupadd"}
	if systemGroup {
		cmdParts = append(cmdParts, "-r")
	}
	cmdParts = append(cmdParts, groupname)
	cmd := strings.Join(cmdParts, " ")

	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to add group %s: %w", groupname, err)
	}
	return nil
}
