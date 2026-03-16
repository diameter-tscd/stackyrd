package infrastructure

import (
	"embed"
	"strings"
	"sync"
	"testing"
)

//go:embed testdata/*
var testFS embed.FS

func TestAferoManager(t *testing.T) {
	// Reset the singleton for testing
	instance = nil

	// Test alias configuration
	aliasMap := map[string]string{
		"config": "all:testdata/config.yaml",
		"readme": "all:testdata/README.md",
		"test":   "all:testdata/test.txt",
	}

	// Test initialization
	t.Run("Init", func(t *testing.T) {
		Init(testFS, aliasMap, true)

		if instance == nil {
			t.Fatal("Expected instance to be initialized")
		}

		if instance.fs == nil {
			t.Fatal("Expected filesystem to be initialized")
		}

		if len(instance.aliases) != 3 {
			t.Errorf("Expected 3 aliases, got %d", len(instance.aliases))
		}
	})

	// Test Exists function
	t.Run("Exists", func(t *testing.T) {
		// Test non-existing alias
		if Exists("nonexistent") {
			t.Error("Expected 'nonexistent' alias to not exist")
		}

		// Test existing alias but non-existing file
		aliasMap := map[string]string{
			"missing": "all:testdata/missing.txt",
		}
		Init(testFS, aliasMap, true)
		if Exists("missing") {
			t.Error("Expected 'missing' alias to not exist (file doesn't exist)")
		}
	})

	// Test GetAliases function
	t.Run("GetAliases", func(t *testing.T) {
		aliases := GetAliases()
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
		fs := GetFileSystem()
		if fs == nil {
			t.Error("Expected filesystem to be returned")
		}
	})

	// Test development mode (CopyOnWriteFs)
	t.Run("DevelopmentMode", func(t *testing.T) {
		// Should be CopyOnWriteFs in development mode
		fs := GetFileSystem()
		if fs == nil {
			t.Error("Expected filesystem to be initialized")
		}
	})

	// Test production mode (ReadOnlyFs)
	t.Run("ProductionMode", func(t *testing.T) {
		// Create a new test with production mode
		// Reset instance for this test
		instance = nil

		// Create a new once variable for this test
		originalOnce := once
		once = sync.Once{}

		aliasMap := map[string]string{
			"test": "all:testdata/test.txt",
		}
		Init(testFS, aliasMap, false)

		// Should be ReadOnlyFs in production mode
		fs := GetFileSystem()
		if fs == nil {
			t.Error("Expected filesystem to be initialized")
		}

		// Restore original once
		once = originalOnce
	})

	// Test singleton behavior (multiple Init calls)
	t.Run("Singleton", func(t *testing.T) {
		aliasMap1 := map[string]string{
			"test1": "all:testdata/test.txt",
		}
		aliasMap2 := map[string]string{
			"test2": "all:testdata/test.txt",
		}

		Init(testFS, aliasMap1, true)
		initialInstance := instance

		Init(testFS, aliasMap2, true) // Should be ignored due to singleton
		if instance != initialInstance {
			t.Error("Expected singleton behavior - instance should not change")
		}
	})

	// Test error handling
	t.Run("ErrorHandling", func(t *testing.T) {
		// Reset instance
		instance = nil

		// Test Read without initialization
		_, err := Read("test")
		if err == nil {
			t.Error("Expected error when reading without initialization")
		}
		if !strings.Contains(err.Error(), "not initialized") {
			t.Errorf("Expected 'not initialized' error, got: %v", err)
		}

		// Test Stream without initialization
		_, err = Stream("test")
		if err == nil {
			t.Error("Expected error when streaming without initialization")
		}
		if !strings.Contains(err.Error(), "not initialized") {
			t.Errorf("Expected 'not initialized' error, got: %v", err)
		}

		// Test Exists without initialization
		if Exists("test") {
			t.Error("Expected false when checking existence without initialization")
		}
	})
}
