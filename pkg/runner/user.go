package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

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
		for _, g := range modifications.AppendToGroups {
			if strings.TrimSpace(g) == "" {
				return fmt.Errorf("group name in AppendToGroups cannot be empty")
			}
		}
		cmdParts = append(cmdParts, "-aG", strings.Join(modifications.AppendToGroups, ","))
		modifiedSomething = true
	}
	if len(modifications.SetSecondaryGroups) > 0 {
		for _, g := range modifications.SetSecondaryGroups {
			if strings.TrimSpace(g) == "" {
				return fmt.Errorf("group name in SetSecondaryGroups cannot be empty")
			}
		}
		cmdParts = append(cmdParts, "-G", strings.Join(modifications.SetSecondaryGroups, ","))
		modifiedSomething = true
	}

	if modifications.NewShell != nil {
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
		return fmt.Errorf("MoveHomeDirContents can only be true if NewHomeDir is also specified")
	}

	if modifications.NewComment != nil {
		cmdParts = append(cmdParts, "-c", *modifications.NewComment)
		modifiedSomething = true
	}

	if !modifiedSomething {
		return nil // No modifications requested
	}

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
	return nil
}

func (r *defaultRunner) ConfigureSudoer(ctx context.Context, conn connector.Connector, sudoerName, content string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(sudoerName) == "" {
		return fmt.Errorf("sudoerName cannot be empty")
	}
	if strings.ContainsAny(sudoerName, "/\\ !@#$%^&*()+={}|[]:;\"'<>,.?~`") {
		return fmt.Errorf("invalid characters in sudoerName: %s", sudoerName)
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("content for sudoer file cannot be empty")
	}

	finalSudoerPath := fmt.Sprintf("/etc/sudoers.d/%s", sudoerName)
	tempFileName := fmt.Sprintf("kubexm_sudoer_%s_%d", sudoerName, time.Now().UnixNano())
	remoteTempPath := fmt.Sprintf("/tmp/%s", tempFileName)

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fileStat, statErr := conn.Stat(cleanupCtx, remoteTempPath)
		if statErr == nil && fileStat != nil && fileStat.IsExist {
			r.Remove(cleanupCtx, conn, remoteTempPath, false, false)
		}
	}()

	err := r.WriteFile(ctx, conn, []byte(content), remoteTempPath, "0600", false)
	if err != nil {
		return fmt.Errorf("failed to write temporary sudoer content to %s: %w", remoteTempPath, err)
	}
	visudoCmd := fmt.Sprintf("visudo -cf %s", remoteTempPath)
	_, visudoStderr, visudoErr := r.RunWithOptions(ctx, conn, visudoCmd, &connector.ExecOptions{Sudo: true})
	if visudoErr != nil {
		return fmt.Errorf("sudoer content validation failed for %s using 'visudo -cf': %w (stderr: %s)", remoteTempPath, visudoErr, string(visudoStderr))
	}

	sudoersDDir := "/etc/sudoers.d"
	if errMkdir := r.Mkdirp(ctx, conn, sudoersDDir, "0755", true); errMkdir != nil {
		return fmt.Errorf("failed to ensure sudoers.d directory %s exists: %w", sudoersDDir, errMkdir)
	}

	mvCmd := fmt.Sprintf("mv %s %s", remoteTempPath, finalSudoerPath)
	_, mvStderr, mvErr := r.RunWithOptions(ctx, conn, mvCmd, &connector.ExecOptions{Sudo: true})
	if mvErr != nil {
		return fmt.Errorf("failed to move sudoer file from %s to %s: %w (stderr: %s)", remoteTempPath, finalSudoerPath, mvErr, string(mvStderr))
	}

	if errChmod := r.Chmod(ctx, conn, finalSudoerPath, "0440", true); errChmod != nil {
		return fmt.Errorf("failed to set permissions on %s: %w", finalSudoerPath, errChmod)
	}
	if errChown := r.Chown(ctx, conn, finalSudoerPath, "root", "root", false); errChown != nil {
		return fmt.Errorf("failed to set ownership on %s: %w", finalSudoerPath, errChown)
	}

	return nil
}

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

	chpasswdInput := fmt.Sprintf("%s:%s", username, hashedPassword)
	escapedInput := chpasswdInput

	cmd := fmt.Sprintf("echo %s | chpasswd", escapedInput)

	opts := &connector.ExecOptions{
		Sudo:   true,
		Hidden: true,
	}

	_, stderr, execErr := r.RunWithOptions(ctx, conn, cmd, opts)
	if execErr != nil {
		return fmt.Errorf("failed to set password for user %s using chpasswd: %w (stderr: %s)", username, execErr, string(stderr))
	}
	return nil
}

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
	if err != nil {
		return nil, fmt.Errorf("failed to get UID for user %s: %w", username, err)
	}
	info.UID = strings.TrimSpace(uidStr)

	gidStr, err := r.Run(ctx, conn, fmt.Sprintf("id -g %s", username), false)
	if err != nil {
		return nil, fmt.Errorf("failed to get GID for user %s: %w", username, err)
	}
	info.GID = strings.TrimSpace(gidStr)

	getentOut, err := r.Run(ctx, conn, fmt.Sprintf("getent passwd %s", username), false)
	if err != nil {
		return nil, fmt.Errorf("failed to get passwd entry for user %s: %w", username, err)
	}
	passwdFields := strings.Split(strings.TrimSpace(getentOut), ":")
	if len(passwdFields) >= 7 {
		info.Comment = passwdFields[4]
		info.HomeDir = passwdFields[5]
		info.Shell = passwdFields[6]
	} else {
		return nil, fmt.Errorf("unexpected format from 'getent passwd %s': %s", username, getentOut)
	}

	groupsStr, err := r.Run(ctx, conn, fmt.Sprintf("id -Gn %s", username), false)
	if err != nil {
		return nil, fmt.Errorf("failed to get group names for user %s using 'id -Gn': %w", username, err)
	}
	info.Groups = strings.Fields(strings.TrimSpace(groupsStr))

	return info, nil
}
