package util

import (
	"errors" // Added for errors.Unwrap
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDir(t *testing.T) {
	tmpDir := t.TempDir() // Using t.TempDir() for automatic cleanup

	t.Run("directory_does_not_exist", func(t *testing.T) {
		dirPath := filepath.Join(tmpDir, "newdir")
		err := CreateDir(dirPath)
		if err != nil {
			t.Errorf("CreateDir(%q) returned error: %v", dirPath, err)
		}
		stat, statErr := os.Stat(dirPath)
		if statErr != nil {
			t.Fatalf("os.Stat(%q) after CreateDir failed: %v", dirPath, statErr)
		}
		if !stat.IsDir() {
			t.Errorf("Path %q is not a directory after CreateDir", dirPath)
		}
		if runtime.GOOS != "windows" && stat.Mode().Perm() != 0755 {
			t.Errorf("Expected permissions 0755, got %s", stat.Mode().Perm().String())
		}
	})

	t.Run("directory_already_exists", func(t *testing.T) {
		dirPath := filepath.Join(tmpDir, "existingdir")
		if err := os.Mkdir(dirPath, 0755); err != nil { // Pre-create the directory
			t.Fatalf("Failed to pre-create directory %q: %v", dirPath, err)
		}
		err := CreateDir(dirPath)
		if err != nil {
			t.Errorf("CreateDir(%q) on existing directory returned error: %v", dirPath, err)
		}
		stat, statErr := os.Stat(dirPath)
		if statErr != nil {
			t.Fatalf("os.Stat(%q) on existing directory failed: %v", dirPath, statErr)
		}
		if !stat.IsDir() {
			t.Errorf("Path %q is not a directory", dirPath)
		}
	})

	t.Run("path_exists_but_is_a_file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "existingfile.txt")
		if _, err := os.Create(filePath); err != nil { // Pre-create the file
			t.Fatalf("Failed to pre-create file %q: %v", filePath, err)
		}
		defer os.Remove(filePath) // Clean up the file

		err := CreateDir(filePath)
		if err == nil {
			t.Errorf("CreateDir(%q) on an existing file expected an error, got nil", filePath)
		} else {
			expectedErrorSubstring := "exists but is not a directory"
			if !strings.Contains(err.Error(), expectedErrorSubstring) {
				t.Errorf("CreateDir(%q) error message = %q, want to contain %q", filePath, err.Error(), expectedErrorSubstring)
			}
		}
	})

	t.Run("create_nested_directories", func(t *testing.T) {
		nestedDirPath := filepath.Join(tmpDir, "parent", "child", "grandchild")
		err := CreateDir(nestedDirPath)
		if err != nil {
			t.Errorf("CreateDir(%q) for nested path returned error: %v", nestedDirPath, err)
		}
		stat, statErr := os.Stat(nestedDirPath)
		if statErr != nil {
			t.Fatalf("os.Stat(%q) after CreateDir for nested path failed: %v", nestedDirPath, statErr)
		}
		if !stat.IsDir() {
			t.Errorf("Path %q is not a directory after CreateDir for nested path", nestedDirPath)
		}
	})

	t.Run("parent_path_is_a_file_preventing_creation", func(t *testing.T) {
		parentAsFile := filepath.Join(tmpDir, "parentisfile.txt")
		if err := os.WriteFile(parentAsFile, []byte("i am a file"), 0644); err != nil {
			t.Fatalf("Failed to create file %q to act as parent: %v", parentAsFile, err)
		}
		defer os.Remove(parentAsFile)

		dirToCreate := filepath.Join(parentAsFile, "newdir") // Attempt to create .../parentisfile.txt/newdir
		err := CreateDir(dirToCreate)
		if err == nil {
			t.Errorf("CreateDir with parent as file (%q) expected error, got nil", dirToCreate)
		} else {
			// Error from MkdirAll when part of the path is a file is typically "not a directory" or similar.
			// The error is wrapped, so we check the wrapped error message.
			// On Linux, it's often "not a directory". On Windows, it might be different.
			// Let's check for common substrings.
			errMsgLower := strings.ToLower(err.Error())
			if !strings.Contains(errMsgLower, "not a directory") && !strings.Contains(errMsgLower, "is not a directory") && !strings.Contains(errMsgLower, "no such file or directory") {
				// "no such file or directory" can appear if MkdirAll tries to create parentisfile.txt/ and fails before newdir.
				t.Errorf("CreateDir error message = %q, expected to indicate parent path issue (e.g. 'not a directory')", err.Error())
			}
		}
	})

	t.Run("unwritable_parent_directory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping unwritable parent directory test on Windows due to permission complexities")
		}
		// Create a parent directory
		unwritableParentPath := filepath.Join(tmpDir, "unwritable_parent")
		if err := os.Mkdir(unwritableParentPath, 0755); err != nil {
			t.Fatalf("Failed to create base for unwritable_parent: %v", err)
		}

		// Change its permissions to be non-writable (read-only for owner)
		if err := os.Chmod(unwritableParentPath, 0555); err != nil {
			t.Fatalf("Failed to chmod unwritable_parent to 0555: %v", err)
		}
		// Defer chmod back to writable so RemoveAll can clean it up
		defer os.Chmod(unwritableParentPath, 0755)


		dirToCreateInUnwritable := filepath.Join(unwritableParentPath, "subdir_should_fail")
		err := CreateDir(dirToCreateInUnwritable)
		if err == nil {
			t.Errorf("CreateDir in unwritable parent %q expected an error, got nil", unwritableParentPath)
		} else {
			// Expect permission denied or similar
			if !os.IsPermission(errors.Unwrap(err)) && !os.IsPermission(err) { // Check wrapped and original
				// This check might be too specific. os.MkdirAll might return a different error type
				// if the underlying issue is permission on an intermediate path it tries to create.
				// Let's check for "permission denied" string for more robustness.
				if !strings.Contains(strings.ToLower(err.Error()), "permission denied") {
					t.Errorf("Expected permission error, got: %v", err)
				}
			}
		}
	})
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Case 1: File exists
	existingFilePath := filepath.Join(tmpDir, "exists.txt")
	f, err := os.Create(existingFilePath)
	require.NoError(t, err, "Failed to create test file")
	f.Close()

	exists, err := FileExists(existingFilePath)
	assert.NoError(t, err, "FileExists on existing file should not error")
	assert.True(t, exists, "FileExists should return true for an existing file")

	// Case 2: Directory exists
	existingDirPath := filepath.Join(tmpDir, "exists_dir")
	err = os.Mkdir(existingDirPath, 0755)
	require.NoError(t, err, "Failed to create test directory")

	exists, err = FileExists(existingDirPath)
	assert.NoError(t, err, "FileExists on existing directory should not error")
	assert.True(t, exists, "FileExists should return true for an existing directory")

	// Case 3: Path does not exist
	nonExistentPath := filepath.Join(tmpDir, "not_exists.txt")
	exists, err = FileExists(nonExistentPath)
	assert.NoError(t, err, "FileExists on non-existent path should not error")
	assert.False(t, exists, "FileExists should return false for a non-existent path")

	// Case 4: Empty path string (should be treated as non-existent or error depending on os.Stat)
	// os.Stat("") typically returns an error like "stat : no such file or directory" on non-Windows
	// or "GetFileAttributesEx : The system cannot find the file specified." on Windows.
	// So, it should be treated as os.IsNotExist or a wrapped error.
	exists, err = FileExists("")
	if err != nil { // Expecting an error or (false, nil)
		// If it's an error, it should ideally wrap os.ErrNotExist or similar
		// For now, let's check if exists is false.
		// The function FileExists is designed to return (false, nil) for IsNotExist.
		// An empty path os.Stat("") might return a different error that is *not* IsNotExist.
		// Let's test current behavior:
		assert.False(t, exists, "FileExists on empty path should result in exists=false")
		// We expect an error from os.Stat(""), which FileExists should wrap
		assert.Error(t, err, "FileExists on empty path should ideally return an error from os.Stat")
		assert.Contains(t, err.Error(), "error checking if path '' exists", "Error message for empty path mismatch")
	} else {
		// This path implies os.Stat("") returned nil error, which is highly unlikely.
		// Or, it means os.Stat("") returned an IsNotExist error, and FileExists returned (false, nil)
		assert.False(t, exists, "FileExists on empty path should return false if no error")
	}


	// Case 5: Permission denied on parent (simulated - hard to truly test without changing actual permissions)
	// This test is more about how FileExists handles generic os.Stat errors other than IsNotExist.
	// On Unix, if /unreadable_dir exists but is not readable, and we check /unreadable_dir/somefile:
	// os.Stat("/unreadable_dir/somefile") will return a permission error for "/unreadable_dir/somefile",
	// and os.IsPermission(err) would be true for that error.
	// FileExists should return (false, wrappedError).
	// For simplicity, we'll check that an error is returned if Stat fails unexpectedly.
	// This is already covered by the function's structure.
	// A more direct test would involve creating a directory, chmodding it to unreadable,
	// then trying FileExists on a path inside it.
	if runtime.GOOS != "windows" { // Permission tests are more reliable on POSIX
		unreadableParent := filepath.Join(tmpDir, "unreadable_parent_for_exists_test")
		err = os.Mkdir(unreadableParent, 0755)
		require.NoError(t, err)

		// Make parent unsearchable (no execute bit for others/group, if user is not owner)
		// To make it robust, make it unreadable/unsearchable by owner.
		err = os.Chmod(unreadableParent, 0000) // No permissions for anyone
		require.NoError(t, err)

		defer func() {
			// Attempt to restore permissions to allow cleanup by t.TempDir()
			_ = os.Chmod(unreadableParent, 0755)
		}()

		pathInUnreadable := filepath.Join(unreadableParent, "file_inside.txt")
		exists, errCheck := FileExists(pathInUnreadable)

		assert.False(t, exists, "Expected exists=false when parent is unreadable")
		assert.Error(t, errCheck, "Expected an error when parent directory is unreadable")
		if errCheck != nil {
			assert.True(t, os.IsPermission(errors.Unwrap(errCheck)) || os.IsPermission(errCheck) || strings.Contains(errCheck.Error(), "permission denied"),
				"Expected a permission-related error, got: %v", errCheck)
		}
	}
}
