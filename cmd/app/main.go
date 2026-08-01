package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
)

// @title stackyrd API
// @version 1.0
// @description stackyrd API Documentation - A modular service framework for Go.
// @termsOfService http://swagger.io/terms/

// @license.name Apache 2.0
// @license.url https://github.com/diameter-tscd/stackyrd/blob/master/LICENSE

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

func main() {
	var configURL, port, env string
	var verbose bool
	flag.StringVar(&configURL, "c", "", "URL to load configuration from (YAML format)")
	flag.StringVar(&port, "port", "", "Server port (overrides config)")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&env, "env", "", "Environment (development/staging/production)")
	flag.Parse()

	if configURL != "" {
		if _, err := url.ParseRequestURI(configURL); err != nil {
			fmt.Fprintf(os.Stderr, "invalid config URL format: %v\n", err)
			flag.Usage()
			os.Exit(1)
		}
	}

	_ = verbose

	configManager := NewConfigManager(configURL, port, env)

	app := NewApplication(configManager)

	if err := app.Run(); err != nil {
		fmt.Printf("Fatal error: %v\n", err)
		os.Exit(1)
	}
}
