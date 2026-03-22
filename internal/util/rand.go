package util

import (
	"math/rand"
	"time"
)

const (
	tokenIDChars     = "abcdefghijklmnopqrstuvwxyz0123456789"
	tokenSecretChars = "abcdefghijklmnopqrstuvwxyz0123456789"
)

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func GenerateRandomString(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func GenerateTokenID() string {
	return GenerateRandomString(6, tokenIDChars)
}

func GenerateTokenSecret() string {
	return GenerateRandomString(16, tokenSecretChars)
}