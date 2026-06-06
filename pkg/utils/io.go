package utils

import (
	"fmt"
	"os"
)

// WriteFile writes content to a file, creating it if it doesn't exist.
// It overwrites the file if it exists.
func WriteFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0644)
}

// ReadFile reads the content of a file.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// AppendFile appends content to a file, creating it if it doesn't exist.
func AppendFile(path string, content []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for appending: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(content); err != nil {
		return fmt.Errorf("failed to append to file: %w", err)
	}
	return nil
}
