// This file bootstraps the API Gateway process and starts the Fiber server.
package main

import (
	"log"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/app"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
)

// main is the entry point for the API Gateway binary.
// It loads runtime configuration, builds the HTTP app with all dependencies,
// and starts listening on the configured port.
func main() {
	// Load environment-driven configuration (port, auth mode, timeouts, etc.).
	cfg := config.Load()

	// Build the Fiber app and wire adapters, handlers, and middleware.
	server, err := app.New(cfg)
	if err != nil {
		log.Fatalf("failed to initialize api-gateway: %v", err)
	}

	// Start the HTTP server and keep the process alive until it exits.
	log.Printf("api-gateway listening on :%s (mode=%s)", cfg.AppPort, cfg.AuthMode)
	if err := server.Listen(":" + cfg.AppPort); err != nil {
		log.Fatalf("server stopped with error: %v", err)
	}
}

