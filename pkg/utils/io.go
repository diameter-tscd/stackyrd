package utils

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WriteFile writes content to a file, creating directories if needed.
// It overwrites the file if it exists.
func WriteFile(path string, content []byte) error {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}
	return os.WriteFile(path, content, 0644)
}

// ReadFile reads the content of a file.
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(filepath.Clean(path))
}

// FileExists checks if a file exists and is not a directory.
func FileExists(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// IsDir returns true if the path points to a directory.
func IsDir(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsSymlink returns true if the path is a symbolic link.
func IsSymlink(path string) bool {
	info, err := os.Lstat(filepath.Clean(path))
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// EnsureDir creates a directory and any parents if they don't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(filepath.Clean(path), 0755)
}

// AppendFile appends content to a file, creating it if it doesn't exist.
func AppendFile(path string, content []byte) error {
	path = filepath.Clean(path)
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}
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

// AppendLine appends a single line (with newline) to a file.
func AppendLine(path string, line string) error {
	return AppendFile(path, []byte(line+"\n"))
}

// CopyFile copies a file from src to dst.
// If dst already exists, it is overwritten.
func CopyFile(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	fi, err := srcFile.Stat()
	if err == nil {
		_ = os.Chmod(dst, fi.Mode())
	}

	return nil
}

// MoveFile moves or renames a file. Falls back to copy+delete across devices.
func MoveFile(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := CopyFile(src, dst); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	_ = os.Remove(src)
	return nil
}

// SafeWriteFile atomically writes content to path by writing to a temp file
// in the same directory and then renaming.
func SafeWriteFile(path string, content []byte) error {
	path = filepath.Clean(path)
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp_*"+filepath.Ext(path))
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	cleanup = false
	return nil
}

// WriteReaderToFile copies from an io.Reader to a file.
func WriteReaderToFile(reader io.Reader, path string) error {
	path = filepath.Clean(path)
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	return nil
}

// Touch creates an empty file or updates the modification time of an existing one.
func Touch(path string) error {
	path = filepath.Clean(path)
	f, err := os.OpenFile(path, os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to touch file: %w", err)
	}
	return f.Close()
}

// ListFiles returns non-recursive list of files in a directory, optionally filtered by extension.
// Extensions should include the dot, e.g. ".go". An empty filter returns all files.
func ListFiles(dir string, extensions ...string) ([]string, error) {
	dir = filepath.Clean(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	filter := len(extensions) > 0
	extSet := make(map[string]bool, len(extensions))
	for _, e := range extensions {
		extSet[strings.ToLower(e)] = true
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filter && !extSet[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}

	sort.Strings(files)
	return files, nil
}

// ListFilesRecursive recursively lists all files under root, optionally filtered by extension.
func ListFilesRecursive(root string, extensions ...string) ([]string, error) {
	root = filepath.Clean(root)

	filter := len(extensions) > 0
	extSet := make(map[string]bool, len(extensions))
	for _, e := range extensions {
		extSet[strings.ToLower(e)] = true
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filter && !extSet[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// ListDirs returns the immediate subdirectories of a directory.
func ListDirs(dir string) ([]string, error) {
	dir = filepath.Clean(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(dir, e.Name()))
		}
	}

	sort.Strings(dirs)
	return dirs, nil
}

// Glob returns matching file paths. Wraps filepath.Glob with a clean error message.
func Glob(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern %q: %w", pattern, err)
	}
	return matches, nil
}

// CopyDir recursively copies a directory tree.
// If filter is non-nil, only files for which filter returns true are copied.
func CopyDir(src, dst string, filter func(path string) bool) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		if filter != nil && !filter(path) {
			return nil
		}

		return CopyFile(path, target)
	})
}

// RemoveDir removes a directory and all its contents.
func RemoveDir(dir string) error {
	return os.RemoveAll(filepath.Clean(dir))
}

// IsEmptyDir returns true if the directory exists and contains no entries.
func IsEmptyDir(path string) (bool, error) {
	path = filepath.Clean(path)
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open directory: %w", err)
	}
	defer func() { _ = f.Close() }()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read directory: %w", err)
	}
	return false, nil
}

// FileSize returns the size of a file in bytes.
func FileSize(path string) (int64, error) {
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return 0, fmt.Errorf("path is a directory: %s", path)
	}
	return info.Size(), nil
}

// DirSize recursively calculates the total size of a directory in bytes.
func DirSize(path string) (int64, error) {
	path = filepath.Clean(path)
	var total int64
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total, err
}

// ReadLines reads a file and returns its lines as a string slice.
func ReadLines(path string) ([]string, error) {
	path = filepath.Clean(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	return lines, nil
}

// Head returns the first N lines of a file.
func Head(path string, n int) ([]string, error) {
	path = filepath.Clean(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() && len(lines) < n {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	return lines, nil
}

// Tail returns the last N lines of a file.
func Tail(path string, n int) ([]string, error) {
	path = filepath.Clean(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	return lines, nil
}

// CountLines counts the number of lines in a file.
func CountLines(path string) (int, error) {
	path = filepath.Clean(path)
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var count int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading file: %w", err)
	}
	return count, nil
}

// MD5File returns the MD5 checksum of a file as a hex string.
func MD5File(path string) (string, error) {
	return hashFile(path, md5.New())
}

// SHA256File returns the SHA-256 checksum of a file as a hex string.
func SHA256File(path string) (string, error) {
	return hashFile(path, sha256.New())
}

func hashFile(path string, h io.Writer) (string, error) {
	path = filepath.Clean(path)
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.(hashSum).Sum(nil)), nil
}

type hashSum interface {
	io.Writer
	Sum(b []byte) []byte
}

// UniquePath returns a non-colliding path by appending a numeric suffix
// (e.g., file (1).txt, file (2).txt) if the original path exists.
func UniquePath(path string) string {
	path = filepath.Clean(path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// ResolvePath cleans and converts a path to an absolute path.
func ResolvePath(path string) (string, error) {
	path = filepath.Clean(path)
	if filepath.IsAbs(path) {
		return path, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	return abs, nil
}

// IsChildPath checks whether child is a sub-path of parent (no directory traversal).
func IsChildPath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

// ReadSeekerAt reads a range of bytes from a file at a given offset.
func ReadSeekerAt(path string, offset int64, length int64) ([]byte, error) {
	path = filepath.Clean(path)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, length)
	n, err := f.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read at offset %d: %w", offset, err)
	}

	return buf[:n], nil
}

// ReadWithReader reads the full content of a reader into a byte slice.
func ReadWithReader(reader io.Reader) ([]byte, error) {
	return io.ReadAll(reader)
}

// WriteString writes a string to a file.
func WriteString(path, content string) error {
	return WriteFile(path, []byte(content))
}

// StringToReadCloser converts a string to an io.ReadCloser.
func StringToReadCloser(s string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(s))
}

// BytesToReadCloser converts a byte slice to an io.ReadCloser.
func BytesToReadCloser(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
}

// ReadFileAsString reads a file and returns its content as a string.
func ReadFileAsString(path string) (string, error) {
	data, err := ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
