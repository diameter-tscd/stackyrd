package utils

import (
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var (
	seededRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
	seededRandMu sync.Mutex
)

// RandomString generates a random string of the given length.
func RandomString(length int) string {
	b := make([]byte, length)
	seededRandMu.Lock()
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	seededRandMu.Unlock()
	return string(b)
}

// ToSnakeCase converts a string to snake_case.
func ToSnakeCase(s string) string {
	// Simple implementation; for production might need regex or more robust parsing
	// This replaces spaces and upper case transitions
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// ToCamelCase converts a generic string (snake_case or space separated) to CamelCase.
// Note: This is a basic implementation.
func ToCamelCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == ' ' || r == '-'
	})

	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, "")
}

// IsEmpty checks if a string is empty or contains only whitespace.
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// NewUUID generates a new random UUID (v4).
func NewUUID() string {
	return uuid.New().String()
}
