package utils

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// LoadConfigFromURL loads configuration from a remote URL using HTTP GET
func LoadConfigFromURL(configURL string) error {
	resp, err := http.Get(configURL)
	if err != nil {
		return fmt.Errorf("failed to fetch config from URL %s: %w", configURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch config from URL %s: HTTP %d %s", configURL, resp.StatusCode, resp.Status)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "yaml") && !strings.Contains(contentType, "yml") {
		fmt.Fprintf(os.Stderr, "Warning: Content-Type '%s' does not indicate YAML format\n", contentType)
	}

	if err := viper.ReadConfig(resp.Body); err != nil {
		return fmt.Errorf("failed to parse config from URL %s: %w", configURL, err)
	}

	return nil
}
