package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestSaveTheme(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "config.yaml")
	content := "app:\n  name: \"test\"\n  theme: \"default\"  # TUI color theme\nserver:\n  port: \"8080\"\n"
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	viper.SetConfigFile(src)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatal(err)
	}

	if err := SaveTheme("ocean_blue"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)
	if !strings.Contains(out, `theme: "ocean_blue" # TUI color theme`) {
		t.Fatalf("theme line not updated:\n%s", out)
	}
	if !strings.Contains(out, "server:") || !strings.Contains(out, `port: "8080"`) {
		t.Fatalf("config file mangled:\n%s", out)
	}
}

func TestSaveThemeNoFile(t *testing.T) {
	viper.Reset()
	if err := SaveTheme("ocean_blue"); err == nil {
		t.Fatal("expected error when no config file is loaded")
	}
}
