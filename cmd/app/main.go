package main

import (
	"fmt"
	"net/url"
	"os"

	"stackyrd-nano/pkg/utils"
)

func main() {
	flags := parseFlags()

	configManager := NewConfigManager(flags.ConfigURL)

	app := NewApplication(configManager)

	if err := app.Run(); err != nil {
		fmt.Printf("Fatal error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() *utils.ParsedFlags {
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

	flags, err := utils.ParseFlags(flagDefinitions)
	if err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		utils.PrintUsage(flagDefinitions, AppName)
		os.Exit(1)
	}

	return flags
}
