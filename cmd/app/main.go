package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
)

// main is the entry point of the application
func main() {
	// Parse command line flags
	configURL := parseFlags()

	// Create configuration manager
	configManager := NewConfigManager(configURL)

	// Create application with dependency injection
	app := NewApplication(configManager)

	// Run application with error handling
	if err := app.Run(); err != nil {
		fmt.Printf("Fatal error: %v\n", err)
		os.Exit(1)
	}
}

// parseFlags parses command line flags using standard Go flag package
func parseFlags() string {
	var configURL string
	flag.StringVar(&configURL, "c", "", "URL to load configuration from (YAML format)")
	flag.Parse()

	// Validate URL if provided
	if configURL != "" {
		if _, err := url.ParseRequestURI(configURL); err != nil {
			fmt.Printf("Invalid config URL format: %v\n", err)
			fmt.Println("Usage: stackyard [-c config-url]")
			os.Exit(1)
		}
	}

	return configURL
}
