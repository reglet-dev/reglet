package core

import (
	"context"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

// ServiceHandler is a function that handles a specific AWS service operation.
type ServiceHandler func(ctx context.Context, client *AWSClient, cfg *AWSConfig) (entities.Result, error)

// serviceHandlers maps service names to their handlers.
// Handlers are registered via init() in each service file (iam.go, ec2.go, etc.).
var serviceHandlers = map[string]map[string]ServiceHandler{
	// Populated at init time
}

// RouteToService routes the request to the appropriate service handler.
func RouteToService(ctx context.Context, client *AWSClient, cfg *AWSConfig) (entities.Result, error) {
	// Get service handlers
	handlers, ok := serviceHandlers[cfg.Service]
	if !ok {
		return entities.ResultFailure(
			fmt.Sprintf("Unsupported service: %s", cfg.Service),
			map[string]any{"service": cfg.Service},
		), nil
	}

	// Get operation handler
	handler, ok := handlers[cfg.Operation]
	if !ok {
		return entities.ResultFailure(
			fmt.Sprintf("Unsupported operation: %s for service %s", cfg.Operation, cfg.Service),
			map[string]any{"service": cfg.Service, "operation": cfg.Operation},
		), nil
	}

	// Execute handler
	return handler(ctx, client, cfg)
}

// RegisterServiceHandler registers a handler for a service operation.
// Called by service files (iam.go, ec2.go, etc.) during init().
func RegisterServiceHandler(service, operation string, handler ServiceHandler) {
	if serviceHandlers[service] == nil {
		serviceHandlers[service] = make(map[string]ServiceHandler)
	}
	serviceHandlers[service][operation] = handler
}
