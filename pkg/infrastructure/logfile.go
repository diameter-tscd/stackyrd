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

	"github.com/rs/zerolog"
)

type FileLogger struct {
	mu          sync.RWMutex
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
	f.mu.RLock()
	defer f.mu.RUnlock()
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
	needRotate := false
	f.mu.RLock()
	if f.needsRotation() {
		needRotate = true
	}
	f.mu.RUnlock()
	if needRotate {
		f.mu.Lock()
		if f.needsRotation() {
			f.rotateLocked()
		}
		f.mu.Unlock()
	}
	f.mu.RLock()
	if f.file == nil {
		f.mu.RUnlock()
		f.mu.Lock()
		if f.file == nil {
			f.openCurrentFile()
		}
		f.mu.Unlock()
		f.mu.RLock()
	}
	if f.file == nil {
		f.mu.RUnlock()
		return 0, os.ErrClosed
	}
	n, err := f.file.Write(p)
	f.mu.RUnlock()
	return n, err
}

func (f *FileLogger) WriteLevel(_ zerolog.Level, p []byte) (int, error) {
	return f.Write(p)
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
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rotateLocked()
}

func (f *FileLogger) rotateLocked() {
	if f.file != nil {
		f.file.Close()
		f.file = nil
	}

	oldDate := f.currentDate
	compressJobs := make([][2]string, 0)
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
			compressJobs = append(compressJobs, [2]string{oldFile, gzPath})
		} else if nextFile != "" && i > 0 {
			os.Rename(oldFile, nextFile)
		}
	}

	f.currentDate = time.Now().Format("2006-01-02")
	f.openCurrentFile()
	for _, j := range compressJobs {
		src, dst := j[0], j[1]
		go func() {
			f.compressFile(src, dst)
			os.Remove(src)
		}()
	}
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