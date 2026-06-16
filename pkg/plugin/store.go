package plugin

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/afero"
)

type embedFSAdapter struct {
	fs embed.FS
}

func (e *embedFSAdapter) Chtimes(name string, atime, mtime time.Time) error {
	return fmt.Errorf("chtimes not supported for embedded filesystem")
}

func (e *embedFSAdapter) Chmod(name string, mode os.FileMode) error {
	return fmt.Errorf("chmod not supported for embedded filesystem")
}

func (e *embedFSAdapter) Chown(name string, uid, gid int) error {
	return fmt.Errorf("chown not supported for embedded filesystem")
}

func (e *embedFSAdapter) Name() string {
	return "embedFS"
}

func (e *embedFSAdapter) Stat(name string) (os.FileInfo, error) {
	f, err := e.fs.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return f.Stat()
}

func (e *embedFSAdapter) Rename(oldname, newname string) error {
	return fmt.Errorf("rename not supported for embedded filesystem")
}

func (e *embedFSAdapter) RemoveAll(path string) error {
	return fmt.Errorf("removeall not supported for embedded filesystem")
}

func (e *embedFSAdapter) Remove(name string) error {
	return fmt.Errorf("remove not supported for embedded filesystem")
}

func (e *embedFSAdapter) MkdirAll(path string, perm os.FileMode) error {
	return fmt.Errorf("mkdirall not supported for embedded filesystem")
}

func (e *embedFSAdapter) Mkdir(name string, perm os.FileMode) error {
	return fmt.Errorf("mkdir not supported for embedded filesystem")
}

func (e *embedFSAdapter) Open(name string) (afero.File, error) {
	f, err := e.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return &embedFile{File: f}, nil
}

func (e *embedFSAdapter) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag != os.O_RDONLY {
		return nil, fmt.Errorf("openfile not supported for embedded filesystem (only read-only mode)")
	}
	return e.Open(name)
}

func (e *embedFSAdapter) Create(name string) (afero.File, error) {
	return nil, fmt.Errorf("create not supported for embedded filesystem")
}

type embedFile struct {
	fs.File
}

func (f *embedFile) Close() error {
	return f.File.Close()
}

func (f *embedFile) Read(b []byte) (int, error) {
	return f.File.Read(b)
}

func (f *embedFile) ReadAt(b []byte, off int64) (int, error) {
	if r, ok := f.File.(io.ReaderAt); ok {
		return r.ReadAt(b, off)
	}
	return 0, fmt.Errorf("ReadAt not supported")
}

func (f *embedFile) Seek(offset int64, whence int) (int64, error) {
	if s, ok := f.File.(io.Seeker); ok {
		return s.Seek(offset, whence)
	}
	return 0, fmt.Errorf("Seek not supported")
}

func (f *embedFile) Write(b []byte) (int, error) {
	return 0, fmt.Errorf("write not supported for embedded file")
}

func (f *embedFile) WriteAt(b []byte, off int64) (int, error) {
	return 0, fmt.Errorf("writeat not supported for embedded file")
}

func (f *embedFile) Name() string {
	if n, ok := f.File.(interface{ Name() string }); ok {
		return n.Name()
	}
	return ""
}

func (f *embedFile) Readdir(count int) ([]os.FileInfo, error) {
	if d, ok := f.File.(fs.ReadDirFile); ok {
		entries, err := d.ReadDir(count)
		if err != nil {
			return nil, err
		}
		infos := make([]os.FileInfo, len(entries))
		for i, e := range entries {
			infos[i], err = e.Info()
			if err != nil {
				return nil, err
			}
		}
		return infos, nil
	}
	return nil, fmt.Errorf("Readdir not supported")
}

func (f *embedFile) Readdirnames(n int) ([]string, error) {
	if d, ok := f.File.(fs.ReadDirFile); ok {
		entries, err := d.ReadDir(n)
		if err != nil {
			return nil, err
		}
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		return names, nil
	}
	return nil, fmt.Errorf("Readdirnames not supported")
}

func (f *embedFile) Sync() error {
	return fmt.Errorf("sync not supported for embedded file")
}

func (f *embedFile) Truncate(size int64) error {
	return fmt.Errorf("truncate not supported for embedded file")
}

func (f *embedFile) WriteString(s string) (int, error) {
	return 0, fmt.Errorf("writestring not supported for embedded file")
}

func buildPluginFS(embedded embed.FS, prefix string, storeDir string) afero.Fs {
	baseFS := &embedFSAdapter{fs: embedded}
	basePrefixed := afero.NewBasePathFs(baseFS, prefix)

	if storeDir == "" {
		return afero.NewReadOnlyFs(basePrefixed)
	}

	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return afero.NewReadOnlyFs(basePrefixed)
	}

	overlayFS := afero.NewBasePathFs(afero.NewOsFs(), storeDir)
	return afero.NewCopyOnWriteFs(basePrefixed, overlayFS)
}

func ensureStoreDir(baseDir string) error {
	dirs := []string{
		filepath.Join(baseDir, "scripts"),
		filepath.Join(baseDir, "static"),
		filepath.Join(baseDir, ".cache"),
		filepath.Join(baseDir, "config"),
		filepath.Join(baseDir, "data"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}
