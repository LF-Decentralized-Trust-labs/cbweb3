// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ComponentConfig describes a single monitorable component.
type ComponentConfig struct {
	Name          string `yaml:"name"`
	Type          string `yaml:"type"`           // BESU | CACTI_RELAY | PALADIN | PAYMENT_ORCHESTRATOR
	Endpoint      string `yaml:"endpoint"`
	ContainerName string `yaml:"container_name"` // Docker container name for log collection
}

// AgentConfig is the top-level agent.yaml structure.
type AgentConfig struct {
	SpokeID             string            `yaml:"spoke_id"`
	NocBackendURL       string            `yaml:"noc_backend_url"`
	APIKey              string            `yaml:"api_key"`
	PushIntervalSeconds int               `yaml:"push_interval_seconds"`
	Components          []ComponentConfig `yaml:"components"`
}

// Load reads and validates the agent.yaml at the given path. When the file is
// absent it falls back to environment variables (the toolkit's parametrized deploy
// wires the agent per entity via env, with no per-entity agent.yaml to render/mount).
func Load(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if cfg, ok := loadFromEnv(); ok {
				return cfg, nil
			}
		}
		return nil, fmt.Errorf("config: reading %s: %w", path, err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parsing %s: %w", path, err)
	}

	if cfg.SpokeID == "" {
		return nil, fmt.Errorf("config: spoke_id is required")
	}
	if cfg.NocBackendURL == "" {
		return nil, fmt.Errorf("config: noc_backend_url is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("config: api_key is required")
	}
	if cfg.PushIntervalSeconds <= 0 {
		cfg.PushIntervalSeconds = 15
	}

	return &cfg, nil
}

// loadFromEnv builds the agent config from environment variables, used when no
// agent.yaml is mounted (toolkit deploy). SpokeID/backend/api_key are required; a
// single BESU component is derived from BESU_RPC_URL when present.
func loadFromEnv() (*AgentConfig, bool) {
	spoke := os.Getenv("AGENT_ENTITY")
	if spoke == "" {
		spoke = os.Getenv("AGENT_SPOKE_ID")
	}
	backend := os.Getenv("NOC_BACKEND_URL")
	apiKey := os.Getenv("AGENT_API_KEY")
	if spoke == "" || backend == "" || apiKey == "" {
		return nil, false
	}
	cfg := &AgentConfig{
		SpokeID:             spoke,
		NocBackendURL:       backend,
		APIKey:              apiKey,
		PushIntervalSeconds: 15,
	}
	if besu := os.Getenv("BESU_RPC_URL"); besu != "" {
		cfg.Components = append(cfg.Components, ComponentConfig{
			Name:          "besu",
			Type:          "BESU",
			Endpoint:      besu,
			ContainerName: os.Getenv("AGENT_BESU_CONTAINER"),
		})
	}
	return cfg, true
}

// ConfigPath returns the path to agent.yaml from the AGENT_CONFIG_PATH env var,
// defaulting to /etc/noc-agent/agent.yaml.
func ConfigPath() string {
	if v := os.Getenv("AGENT_CONFIG_PATH"); v != "" {
		return v
	}
	return "/etc/noc-agent/agent.yaml"
}
