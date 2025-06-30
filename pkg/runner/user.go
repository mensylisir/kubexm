package runner

import (
	"context"
	"fmt"
	"strings"
	"time" // For temporary filename generation

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
	cmd := fmt.Sprintf("id -u %s", username)
	exists, err := r.Check(ctx, conn, cmd, false)
	if err != nil {
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

	_, _, err = r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
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

	_, _, err = r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to add group %s: %w", groupname, err)
	}
	return nil
}

// --- Stubs for new user/permission methods from enriched interface ---

// ModifyUser modifies existing user attributes.
func (r *defaultRunner) ModifyUser(ctx context.Context, conn connector.Connector, username string, modifications UserModifications) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for ModifyUser")
	}
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("username cannot be empty for ModifyUser")
	}

	exists, err := r.UserExists(ctx, conn, username)
	if err != nil {
		return fmt.Errorf("failed to check if user %s exists before modifying: %w", username, err)
	}
	if !exists {
		return fmt.Errorf("user %s does not exist, cannot modify", username)
	}

	cmdParts := []string{"usermod"}
	modifiedSomething := false

	if modifications.NewUsername != nil {
		if strings.TrimSpace(*modifications.NewUsername) == "" {
			return fmt.Errorf("new username cannot be empty if provided")
		}
		cmdParts = append(cmdParts, "-l", *modifications.NewUsername)
		modifiedSomething = true
	}
	if modifications.NewPrimaryGroup != nil {
		if strings.TrimSpace(*modifications.NewPrimaryGroup) == "" {
			return fmt.Errorf("new primary group cannot be empty if provided")
		}
		cmdParts = append(cmdParts, "-g", *modifications.NewPrimaryGroup)
		modifiedSomething = true
	}
	if len(modifications.AppendToGroups) > 0 {
		// usermod -aG group1,group2,... user
		// Ensure no empty group names in the list
		for _, g := range modifications.AppendToGroups {
			if strings.TrimSpace(g) == "" {
				return fmt.Errorf("group name in AppendToGroups cannot be empty")
			}
		}
		cmdParts = append(cmdParts, "-aG", strings.Join(modifications.AppendToGroups, ","))
		modifiedSomething = true
	}
	if len(modifications.SetSecondaryGroups) > 0 {
		// usermod -G group1,group2,... user (replaces all secondary)
		// Ensure no empty group names in the list
		for _, g := range modifications.SetSecondaryGroups {
			if strings.TrimSpace(g) == "" {
				return fmt.Errorf("group name in SetSecondaryGroups cannot be empty")
			}
		}
		cmdParts = append(cmdParts, "-G", strings.Join(modifications.SetSecondaryGroups, ","))
		modifiedSomething = true
	}

	if modifications.NewShell != nil {
		// Empty shell might be valid (e.g. /sbin/nologin) but usually it's a path.
		// For now, allow empty if explicitly set via pointer.
		cmdParts = append(cmdParts, "-s", *modifications.NewShell)
		modifiedSomething = true
	}
	if modifications.NewHomeDir != nil {
		if strings.TrimSpace(*modifications.NewHomeDir) == "" {
			return fmt.Errorf("new home directory cannot be empty if provided")
		}
		cmdParts = append(cmdParts, "-d", *modifications.NewHomeDir)
		if modifications.MoveHomeDirContents {
			cmdParts = append(cmdParts, "-m")
		}
		modifiedSomething = true
	} else if modifications.MoveHomeDirContents {
		// -m without -d is usually an error or has specific meaning (create if not exist, but that's for useradd)
		return fmt.Errorf("MoveHomeDirContents can only be true if NewHomeDir is also specified")
	}

	if modifications.NewComment != nil {
		// If the comment contains spaces or shell-sensitive characters, it should be quoted
		// to be treated as a single argument by the shell that `conn.Exec` might invoke.
		cmdParts = append(cmdParts, "-c", shellEscape(*modifications.NewComment))
		modifiedSomething = true
	}

	if !modifiedSomething {
		return nil // No modifications requested
	}

	// The username to act upon is the original username, unless NewUsername is set,
	// in which case usermod applies changes and then renames.
	// If NewUsername is specified, it's the target of the rename, not the user being looked up initially.
	// The username argument to usermod should always be the current name of the user.
	cmdParts = append(cmdParts, username)
	cmd := strings.Join(cmdParts, " ")

	_, stderr, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		finalUsername := username
		if modifications.NewUsername != nil {
			finalUsername = *modifications.NewUsername
		}
		return fmt.Errorf("failed to modify user %s (to %s if renamed): %w (stderr: %s)", username, finalUsername, execErr, string(stderr))
	}

	// If username was changed, the original username variable for subsequent operations (if any) should be updated.
	// Not an issue for this single call method.
	return nil
}

func (r *defaultRunner) ConfigureSudoer(ctx context.Context, conn connector.Connector, sudoerName, content string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(sudoerName) == "" {
		return fmt.Errorf("sudoerName cannot be empty")
	}
	// Validate sudoerName: should be a simple filename, no slashes, no spaces, etc.
	// This is a basic validation.
	if strings.ContainsAny(sudoerName, "/\\ !@#$%^&*()+={}|[]:;\"'<>,.?~`") {
		return fmt.Errorf("invalid characters in sudoerName: %s", sudoerName)
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("content for sudoer file cannot be empty")
	}

	// Path for the final sudoer file
	finalSudoerPath := fmt.Sprintf("/etc/sudoers.d/%s", sudoerName)

	// Create a temporary file path on the remote system
	// Note: Facts might provide a better temp directory, but /tmp is common.
	// The filename should be unique enough to avoid collisions if multiple operations run.
	// For connector-based operations, this temp file is on the *remote* host.
	// We'll use WriteFile to write it to a user-writable path first, then sudo mv.

	// Simplified: Assume WriteFile can write to a standard user temp location if sudo=false for it.
	// Or, we write it via WriteFile with sudo to a root-owned temp, then visudo, then mv.
	// Let's try writing to /tmp with a unique name, no sudo initially for the temp file content.

	// A safer approach for temp file handling on remote:
	// 1. Generate a unique temp file name.
	// 2. Use `mktemp` on remote to create it securely, get its path.
	// 3. Write content to it.
	// 4. `visudo -cf <remote_temp_path>`
	// 5. `sudo mv <remote_temp_path> <final_path>`
	// 6. `sudo chmod/chown <final_path>`
	// 7. `sudo rm -f <remote_temp_path>` (if mv didn't consume it, or if visudo failed)

	// Simpler path for now: WriteFile to a /tmp location (no sudo for this write), then visudo, then sudo mv.
	// This assumes /tmp is generally writable.
	tempFileName := fmt.Sprintf("kubexm_sudoer_%s_%d", sudoerName, time.Now().UnixNano())
	remoteTempPath := fmt.Sprintf("/tmp/%s", tempFileName) // Using /tmp as a common writable directory

	defer func() {
		// Ensure temporary file is cleaned up regardless of success/failure of visudo or mv
		// Use a background context for cleanup to avoid issues if main ctx is cancelled.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Remove with sudo just in case, though it should be user-owned if written without sudo.
		// If WriteFile to temp was with sudo, then sudo for remove is definitely needed.
		// Let's assume WriteFile to temp does not use sudo.
		// If visudo fails, we remove. If mv succeeds, temp file is gone.
		// This defer is a fallback.
		fileStat, statErr := conn.Stat(cleanupCtx, remoteTempPath)
		if statErr == nil && fileStat != nil && fileStat.IsExist { // Check error first, then filestat and its IsExist field
			r.Remove(cleanupCtx, conn, remoteTempPath, false) // Try non-sudo first for /tmp file
		}
	}()

	// 1. Write content to temporary file (no sudo for this step, assuming /tmp is writable)
	// Permissions for temp file are not critical as it's checked by visudo and then moved. 0600 is good practice.
	err := r.WriteFile(ctx, conn, []byte(content), remoteTempPath, "0600", false)
	if err != nil {
		return fmt.Errorf("failed to write temporary sudoer content to %s: %w", remoteTempPath, err)
	}

	// 2. Validate syntax with visudo -cf
	// visudo needs to read the temp file. It doesn't need sudo to read a /tmp file.
	// However, `visudo` itself often requires sudo to run, even in check mode, depending on system config.
	// Let's assume visudo -cf needs sudo to operate correctly or to access its own required files.
	visudoCmd := fmt.Sprintf("visudo -cf %s", shellEscape(remoteTempPath))
	_, visudoStderr, visudoErr := r.RunWithOptions(ctx, conn, visudoCmd, &connector.ExecOptions{Sudo: true})
	if visudoErr != nil {
		// If visudo fails, the content is bad. Temp file will be cleaned by defer.
		return fmt.Errorf("sudoer content validation failed for %s using 'visudo -cf': %w (stderr: %s)", remoteTempPath, visudoErr, string(visudoStderr))
	}

	// 3. Ensure /etc/sudoers.d directory exists
	sudoersDDir := "/etc/sudoers.d"
	if errMkdir := r.Mkdirp(ctx, conn, sudoersDDir, "0755", true); errMkdir != nil {
		return fmt.Errorf("failed to ensure sudoers.d directory %s exists: %w", sudoersDDir, errMkdir)
	}

	// 4. Move temporary file to final destination with sudo
	mvCmd := fmt.Sprintf("mv %s %s", shellEscape(remoteTempPath), shellEscape(finalSudoerPath))
	_, mvStderr, mvErr := r.RunWithOptions(ctx, conn, mvCmd, &connector.ExecOptions{Sudo: true})
	if mvErr != nil {
		// Attempt to clean up finalSudoerPath if mv failed but left a partial/incorrect file,
		// though mv is usually atomic. More importantly, temp file is cleaned by defer.
		return fmt.Errorf("failed to move sudoer file from %s to %s: %w (stderr: %s)", remoteTempPath, finalSudoerPath, mvErr, string(mvStderr))
	}

	// 5. Set correct permissions and ownership for the final file
	if errChmod := r.Chmod(ctx, conn, finalSudoerPath, "0440", true); errChmod != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", finalSudoerPath, errChmod)
	}
	if errChown := r.Chown(ctx, conn, finalSudoerPath, "root", "root", false); errChown != nil {
		return fmt.Errorf("failed to set ownership on %s: %w", finalSudoerPath, errChown)
	}

	return nil // Temp file is removed by mv, or by defer if mv fails before that.
}

// SetUserPassword sets a user's password using a pre-hashed value.
// The hashedPassword must be in the format expected by `chpasswd` (e.g., SHA512 crypt string).
func (r *defaultRunner) SetUserPassword(ctx context.Context, conn connector.Connector, username, hashedPassword string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for SetUserPassword")
	}
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("username cannot be empty for SetUserPassword")
	}
	if strings.TrimSpace(hashedPassword) == "" {
		return fmt.Errorf("hashedPassword cannot be empty for SetUserPassword")
	}

	exists, err := r.UserExists(ctx, conn, username)
	if err != nil {
		return fmt.Errorf("failed to check if user %s exists before setting password: %w", username, err)
	}
	if !exists {
		return fmt.Errorf("user %s does not exist, cannot set password", username)
	}

	// Construct the input for chpasswd
	chpasswdInput := fmt.Sprintf("%s:%s", username, hashedPassword)
	// Escape the input for the echo command to handle special characters in username or hash safely
	escapedInput := shellEscape(chpasswdInput) // shellEscape is from file.go

	// Command: echo 'username:hashed_password' | chpasswd
	// The sudo applies to chpasswd.
	cmd := fmt.Sprintf("echo %s | chpasswd", escapedInput)


	opts := &connector.ExecOptions{
		Sudo:   true,
		Hidden: true, // Hide this command from logs as it contains password info (even if hashed)
	}

	_, stderr, execErr := r.RunWithOptions(ctx, conn, cmd, opts)
	if execErr != nil {
		return fmt.Errorf("failed to set password for user %s using chpasswd: %w (stderr: %s)", username, execErr, string(stderr))
	}
	return nil
}

// GetUserInfo retrieves detailed information about a user.
func (r *defaultRunner) GetUserInfo(ctx context.Context, conn connector.Connector, username string) (*UserInfo, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil for GetUserInfo")
	}
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("username cannot be empty for GetUserInfo")
	}

	exists, err := r.UserExists(ctx, conn, username)
	if err != nil {
		return nil, fmt.Errorf("failed to check if user %s exists: %w", username, err)
	}
	if !exists {
		return nil, fmt.Errorf("user %s does not exist", username)
	}

	info := &UserInfo{Username: username}

	uidStr, err := r.Run(ctx, conn, fmt.Sprintf("id -u %s", username), false)
	if err != nil { return nil, fmt.Errorf("failed to get UID for user %s: %w", username, err) }
	info.UID = strings.TrimSpace(uidStr)

	gidStr, err := r.Run(ctx, conn, fmt.Sprintf("id -g %s", username), false)
	if err != nil { return nil, fmt.Errorf("failed to get GID for user %s: %w", username, err) }
	info.GID = strings.TrimSpace(gidStr)

	getentOut, err := r.Run(ctx, conn, fmt.Sprintf("getent passwd %s", username), false)
	if err != nil { return nil, fmt.Errorf("failed to get passwd entry for user %s: %w", username, err) }
	passwdFields := strings.Split(strings.TrimSpace(getentOut), ":")
	if len(passwdFields) >= 7 {
		info.Comment = passwdFields[4]
		info.HomeDir = passwdFields[5]
		info.Shell = passwdFields[6]
	} else { return nil, fmt.Errorf("unexpected format from 'getent passwd %s': %s", username, getentOut) }

	groupsStr, err := r.Run(ctx, conn, fmt.Sprintf("id -Gn %s", username), false)
	if err != nil { return nil, fmt.Errorf("failed to get group names for user %s using 'id -Gn': %w", username, err) }
	info.Groups = strings.Fields(strings.TrimSpace(groupsStr))

	return info, nil
}
