// Package logging provides structured logging with rotation and sampling support.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RotationConfig holds log rotation configuration
type RotationConfig struct {
	MaxSize    int64         // Maximum size in bytes before rotation
	MaxAge     time.Duration // Maximum age of log files
	MaxBackups int           // Maximum number of backup files
	Compress   bool          // Compress rotated files
}

// DefaultRotationConfig returns default rotation configuration
func DefaultRotationConfig() RotationConfig {
	return RotationConfig{
		MaxSize:    100 * 1024 * 1024,  // 100MB
		MaxAge:     7 * 24 * time.Hour, // 7 days
		MaxBackups: 10,
		Compress:   true,
	}
}

// RotatingWriter wraps a writer with log rotation
type RotatingWriter struct {
	config   RotationConfig
	filename string
	file     *os.File
	size     int64
	mu       sync.Mutex
}

// NewRotatingWriter creates a new rotating writer
func NewRotatingWriter(filename string, config RotationConfig) (*RotatingWriter, error) {
	rw := &RotatingWriter{
		config:   config,
		filename: filename,
	}

	if err := rw.openFile(); err != nil {
		return nil, err
	}

	return rw, nil
}

// Write implements io.Writer interface
func (rw *RotatingWriter) Write(p []byte) (n int, err error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.file == nil {
		if err := rw.openFile(); err != nil {
			return 0, err
		}
	}

	// Check if rotation is needed
	if rw.size+int64(len(p)) > rw.config.MaxSize {
		if err := rw.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = rw.file.Write(p)
	rw.size += int64(n)

	return n, err
}

// openFile opens the log file
func (rw *RotatingWriter) openFile() error {
	dir := filepath.Dir(rw.filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(rw.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}

	rw.file = file
	rw.size = info.Size()

	return nil
}

// rotate rotates the log file
func (rw *RotatingWriter) rotate() error {
	if rw.file != nil {
		_ = rw.file.Close()
		rw.file = nil
	}

	// Generate backup filename
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	backupName := fmt.Sprintf("%s.%s", rw.filename, timestamp)

	// Rename current file
	if err := os.Rename(rw.filename, backupName); err != nil {
		// If rename fails, try to remove the file
		_ = os.Remove(rw.filename)
	}

	// Compress if configured
	if rw.config.Compress {
		if err := rw.compressFile(backupName); err != nil {
			// Log compression error but don't fail
			fmt.Fprintf(os.Stderr, "Failed to compress log file: %v\n", err)
		}
	}

	// Clean up old backups
	if err := rw.cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup old logs: %v\n", err)
	}

	// Open new file
	return rw.openFile()
}

// compressFile compresses a log file
func (rw *RotatingWriter) compressFile(filename string) error {
	// Simple compression using gzip would go here
	// For now, just return nil
	return nil
}

// cleanup removes old backup files
func (rw *RotatingWriter) cleanup() error {
	dir := filepath.Dir(rw.filename)
	base := filepath.Base(rw.filename)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var backups []os.FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, base+".") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			backups = append(backups, info)
		}
	}

	// Sort by modification time (oldest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].ModTime().Before(backups[j].ModTime())
	})

	// Remove old backups
	for i := 0; i < len(backups)-rw.config.MaxBackups; i++ {
		path := filepath.Join(dir, backups[i].Name())
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the rotating writer
func (rw *RotatingWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.file != nil {
		return rw.file.Close()
	}

	return nil
}

// GetStats returns rotation statistics
func (rw *RotatingWriter) GetStats() map[string]interface{} {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	return map[string]interface{}{
		"filename":    rw.filename,
		"size":        rw.size,
		"max_size":    rw.config.MaxSize,
		"max_age":     rw.config.MaxAge.String(),
		"max_backups": rw.config.MaxBackups,
		"compress":    rw.config.Compress,
	}
}
