package utils

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// FlagDefinition represents a command-line flag definition
type FlagDefinition struct {
	Name         string                        // Flag name (without dash)
	DefaultValue interface{}                   // Default value
	Description  string                        // Help text
	Validator    func(value interface{}) error // Optional validation function
}

// ParsedFlags holds the parsed flag values
type ParsedFlags struct {
	ConfigURL string // -c flag value
	Port      string // -port flag value
	Verbose   bool   // -verbose flag value
	Env       string // -env flag value
	// Add new flags here as needed
}

// ParseFlags parses command line flags based on provided definitions and returns structured flag values
func ParseFlags(flagDefinitions []FlagDefinition) (*ParsedFlags, error) {
	parsed := &ParsedFlags{}

	// Create a map to hold flag pointers
	flagPtrs := make(map[string]interface{})

	// Dynamically define flags based on flagDefinitions
	for _, def := range flagDefinitions {
		switch v := def.DefaultValue.(type) {
		case string:
			flagPtrs[def.Name] = flag.String(def.Name, v, def.Description)
		case int:
			flagPtrs[def.Name] = flag.Int(def.Name, v, def.Description)
		case bool:
			flagPtrs[def.Name] = flag.Bool(def.Name, v, def.Description)
		default:
			return nil, fmt.Errorf("unsupported flag type for %s: %T", def.Name, v)
		}
	}

	// Parse the flags
	flag.Parse()

	// Extract values and validate
	for _, def := range flagDefinitions {
		var value interface{}

		switch ptr := flagPtrs[def.Name].(type) {
		case *string:
			value = *ptr
			switch def.Name {
			case "c":
				parsed.ConfigURL = *ptr
			case "port":
				parsed.Port = *ptr
			case "env":
				parsed.Env = *ptr
			}
			// Add new string flag assignments here
		case *int:
			value = *ptr
			// Add new int flag assignments here
		case *bool:
			value = *ptr
			if def.Name == "verbose" {
				parsed.Verbose = *ptr
			}
			// Add new bool flag assignments here
		}

		// Validate the value if validator is provided
		if def.Validator != nil {
			if err := def.Validator(value); err != nil {
				return nil, fmt.Errorf("flag -%s validation failed: %w", def.Name, err)
			}
		}
	}

	return parsed, nil
}

// LoadConfigFromURL loads configuration from a remote URL using HTTP GET
func LoadConfigFromURL(configURL string) error {
	// Make HTTP GET request to fetch the config
	resp, err := http.Get(configURL)
	if err != nil {
		return fmt.Errorf("failed to fetch config from URL %s: %w", configURL, err)
	}
	defer resp.Body.Close()

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch config from URL %s: HTTP %d %s", configURL, resp.StatusCode, resp.Status)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !contains(contentType, "yaml") && !contains(contentType, "yml") {
		fmt.Fprintf(os.Stderr, "Warning: Content-Type '%s' does not indicate YAML format\n", contentType)
	}

	// Read the response body and set it as config
	if err := viper.ReadConfig(resp.Body); err != nil {
		return fmt.Errorf("failed to parse config from URL %s: %w", configURL, err)
	}

	return nil
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// PrintUsage prints the usage information for command line flags based on provided definitions
func PrintUsage(flagDefinitions []FlagDefinition, appName string) {
	fmt.Printf("Usage of %s:\n", appName)
	for _, def := range flagDefinitions {
		switch def.DefaultValue.(type) {
		case string:
			fmt.Printf("  -%s string\n", def.Name)
		case int:
			fmt.Printf("  -%s int\n", def.Name)
		case bool:
			fmt.Printf("  -%s\n", def.Name)
		}
		fmt.Printf("        %s", def.Description)
		if def.DefaultValue != "" && def.DefaultValue != false && def.DefaultValue != 0 {
			fmt.Printf(" (default %v)", def.DefaultValue)
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  ./%-40s # Load config from local config.yaml\n", appName)
	fmt.Printf("  ./%s -c http://example.com/config.yaml\n", appName)
	fmt.Printf("  ./%s -port 9090 -env production\n", appName)
	fmt.Printf("  ./%s -c https://config.example.com/app.yaml -verbose\n", appName)
	fmt.Println()
}
