// SPDX-License-Identifier: Apache-2.0

// This file bootstraps the API Gateway process and starts the Fiber server.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/app"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/config"
)

func main() {
	cfg := config.Load()

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("failed to initialize api-gateway: %v", err)
	}

	// Listen for OS termination signals for graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("api-gateway listening on :%s", cfg.AppPort)
		if err := application.Fiber.Listen(":" + cfg.AppPort); err != nil {
			log.Fatalf("server stopped with error: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down api-gateway …")

	if err := application.Shutdown(); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Println("api-gateway stopped")
}
