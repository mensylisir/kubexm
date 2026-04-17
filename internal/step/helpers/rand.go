package helpers

import (
	"math/rand"
	"sync"
	"time"
)

const (
	tokenIDChars     = "abcdefghijklmnopqrstuvwxyz0123456789"
	tokenSecretChars = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// seededRand is protected by randMu to ensure thread-safe access.
// math/rand.Rand is not safe for concurrent use.
var (
	seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	randMu     sync.Mutex
)

func GenerateRandomString(length int, charset string) string {
	b := make([]byte, length)
	randMu.Lock()
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	randMu.Unlock()
	return string(b)
}

func GenerateTokenID() string {
	return GenerateRandomString(6, tokenIDChars)
}

func GenerateTokenSecret() string {
	return GenerateRandomString(16, tokenSecretChars)
}
