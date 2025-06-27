package util

import (
	"errors" // Added for errors.Unwrap
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
