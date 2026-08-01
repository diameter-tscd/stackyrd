package utils

import (
	"crypto/rand"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandomString generates a cryptographically random string of the given length.
func RandomString(length int) string {
	if length < 0 {
		length = 0
	}
	b := make([]byte, length)
	limit := big.NewInt(int64(len(charset)))
	for i := range b {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			panic("crypto/rand failure: " + err.Error())
		}
		b[i] = charset[n.Int64()]
	}
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
		if len(part) == 0 {
			continue
		}
		// Work on runes so a multi-byte leading character isn't byte-split.
		runes := []rune(strings.ToLower(part))
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		parts[i] = string(runes)
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
