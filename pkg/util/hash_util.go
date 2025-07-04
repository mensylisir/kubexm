package util

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// ComputeFileChecksum calculates the checksum of a file using the specified algorithm.
// Supported algorithms: "md5", "sha256", "sha512".
// Returns the hex-encoded checksum string.
func ComputeFileChecksum(filePath string, algo string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s for checksum: %w", filePath, err)
	}
	defer file.Close()

	var h hash.Hash
	switch strings.ToLower(algo) {
	case "md5":
		h = md5.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	// Add other hash algorithms here if needed, e.g., sha1
	// case "sha1":
	//  h = sha1.New()
	default:
		return "", fmt.Errorf("unsupported checksum algorithm: %s. Supported: md5, sha256, sha512", algo)
	}

	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("failed to compute checksum for file %s: %w", filePath, err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
