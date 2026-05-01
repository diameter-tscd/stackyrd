package main

import (
	"fmt"
	"net/url"
	"os"

	"stackyrd/pkg/utils"
)

// @title stackyrd API
// @version 1.0
// @description stackyrd API Documentation
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email admin@stackyrd.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

// main is the entry point of the application
func main() {
	// Parse command line flags
	flags := parseFlags()

	// Create configuration manager
	configManager := NewConfigManager(flags.ConfigURL)

	// Create application with dependency injection
	app := NewApplication(configManager)

	// Run application with error handling
	if err := app.Run(); err != nil {
		fmt.Printf("Fatal error: %v\n", err)
		os.Exit(1)
	}
}

// parseFlags parses command line flags using the parameter utility
func parseFlags() *utils.ParsedFlags {
	// Define flag definitions
	flagDefinitions := []utils.FlagDefinition{
		{
			Name:         "c",
			DefaultValue: "",
			Description:  "URL to load configuration from (YAML format)",
			Validator: func(value interface{}) error {
				if urlStr, ok := value.(string); ok && urlStr != "" {
					if _, err := url.ParseRequestURI(urlStr); err != nil {
						return fmt.Errorf("invalid config URL format: %w", err)
					}
				}
				return nil
			},
		},
		{
			Name:         "port",
			DefaultValue: "",
			Description:  "Server port (overrides config)",
		},
		{
			Name:         "verbose",
			DefaultValue: false,
			Description:  "Enable verbose logging",
		},
		{
			Name:         "env",
			DefaultValue: "",
			Description:  "Environment (development/staging/production)",
		},
	}

	// Parse flags using the utility
	flags, err := utils.ParseFlags(flagDefinitions)
	if err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		utils.PrintUsage(flagDefinitions, AppName)
		os.Exit(1)
	}

	return flags
}
