package infrastructure

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/spf13/afero"
)

type EmbedFSAdapter struct {
	FS embed.FS
}

func (e *EmbedFSAdapter) Chtimes(name string, atime, mtime time.Time) error {
	return fmt.Errorf("chtimes not supported for embedded filesystem")
}

func (e *EmbedFSAdapter) Chmod(name string, mode os.FileMode) error {
	return fmt.Errorf("chmod not supported for embedded filesystem")
}

func (e *EmbedFSAdapter) Chown(name string, uid, gid int) error {
	return fmt.Errorf("chown not supported for embedded filesystem")
}

func (e *EmbedFSAdapter) Name() string {
	return "embedFS"
}

func (e *EmbedFSAdapter) Stat(name string) (os.FileInfo, error) {
	f, err := e.FS.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return f.Stat()
}

func (e *EmbedFSAdapter) Rename(oldname, newname string) error {
	return fmt.Errorf("rename not supported for embedded filesystem")
}

func (e *EmbedFSAdapter) RemoveAll(path string) error {
	return fmt.Errorf("removeall not supported for embedded filesystem")
}

func (e *EmbedFSAdapter) Remove(name string) error {
	return fmt.Errorf("remove not supported for embedded filesystem")
}

func (e *EmbedFSAdapter) MkdirAll(path string, perm os.FileMode) error {
	return fmt.Errorf("mkdirall not supported for embedded filesystem")
}

func (e *EmbedFSAdapter) Mkdir(name string, perm os.FileMode) error {
	return fmt.Errorf("mkdir not supported for embedded filesystem")
}

func (e *EmbedFSAdapter) Open(name string) (afero.File, error) {
	f, err := e.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &EmbedFile{File: f}, nil
}

func (e *EmbedFSAdapter) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag != os.O_RDONLY {
		return nil, fmt.Errorf("openfile not supported for embedded filesystem (only read-only mode)")
	}
	return e.Open(name)
}

func (e *EmbedFSAdapter) Create(name string) (afero.File, error) {
	return nil, fmt.Errorf("create not supported for embedded filesystem")
}

type EmbedFile struct {
	fs.File
}

func (f *EmbedFile) Close() error {
	return f.File.Close()
}

func (f *EmbedFile) Read(b []byte) (int, error) {
	return f.File.Read(b)
}

func (f *EmbedFile) ReadAt(b []byte, off int64) (int, error) {
	if r, ok := f.File.(io.ReaderAt); ok {
		return r.ReadAt(b, off)
	}
	return 0, fmt.Errorf("ReadAt not supported")
}

func (f *EmbedFile) Seek(offset int64, whence int) (int64, error) {
	if s, ok := f.File.(io.Seeker); ok {
		return s.Seek(offset, whence)
	}
	return 0, fmt.Errorf("Seek not supported")
}

func (f *EmbedFile) Write(b []byte) (int, error) {
	return 0, fmt.Errorf("write not supported for embedded file")
}

func (f *EmbedFile) WriteAt(b []byte, off int64) (int, error) {
	return 0, fmt.Errorf("writeat not supported for embedded file")
}

func (f *EmbedFile) Name() string {
	if n, ok := f.File.(interface{ Name() string }); ok {
		return n.Name()
	}
	return ""
}

func (f *EmbedFile) Readdir(count int) ([]os.FileInfo, error) {
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

func (f *EmbedFile) Readdirnames(n int) ([]string, error) {
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

func (f *EmbedFile) Sync() error {
	return fmt.Errorf("sync not supported for embedded file")
}

func (f *EmbedFile) Truncate(size int64) error {
	return fmt.Errorf("truncate not supported for embedded file")
}

func (f *EmbedFile) WriteString(s string) (int, error) {
	return 0, fmt.Errorf("writestring not supported for embedded file")
}
