// Package collector performs health checks against infrastructure components.
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-agent/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-agent/internal/logs"
)

// CheckResult carries the outcome of a single component health check.
type CheckResult struct {
	Name        string
	Type        string
	Endpoint    string
	Status      string  // HEALTHY | DEGRADED | OFFLINE | UNKNOWN
	BlockNumber *int64
	Diagnostic  *string
	LogLines    []logs.LogLine
	CheckedAt   time.Time
}

// Checker runs health checks for a component.
type Checker struct {
	httpClient *http.Client
	docker     *logs.DockerClient
}

// New creates a Checker with a configured HTTP client and Docker client.
func New(docker *logs.DockerClient) *Checker {
	return &Checker{
		httpClient: &http.Client{Timeout: 8 * time.Second},
		docker:     docker,
	}
}

// Check performs a health check for the given component configuration.
func (ch *Checker) Check(ctx context.Context, comp config.ComponentConfig, prevBlockNumber *int64) CheckResult {
	result := CheckResult{
		Name:      comp.Name,
		Type:      comp.Type,
		Endpoint:  comp.Endpoint,
		CheckedAt: time.Now(),
	}

	switch comp.Type {
	case "BESU":
		result = ch.checkBesu(ctx, comp, prevBlockNumber, result)
	case "CACTI_RELAY":
		result = ch.checkCactiRelay(ctx, comp, result)
	case "PALADIN":
		result = ch.checkPaladin(ctx, comp, result)
	default:
		result.Status = "UNKNOWN"
	}

	// Collect logs
	if comp.ContainerName != "" && ch.docker != nil {
		lines, err := ch.collectLogs(ctx, comp.ContainerName)
		if err == nil {
			result.LogLines = lines
		}
	}

	return result
}

func (ch *Checker) checkBesu(ctx context.Context, comp config.ComponentConfig, prevBlock *int64, r CheckResult) CheckResult {
	blockNum, err := eth_blockNumber(ctx, ch.httpClient, comp.Endpoint)
	if err != nil {
		r.Status = "OFFLINE"
		diag := err.Error()
		r.Diagnostic = &diag
		return r
	}

	r.BlockNumber = &blockNum

	if prevBlock != nil && *prevBlock == blockNum {
		r.Status = "DEGRADED"
		diag := fmt.Sprintf("block number unchanged: %d", blockNum)
		r.Diagnostic = &diag
	} else {
		r.Status = "HEALTHY"
	}
	return r
}

func (ch *Checker) checkCactiRelay(ctx context.Context, comp config.ComponentConfig, r CheckResult) CheckResult {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, comp.Endpoint+"/api/v1/plugins/@hyperledger/cactus-plugin-ledger-connector-besu/get-block", nil)
	if err != nil {
		r.Status = "OFFLINE"
		return r
	}
	resp, err := ch.httpClient.Do(req)
	if err != nil {
		r.Status = "OFFLINE"
		diag := err.Error()
		r.Diagnostic = &diag
		return r
	}
	defer resp.Body.Close()
	latency := time.Since(start)

	if resp.StatusCode >= 400 {
		r.Status = "OFFLINE"
		diag := fmt.Sprintf("HTTP %d", resp.StatusCode)
		r.Diagnostic = &diag
		return r
	}
	if latency > 5*time.Second {
		r.Status = "DEGRADED"
		diag := fmt.Sprintf("latency %.1fs > 5s threshold", latency.Seconds())
		r.Diagnostic = &diag
	} else {
		r.Status = "HEALTHY"
	}
	return r
}

func (ch *Checker) checkPaladin(ctx context.Context, comp config.ComponentConfig, r CheckResult) CheckResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, comp.Endpoint+"/api/v1/status", nil)
	if err != nil {
		r.Status = "OFFLINE"
		return r
	}
	resp, err := ch.httpClient.Do(req)
	if err != nil {
		r.Status = "OFFLINE"
		diag := err.Error()
		r.Diagnostic = &diag
		return r
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		r.Status = "OFFLINE"
		diag := fmt.Sprintf("HTTP %d", resp.StatusCode)
		r.Diagnostic = &diag
		return r
	}

	// Check for PD020704 in response (Paladin healthy indicator)
	if !strings.Contains(string(body), "PD020704") {
		r.Status = "DEGRADED"
		diag := "PD020704 not found in response"
		r.Diagnostic = &diag
	} else {
		r.Status = "HEALTHY"
	}
	return r
}

func (ch *Checker) collectLogs(ctx context.Context, containerName string) ([]logs.LogLine, error) {
	id, err := ch.docker.ContainerIDByName(ctx, containerName)
	if err != nil {
		return nil, err
	}
	return ch.docker.TailLogs(ctx, id, 50)
}

// eth_blockNumber calls eth_blockNumber JSON-RPC and returns the current block number.
func eth_blockNumber(ctx context.Context, client *http.Client, endpoint string) (int64, error) {
	body := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("eth_blockNumber: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("eth_blockNumber: decode: %w", err)
	}

	var blockNum int64
	if _, err := fmt.Sscanf(result.Result, "0x%x", &blockNum); err != nil {
		// Try decimal
		if _, err2 := fmt.Sscanf(result.Result, "%d", &blockNum); err2 != nil {
			return 0, fmt.Errorf("eth_blockNumber: parse %q: %w", result.Result, err)
		}
	}
	return blockNum, nil
}
