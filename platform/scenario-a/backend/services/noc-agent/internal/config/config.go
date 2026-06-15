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

// Load reads and validates the agent.yaml at the given path.
func Load(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
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

// ConfigPath returns the path to agent.yaml from the AGENT_CONFIG_PATH env var,
// defaulting to /etc/noc-agent/agent.yaml.
func ConfigPath() string {
	if v := os.Getenv("AGENT_CONFIG_PATH"); v != "" {
		return v
	}
	return "/etc/noc-agent/agent.yaml"
}
