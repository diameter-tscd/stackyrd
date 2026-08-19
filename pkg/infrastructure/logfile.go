package infrastructure

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"
)

type FileLogger struct {
	mu          sync.Mutex
	basePath    string
	filename    string
	maxFiles    int
	compress    bool
	currentDate string
	file        *os.File
}

func init() {
	RegisterComponent("logfile", func(cfg *config.Config, l *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.Log.Enabled {
			return nil, nil
		}

		basePath := cfg.Log.Path
		if !filepath.IsAbs(basePath) {
			cwd, err := os.Getwd()
			if err == nil {
				basePath = filepath.Join(cwd, basePath)
			}
		}

		if err := os.MkdirAll(basePath, 0o755); err != nil {
			return nil, err
		}

		fl := &FileLogger{
			basePath:    basePath,
			filename:    cfg.Log.Filename,
			maxFiles:    max(1, cfg.Log.MaxFiles),
			compress:    cfg.Log.Compress,
			currentDate: time.Now().Format("2006-01-02"),
		}
		fl.openCurrentFile()
		return fl, nil
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (f *FileLogger) Name() string { return "Log File" }

func (f *FileLogger) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

func (f *FileLogger) GetStatus() map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return map[string]interface{}{
		"connected":    f.file != nil,
		"path":         f.basePath,
		"filename":     f.filename,
		"max_files":    f.maxFiles,
		"compress":     f.compress,
		"current_date": f.currentDate,
	}
}

func (f *FileLogger) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.needsRotation() {
		f.rotate()
	}

	if f.file == nil {
		f.openCurrentFile()
	}

	if f.file == nil {
		return 0, os.ErrClosed
	}

	return f.file.Write(p)
}

func (f *FileLogger) needsRotation() bool {
	return time.Now().Format("2006-01-02") != f.currentDate
}

func (f *FileLogger) openCurrentFile() {
	dateStr := time.Now().Format("2006-01-02")
	name := filepath.Join(f.basePath, fmt.Sprintf("%s.%s", f.filename, dateStr))
	var err error
	f.file, err = os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		f.file = nil
	}
}

func (f *FileLogger) rotate() {
	if f.file != nil {
		f.file.Close()
		f.file = nil
	}

	oldDate := f.currentDate
	for i := f.maxFiles - 1; i >= 0; i-- {
		var oldFile string
		if i == 0 {
			oldFile = filepath.Join(f.basePath, fmt.Sprintf("%s.%s", f.filename, oldDate))
		} else {
			oldFile = filepath.Join(f.basePath, fmt.Sprintf("%s.%d.%s", f.filename, i, oldDate))
		}

		nextFile := ""
		if i < f.maxFiles-1 {
			nextDate := time.Now().Format("2006-01-02")
			nextFile = filepath.Join(f.basePath, fmt.Sprintf("%s.%d.%s", f.filename, i+1, nextDate))
		}

		if _, err := os.Stat(oldFile); os.IsNotExist(err) {
			continue
		}

		if f.compress {
			gzPath := oldFile + ".gz"
			f.compressFile(oldFile, gzPath)
			os.Remove(oldFile)
		} else if nextFile != "" && i > 0 {
			os.Rename(oldFile, nextFile)
		}
	}

	f.currentDate = time.Now().Format("2006-01-02")
	f.openCurrentFile()
}

func (f *FileLogger) compressFile(src, dst string) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer out.Close()

	gzw := gzip.NewWriter(out)
	io.Copy(gzw, in)
	gzw.Close()
}