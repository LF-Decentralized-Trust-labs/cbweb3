// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-agent/internal/collector"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-agent/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-agent/internal/logs"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-agent/internal/pusher"
)

func main() {
	cfgPath := config.ConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	socketPath := os.Getenv("DOCKER_SOCKET")
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	dockerClient := logs.New(socketPath)
	// Report a socket the agent cannot use once, at startup. Without this the failure is
	// invisible: the agent runs as a non-root uid (finding R2-M-12), the socket is
	// normally root:docker mode 0660, and collectLogs discards the resulting error — so
	// the Log Viewer goes empty with nothing here to explain it. Health collection does
	// not use the socket, so this reports and carries on rather than exiting.
	if err := dockerClient.Ping(context.Background()); err != nil {
		log.Printf("ERROR: docker socket %s is not usable: %v", socketPath, err)
		log.Printf("ERROR: container logs will be empty. This agent runs as a non-root uid; " +
			"give the container the socket's group (compose group_add / NOC_DOCKER_GID, or " +
			"docker run --group-add). Health collection is unaffected.")
	}
	chk := collector.New(dockerClient)
	push := pusher.New(cfg)

	// prevBlockNumbers tracks last block per component to detect BESU stalls.
	prevBlockNumbers := make(map[string]*int64)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	interval := time.Duration(cfg.PushIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("noc-agent started for spoke=%s, interval=%ds, components=%d",
		cfg.SpokeID, cfg.PushIntervalSeconds, len(cfg.Components))

	for {
		select {
		case <-quit:
			log.Println("shutting down")
			return

		case <-ticker.C:
			results := make([]collector.CheckResult, 0, len(cfg.Components))
			for _, comp := range cfg.Components {
				prev := prevBlockNumbers[comp.Name]
				r := chk.Check(ctx, comp, prev)
				results = append(results, r)
				if r.BlockNumber != nil {
					bn := *r.BlockNumber
					prevBlockNumbers[comp.Name] = &bn
				}
			}

			if err := push.Push(ctx, results); err != nil {
				log.Printf("push failed: %v", err)
			} else {
				log.Printf("pushed %d components to backend", len(results))
			}
		}
	}
}
