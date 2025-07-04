package util

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"errors" // Added for errors.Unwrap

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper function to calculate checksum for a byte slice, for test comparison
func calculateChecksum(data []byte, algo string) string {
	var h []byte
	switch algo {
	case "md5":
		sum := md5.Sum(data)
		h = sum[:]
	case "sha256":
		sum := sha256.Sum256(data)
		h = sum[:]
	case "sha512":
		sum := sha512.Sum512(data)
		h = sum[:]
	default:
		return "" // Should not happen in test if algo is controlled
	}
	return hex.EncodeToString(h)
}

func TestComputeFileChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	testFilePath := filepath.Join(tmpDir, "checksum_test_file.txt")
	testData := []byte("This is test data for checksum calculation.")

	err := os.WriteFile(testFilePath, testData, 0644)
	require.NoError(t, err, "Failed to write test file")

	t.Run("MD5", func(t *testing.T) {
		expectedChecksum := calculateChecksum(testData, "md5")
		actualChecksum, err := ComputeFileChecksum(testFilePath, "md5")
		require.NoError(t, err)
		assert.Equal(t, expectedChecksum, actualChecksum)

		actualChecksumUpper, errUpper := ComputeFileChecksum(testFilePath, "MD5") // Test case insensitivity
		require.NoError(t, errUpper)
		assert.Equal(t, expectedChecksum, actualChecksumUpper)
	})

	t.Run("SHA256", func(t *testing.T) {
		expectedChecksum := calculateChecksum(testData, "sha256")
		actualChecksum, err := ComputeFileChecksum(testFilePath, "sha256")
		require.NoError(t, err)
		assert.Equal(t, expectedChecksum, actualChecksum)

		actualChecksumUpper, errUpper := ComputeFileChecksum(testFilePath, "SHA256")
		require.NoError(t, errUpper)
		assert.Equal(t, expectedChecksum, actualChecksumUpper)
	})

	t.Run("SHA512", func(t *testing.T) {
		expectedChecksum := calculateChecksum(testData, "sha512")
		actualChecksum, err := ComputeFileChecksum(testFilePath, "sha512")
		require.NoError(t, err)
		assert.Equal(t, expectedChecksum, actualChecksum)

		actualChecksumUpper, errUpper := ComputeFileChecksum(testFilePath, "SHA512")
		require.NoError(t, errUpper)
		assert.Equal(t, expectedChecksum, actualChecksumUpper)
	})


	t.Run("FileNotFound", func(t *testing.T) {
		_, err := ComputeFileChecksum(filepath.Join(tmpDir, "nonexistentfile.txt"), "sha256")
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err) || os.IsNotExist(errors.Unwrap(err)), "Expected os.IsNotExist error for non-existent file")
		assert.Contains(t, err.Error(), "failed to open file")
	})

	t.Run("UnsupportedAlgorithm", func(t *testing.T) {
		_, err := ComputeFileChecksum(testFilePath, "sha1") // Assuming sha1 is not added yet
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported checksum algorithm: sha1")
	})

	t.Run("EmptyFile", func(t *testing.T) {
		emptyFilePath := filepath.Join(tmpDir, "empty_checksum_file.txt")
		err := os.WriteFile(emptyFilePath, []byte{}, 0644)
		require.NoError(t, err)

		// SHA256 of empty string
		expectedChecksum := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		actualChecksum, err := ComputeFileChecksum(emptyFilePath, "sha256")
		require.NoError(t, err)
		assert.Equal(t, expectedChecksum, actualChecksum)
	})
}
