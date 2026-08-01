package infrastructure_test

import (
	"embed"
	"strings"
	"testing"

	"stackyrd/pkg/infrastructure"
)

//go:embed testdata/config.yaml testdata/README.md testdata/test.txt
var testFS embed.FS

func TestAferoRead(t *testing.T) {
	aliasMap := map[string]string{
		"config": "all:testdata/config.yaml",
		"readme": "all:testdata/README.md",
		"test":   "all:testdata/test.txt",
	}
	infrastructure.Init(testFS, aliasMap, true)

	t.Run("Exists", func(t *testing.T) {
		if infrastructure.Exists("nonexistent") {
			t.Error("Expected 'nonexistent' alias to not exist")
		}
		if !infrastructure.Exists("config") {
			t.Error("Expected 'config' alias to exist")
		}
	})

	t.Run("Read", func(t *testing.T) {
		content, err := infrastructure.Read("test")
		if err != nil {
			t.Errorf("Expected to read file, got error: %v", err)
		}
		if !strings.Contains(string(content), "test content") {
			t.Errorf("Expected file content to contain 'test content', got: %s", string(content))
		}
	})

	t.Run("Stream", func(t *testing.T) {
		stream, err := infrastructure.Stream("test")
		if err != nil {
			t.Errorf("Expected to stream file, got error: %v", err)
		}
		if stream == nil {
			t.Error("Expected stream to be returned")
		}
		_ = stream.Close()
	})

	t.Run("ReadError", func(t *testing.T) {
		_, err := infrastructure.Read("nonexistent")
		if err == nil {
			t.Error("Expected error when reading non-existent alias")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}
	})
}
