package utils

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// isLoopback reports whether host resolves to a loopback address.
func isLoopback(host string) bool {
	if host == "localhost" || host == "" {
		return true
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return false
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip != nil && ip.IsLoopback() {
			return true
		}
	}
	return false
}

// LoadConfigFromURL loads configuration from a remote URL using HTTP GET
func LoadConfigFromURL(configURL string) error {
	// Restrict redirects and time the request so a hostile URL cannot probe
	// internal services (SSRF) or hang startup indefinitely.
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return fmt.Errorf("too many redirects")
			}
			host := req.URL.Hostname()
			if isLoopback(host) {
				return fmt.Errorf("redirect to loopback address blocked: %s", host)
			}
			return nil
		},
	}
	resp, err := client.Get(configURL)
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
