package infrastructure_test

import (
	"embed"
	"strings"
	"testing"

	"github.com/diameter-tscd/stackyrd/pkg/infrastructure"
)

//go:embed testdata/config.yaml testdata/README.md testdata/test.txt
var testFS embed.FS

func TestAferoManager(t *testing.T) {
	// Test alias configuration
	aliasMap := map[string]string{
		"config": "all:testdata/config.yaml",
		"readme": "all:testdata/README.md",
		"test":   "all:testdata/test.txt",
	}

	// Test initialization
	t.Run("Init", func(t *testing.T) {
		infrastructure.ResetForTesting()
		infrastructure.Init(testFS, aliasMap, true)

		if infrastructure.GetFileSystem() == nil {
			t.Fatal("Expected filesystem to be initialized")
		}

		if len(infrastructure.GetAliases()) != 3 {
			t.Errorf("Expected 3 aliases, got %d", len(infrastructure.GetAliases()))
		}
	})

	// Test Exists function
	t.Run("Exists", func(t *testing.T) {
		// Reset and re-initialize for this test to ensure singleton is set
		infrastructure.ResetForTesting()
		infrastructure.Init(testFS, aliasMap, true)

		// Debug: Check if aliases are set
		aliases := infrastructure.GetAliases()
		t.Logf("Aliases after Init: %v", aliases)

		// Debug: Check if filesystem is set
		fs := infrastructure.GetFileSystem()
		t.Logf("Filesystem is nil: %v", fs == nil)

		// Test non-existing alias
		if infrastructure.Exists("nonexistent") {
			t.Error("Expected 'nonexistent' alias to not exist")
		}

		// Test existing alias
		if !infrastructure.Exists("config") {
			t.Error("Expected 'config' alias to exist")
		}
	})

	// Test GetAliases function
	t.Run("GetAliases", func(t *testing.T) {
		aliases := infrastructure.GetAliases()
		if len(aliases) != 3 {
			t.Errorf("Expected 3 aliases, got %d. Aliases: %v", len(aliases), aliases)
		}

		if aliases["config"] != "all:testdata/config.yaml" {
			t.Errorf("Expected config alias to be 'all:testdata/config.yaml', got %s", aliases["config"])
		}

		if aliases["readme"] != "all:testdata/README.md" {
			t.Errorf("Expected readme alias to be 'all:testdata/README.md', got %s", aliases["readme"])
		}

		if aliases["test"] != "all:testdata/test.txt" {
			t.Errorf("Expected test alias to be 'all:testdata/test.txt', got %s", aliases["test"])
		}
	})

	// Test GetFileSystem function
	t.Run("GetFileSystem", func(t *testing.T) {
		fs := infrastructure.GetFileSystem()
		if fs == nil {
			t.Error("Expected filesystem to be returned")
		}
	})

	// Test Read function
	t.Run("Read", func(t *testing.T) {
		content, err := infrastructure.Read("test")
		if err != nil {
			t.Errorf("Expected to read file, got error: %v", err)
		}
		if !strings.Contains(string(content), "test content") {
			t.Errorf("Expected file content to contain 'test content', got: %s", string(content))
		}
	})

	// Test Stream function
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

	// Test development mode (CopyOnWriteFs)
	t.Run("DevelopmentMode", func(t *testing.T) {
		// Reset and re-initialize in development mode
		infrastructure.ResetForTesting()
		infrastructure.Init(testFS, aliasMap, true)

		// Should be CopyOnWriteFs in development mode
		fs := infrastructure.GetFileSystem()
		if fs == nil {
			t.Error("Expected filesystem to be initialized")
		}
	})

	// Test production mode (ReadOnlyFs)
	t.Run("ProductionMode", func(t *testing.T) {
		// Reset and re-initialize in production mode
		infrastructure.ResetForTesting()
		aliasMap := map[string]string{
			"test": "all:testdata/test.txt",
		}
		infrastructure.Init(testFS, aliasMap, false)

		// Should be ReadOnlyFs in production mode
		fs := infrastructure.GetFileSystem()
		if fs == nil {
			t.Error("Expected filesystem to be initialized")
		}
	})

	// Test error handling
	t.Run("ErrorHandling", func(t *testing.T) {
		// Reset for this test
		infrastructure.ResetForTesting()

		// Test Read without initialization
		_, err := infrastructure.Read("test")
		if err == nil {
			t.Error("Expected error when reading without initialization")
		}
		if !strings.Contains(err.Error(), "not initialized") {
			t.Errorf("Expected 'not initialized' error, got: %v", err)
		}

		// Test Stream without initialization
		_, err = infrastructure.Stream("test")
		if err == nil {
			t.Error("Expected error when streaming without initialization")
		}
		if !strings.Contains(err.Error(), "not initialized") {
			t.Errorf("Expected 'not initialized' error, got: %v", err)
		}

		// Test Exists without initialization
		if infrastructure.Exists("test") {
			t.Error("Expected false when checking existence without initialization")
		}

		// Now initialize and test with non-existent alias
		infrastructure.Init(testFS, aliasMap, true)

		// Test Read with non-existent alias
		_, err = infrastructure.Read("nonexistent")
		if err == nil {
			t.Error("Expected error when reading non-existent alias")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}

		// Test Stream with non-existent alias
		_, err = infrastructure.Stream("nonexistent")
		if err == nil {
			t.Error("Expected error when streaming non-existent alias")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Expected 'not found' error, got: %v", err)
		}
	})
}
