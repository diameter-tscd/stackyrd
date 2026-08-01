package utils

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// isRestrictedIP reports whether the IP is a loopback, private (RFC1918),
// link-local, unspecified, or multicast address — i.e. anything that could be
// used to probe internal infrastructure from a remote config URL.
func isRestrictedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast()
}

// isRestrictedHost reports whether host resolves to a restricted address.
func isRestrictedHost(host string) bool {
	if host == "localhost" || host == "" || strings.HasPrefix(host, "127.") {
		return true
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		// Unresolvable: fail closed rather than let a later resolution land on
		// an unexpected internal address.
		return true
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip != nil && isRestrictedIP(ip) {
			return true
		}
	}
	return false
}

// validateConfigURL rejects URLs pointing at restricted/internal hosts so a
// hostile config URL cannot probe infrastructure (SSRF).
func validateConfigURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid config URL: %w", err)
	}
	host := u.Hostname()
	if isRestrictedHost(host) {
		return fmt.Errorf("config URL host not allowed (SSRF guard): %s", host)
	}
	return nil
}

// LoadConfigFromURL loads configuration from a remote URL using HTTP GET
func LoadConfigFromURL(configURL string) error {
	if err := validateConfigURL(configURL); err != nil {
		return err
	}

	// Restrict redirects and time the request so a hostile URL cannot probe
	// internal services (SSRF) or hang startup indefinitely.
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 {
				return fmt.Errorf("too many redirects")
			}
			host := req.URL.Hostname()
			if isRestrictedHost(host) {
				return fmt.Errorf("redirect to restricted address blocked: %s", host)
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
