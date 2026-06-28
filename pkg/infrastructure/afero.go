package infrastructure

import (
	"embed"
	"fmt"
	"io"
	"strings"
	"sync"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/spf13/afero"
)

// assets is the global singleton Afero manager
var (
	instance *aferoManager
	once     sync.Once
)

// aferoManager represents the singleton Afero filesystem manager
type aferoManager struct {
	fs      afero.Fs
	aliases map[string]string
	mu      sync.RWMutex
}



// Init initializes the singleton Afero manager with the given configuration
// This function is safe to call multiple times - subsequent calls will be ignored
func Init(embedFS embed.FS, aliasMap map[string]string, isDev bool) {
	once.Do(func() {
		instance = &aferoManager{
			aliases: make(map[string]string),
		}

		// Set up the filesystem based on environment
		if isDev {
			// Development mode: CopyOnWriteFs allows local overrides
			// Base layer is embed.FS, writable layer is OS filesystem
			baseFS := &EmbedFSAdapter{FS: embedFS}
			writableFS := afero.NewOsFs()
			instance.fs = afero.NewCopyOnWriteFs(baseFS, writableFS)
		} else {
			// Production mode: Read-only filesystem wrapping embed.FS
			baseFS := &EmbedFSAdapter{FS: embedFS}
			instance.fs = afero.NewReadOnlyFs(baseFS)
		}

		// Copy the alias map to avoid external mutations
		for alias, path := range aliasMap {
			instance.aliases[alias] = path
		}
	})
}

// Read reads the file content for the given alias
// Returns the file content as bytes and any error encountered
func Read(alias string) ([]byte, error) {
	if instance == nil {
		return nil, fmt.Errorf("afero manager not initialized. Call Init() first")
	}

	instance.mu.RLock()
	defer instance.mu.RUnlock()

	// Resolve alias to physical path
	physicalPath, err := instance.resolveAlias(alias)
	if err != nil {
		return nil, err
	}

	// Read the file using Afero
	return afero.ReadFile(instance.fs, physicalPath)
}

// Stream returns a ReadCloser for streaming the file content for the given alias
// The caller is responsible for closing the returned ReadCloser
func Stream(alias string) (io.ReadCloser, error) {
	if instance == nil {
		return nil, fmt.Errorf("afero manager not initialized. Call Init() first")
	}

	instance.mu.RLock()
	defer instance.mu.RUnlock()

	// Resolve alias to physical path
	physicalPath, err := instance.resolveAlias(alias)
	if err != nil {
		return nil, err
	}

	// Open the file using Afero
	return instance.fs.Open(physicalPath)
}

// Exists checks if the alias exists in the alias map AND the file exists in the filesystem
// Returns true if both conditions are met, false otherwise
func Exists(alias string) bool {
	if instance == nil {
		return false
	}

	instance.mu.RLock()
	defer instance.mu.RUnlock()

	// Check if alias exists in map
	physicalPath, exists := instance.aliases[alias]
	if !exists {
		return false
	}

	// Handle "all:" prefix if present
	physicalPath = strings.TrimPrefix(physicalPath, "all:")

	// Check if file exists in filesystem
	_, err := instance.fs.Stat(physicalPath)
	return err == nil
}

// resolveAlias resolves an alias to its physical path
// Handles the "all:" prefix that may be used with embed.FS
func (m *aferoManager) resolveAlias(alias string) (string, error) {
	physicalPath, exists := m.aliases[alias]
	if !exists {
		return "", fmt.Errorf("alias '%s' not found in alias map", alias)
	}

	// Handle "all:" prefix if present
	physicalPath = strings.TrimPrefix(physicalPath, "all:")

	return physicalPath, nil
}

// GetAliases returns a copy of all configured aliases
// This is useful for debugging or introspection
func GetAliases() map[string]string {
	if instance == nil {
		return make(map[string]string)
	}

	instance.mu.RLock()
	defer instance.mu.RUnlock()

	// Return a copy to prevent external mutations
	aliases := make(map[string]string)
	for alias, path := range instance.aliases {
		aliases[alias] = path
	}

	return aliases
}

// GetFileSystem returns the underlying Afero filesystem
// This is useful for advanced operations that need direct filesystem access
func GetFileSystem() afero.Fs {
	if instance == nil {
		return nil
	}

	instance.mu.RLock()
	defer instance.mu.RUnlock()

	return instance.fs
}

// ResetForTesting resets the singleton for testing purposes
// This function should only be used in tests
func ResetForTesting() {
	instance = nil
	once = sync.Once{}
}

// Name returns the component name
func (m *aferoManager) Name() string {
	return "Afero Filesystem"
}

// GetStatus returns the current status
func (m *aferoManager) GetStatus() map[string]interface{} {
	if instance == nil {
		return map[string]interface{}{"initialized": false}
	}
	instance.mu.RLock()
	defer instance.mu.RUnlock()
	return map[string]interface{}{
		"initialized": true,
		"aliases":     len(instance.aliases),
	}
}

// Close cleans up the filesystem manager
func (m *aferoManager) Close() error {
	return nil
}

func init() {
	// Register as infrastructure component — uses OS filesystem by default.
	// Call Init() separately with an embed.FS to layer embed on top.
	RegisterComponent("afero", func(cfg *config.Config, l *logger.Logger) (InfrastructureComponent, error) {
		once.Do(func() {
			instance = &aferoManager{
				fs:      afero.NewOsFs(),
				aliases: make(map[string]string),
			}
		})
		l.Info("Afero filesystem manager initialized (OS filesystem)")
		return instance, nil
	})
}
