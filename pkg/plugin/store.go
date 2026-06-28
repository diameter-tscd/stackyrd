package plugin

import (
	"embed"
	"os"
	"path/filepath"

	"stackyrd/pkg/infrastructure"

	"github.com/spf13/afero"
)

func buildPluginFS(embedded embed.FS, prefix string, storeDir string) afero.Fs {
	baseFS := &infrastructure.EmbedFSAdapter{FS: embedded}
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
