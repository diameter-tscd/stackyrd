package plugin

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/spf13/afero"
)

type TSCache struct {
	cacheDir string
	mu       sync.RWMutex
}

func NewTSCache(cacheDir string) *TSCache {
	return &TSCache{
		cacheDir: cacheDir,
	}
}

func (c *TSCache) cachePath(hash string) string {
	return filepath.Join(c.cacheDir, hash+".js")
}

func (c *TSCache) Compile(fs afero.Fs, path string, source []byte) ([]byte, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256(source))
	cachePath := c.cachePath(hash)

	c.mu.RLock()
	cached, err := afero.ReadFile(fs, cachePath)
	c.mu.RUnlock()
	if err == nil {
		return cached, nil
	}

	loaderTS := api.LoaderTS
	result := api.Transform(string(source), api.TransformOptions{
		Loader: loaderTS,
		Target: api.ES2020,
	})

	if len(result.Errors) > 0 {
		errMsg := "esbuild transform errors:"
		for _, e := range result.Errors {
			errMsg += fmt.Sprintf("\n  %s", e.Text)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	compiled := []byte(result.Code)

	c.mu.Lock()
	defer c.mu.Unlock()
	if err := afero.WriteFile(fs, cachePath, compiled, 0644); err != nil {
		return compiled, nil
	}

	return compiled, nil
}
