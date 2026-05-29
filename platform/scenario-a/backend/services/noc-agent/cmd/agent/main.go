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
