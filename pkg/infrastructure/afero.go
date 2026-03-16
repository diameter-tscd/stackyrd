package infrastructure

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

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

// embedFSWrapper wraps embed.FS to implement afero.Fs interface
type embedFSWrapper struct {
	fs embed.FS
}

// Chtimes changes file access and modification times (not supported for embed.FS)
func (e *embedFSWrapper) Chtimes(name string, atime, mtime time.Time) error {
	return fmt.Errorf("chtimes not supported for embedded filesystem")
}

// OpenFile opens a file with the given flags and permissions (not supported for embed.FS)
func (e *embedFSWrapper) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag != os.O_RDONLY {
		return nil, fmt.Errorf("openfile not supported for embedded filesystem (only read-only mode)")
	}
	return e.Open(name)
}

// Open opens a file from the embedded filesystem
func (e *embedFSWrapper) Open(name string) (afero.File, error) {
	file, err := e.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return &embedFile{File: file}, nil
}

// Create creates a new file (not supported for embed.FS)
func (e *embedFSWrapper) Create(name string) (afero.File, error) {
	return nil, fmt.Errorf("create not supported for embedded filesystem")
}

// Mkdir creates a directory (not supported for embed.FS)
func (e *embedFSWrapper) Mkdir(name string, perm os.FileMode) error {
	return fmt.Errorf("mkdir not supported for embedded filesystem")
}

// MkdirAll creates a directory path (not supported for embed.FS)
func (e *embedFSWrapper) MkdirAll(path string, perm os.FileMode) error {
	return fmt.Errorf("mkdirall not supported for embedded filesystem")
}

// Remove removes a file (not supported for embed.FS)
func (e *embedFSWrapper) Remove(name string) error {
	return fmt.Errorf("remove not supported for embedded filesystem")
}

// RemoveAll removes a directory path (not supported for embed.FS)
func (e *embedFSWrapper) RemoveAll(path string) error {
	return fmt.Errorf("removeall not supported for embedded filesystem")
}

// Rename renames a file (not supported for embed.FS)
func (e *embedFSWrapper) Rename(oldname, newname string) error {
	return fmt.Errorf("rename not supported for embedded filesystem")
}

// Stat returns file info
func (e *embedFSWrapper) Stat(name string) (os.FileInfo, error) {
	file, err := e.fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file info from the opened file
	if stat, ok := file.(fs.FileInfo); ok {
		return stat, nil
	}

	// Fallback: try to get info from the file itself
	if stater, ok := file.(interface{ Stat() (fs.FileInfo, error) }); ok {
		return stater.Stat()
	}

	return nil, fmt.Errorf("stat not supported for this file")
}

// Name returns the name of the filesystem
func (e *embedFSWrapper) Name() string {
	return "embedFS"
}

// Chmod changes file permissions (not supported for embed.FS)
func (e *embedFSWrapper) Chmod(name string, mode os.FileMode) error {
	return fmt.Errorf("chmod not supported for embedded filesystem")
}

// Chown changes file ownership (not supported for embed.FS)
func (e *embedFSWrapper) Chown(name string, uid, gid int) error {
	return fmt.Errorf("chown not supported for embedded filesystem")
}

// embedFile wraps an fs.File to implement afero.File interface
type embedFile struct {
	fs.File
}

// Close closes the file
func (e *embedFile) Close() error {
	return e.File.Close()
}

// Read reads from the file
func (e *embedFile) Read(b []byte) (int, error) {
	return e.File.Read(b)
}

// ReadAt reads from the file at a specific offset
func (e *embedFile) ReadAt(b []byte, off int64) (int, error) {
	if reader, ok := e.File.(io.ReaderAt); ok {
		return reader.ReadAt(b, off)
	}
	return 0, fmt.Errorf("ReadAt not supported")
}

// Seek seeks to a position in the file
func (e *embedFile) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := e.File.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	}
	return 0, fmt.Errorf("Seek not supported")
}

// Write writes to the file (not supported for embed.FS)
func (e *embedFile) Write(b []byte) (int, error) {
	return 0, fmt.Errorf("write not supported for embedded file")
}

// WriteAt writes to the file at a specific offset (not supported for embed.FS)
func (e *embedFile) WriteAt(b []byte, off int64) (int, error) {
	return 0, fmt.Errorf("writeat not supported for embedded file")
}

// Name returns the file name
func (e *embedFile) Name() string {
	// Try to get name from the underlying file
	if namer, ok := e.File.(interface{ Name() string }); ok {
		return namer.Name()
	}
	return ""
}

// Readdir reads directory entries
func (e *embedFile) Readdir(count int) ([]os.FileInfo, error) {
	if dir, ok := e.File.(fs.ReadDirFile); ok {
		entries, err := dir.ReadDir(count)
		if err != nil {
			return nil, err
		}

		fileInfos := make([]os.FileInfo, len(entries))
		for i, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			fileInfos[i] = info
		}
		return fileInfos, nil
	}
	return nil, fmt.Errorf("Readdir not supported")
}

// Readdirnames reads directory entry names
func (e *embedFile) Readdirnames(n int) ([]string, error) {
	if dir, ok := e.File.(fs.ReadDirFile); ok {
		entries, err := dir.ReadDir(n)
		if err != nil {
			return nil, err
		}

		names := make([]string, len(entries))
		for i, entry := range entries {
			names[i] = entry.Name()
		}
		return names, nil
	}
	return nil, fmt.Errorf("Readdirnames not supported")
}

// Sync synchronizes file data (not supported for embed.FS)
func (e *embedFile) Sync() error {
	return fmt.Errorf("sync not supported for embedded file")
}

// Truncate truncates the file (not supported for embed.FS)
func (e *embedFile) Truncate(size int64) error {
	return fmt.Errorf("truncate not supported for embedded file")
}

// WriteString writes a string to the file (not supported for embed.FS)
func (e *embedFile) WriteString(s string) (int, error) {
	return 0, fmt.Errorf("writeString not supported for embedded file")
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
			baseFS := &embedFSWrapper{fs: embedFS}
			writableFS := afero.NewOsFs()
			instance.fs = afero.NewCopyOnWriteFs(baseFS, writableFS)
		} else {
			// Production mode: Read-only filesystem wrapping embed.FS
			baseFS := &embedFSWrapper{fs: embedFS}
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
	if filepath.HasPrefix(physicalPath, "all:") {
		physicalPath = physicalPath[4:] // Remove "all:" prefix
	}

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
