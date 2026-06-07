// Package infrastructure provides auto-registered infrastructure component abstractions (databases, queues, storage, etc.).
package infrastructure

import (
	"context"

	"github.com/diameter-tscd/stackyrd/config"
	"github.com/diameter-tscd/stackyrd/pkg/logger"
)

// InfrastructureComponent defines the interface that all infrastructure managers must implement
type InfrastructureComponent interface {
	// Name returns the display name of the component
	Name() string

	// Close gracefully shuts down the component
	Close(ctx context.Context) error

	// GetStatus returns the current status of the component
	GetStatus(ctx context.Context) map[string]interface{}
}

// ComponentFactory is a function that creates an infrastructure component
type ComponentFactory func(cfg *config.Config, logger *logger.Logger) (InfrastructureComponent, error)
