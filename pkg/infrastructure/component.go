package infrastructure

import (
"stackyard/config"
"stackyard/pkg/logger"
)

// InfrastructureComponent defines the interface that all infrastructure managers must implement
type InfrastructureComponent interface {
	// Name returns the display name of the component
	Name() string
	
	// Close gracefully shuts down the component
	Close() error
	
	// GetStatus returns the current status of the component
	GetStatus() map[string]interface{}
}

// ComponentFactory is a function that creates an infrastructure component
type ComponentFactory func(cfg *config.Config, logger *logger.Logger) (InfrastructureComponent, error)
