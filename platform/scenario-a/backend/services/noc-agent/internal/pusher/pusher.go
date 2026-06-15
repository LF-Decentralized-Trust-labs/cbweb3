// SPDX-License-Identifier: Apache-2.0

// Package pusher sends collected health data to the NOC backend.
package pusher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-agent/internal/collector"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-agent/internal/config"
)

// Pusher sends push payloads to the NOC backend.
type Pusher struct {
	cfg        *config.AgentConfig
	httpClient *http.Client
}

// New creates a Pusher.
func New(cfg *config.AgentConfig) *Pusher {
	return &Pusher{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type componentPayload struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Endpoint    string  `json:"endpoint"`
	Status      string  `json:"status"`
	BlockNumber *int64  `json:"block_number,omitempty"`
	Diagnostic  *string `json:"diagnostic,omitempty"`
}

type logLinePayload struct {
	ComponentName string    `json:"component_name"`
	Stream        string    `json:"stream"`
	LogLine       string    `json:"log_line"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type pushPayload struct {
	SpokeID             string             `json:"spoke_id"`
	AgentName           string             `json:"agent_name"`
	PushIntervalSeconds int                `json:"push_interval_seconds"`
	CollectedAt         time.Time          `json:"collected_at"`
	Components          []componentPayload `json:"components"`
	LogLines            []logLinePayload   `json:"log_lines,omitempty"`
}

// Push sends results to the backend /internal/v1/push endpoint.
func (p *Pusher) Push(ctx context.Context, results []collector.CheckResult) error {
	components := make([]componentPayload, 0, len(results))
	logLines := make([]logLinePayload, 0)

	for _, r := range results {
		components = append(components, componentPayload{
			Name:        r.Name,
			Type:        r.Type,
			Endpoint:    r.Endpoint,
			Status:      r.Status,
			BlockNumber: r.BlockNumber,
			Diagnostic:  r.Diagnostic,
		})
		for _, ll := range r.LogLines {
			logLines = append(logLines, logLinePayload{
				ComponentName: r.Name,
				Stream:        ll.Stream,
				LogLine:       ll.Text,
				OccurredAt:    ll.OccurredAt,
			})
		}
	}

	payload := pushPayload{
		SpokeID:             p.cfg.SpokeID,
		AgentName:           agentName(p.cfg),
		PushIntervalSeconds: p.cfg.PushIntervalSeconds,
		CollectedAt:         time.Now(),
		Components:          components,
		LogLines:            logLines,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pusher: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.cfg.NocBackendURL+"/internal/v1/push", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("pusher: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Key", p.cfg.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pusher: push request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("pusher: backend returned %d", resp.StatusCode)
	}
	return nil
}

func agentName(cfg *config.AgentConfig) string {
	return fmt.Sprintf("noc-agent-%s", cfg.SpokeID)
}
