// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/certsource"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// Deps holds all external dependencies injected into the orchestration engine.
// All fields are required unless marked optional.
type Deps struct {
	// KeyProvider manages secp256k1 keys. Never nil.
	KeyProvider keyprovider.KeyProvider

	// CertSource manages X.509 certificate issuance. Never nil.
	CertSource certsource.CertSource

	// RelayRegistrar handles spoke registration with the Cacti relay. Never nil
	// (use NoOpRelayRegistrar for testing or when relay is not yet available).
	RelayRegistrar RelayRegistrar

	// ScriptsDir is the absolute path to deploy/local/paladin/scripts/ —
	// where the Go test scripts (TestDeployEVMRegistry, etc.) live.
	ScriptsDir string

	// ComposeTemplatePath is the absolute path to the Paladin compose template
	// (provisioning/templates/central-bank/paladin-compose.yaml).
	ComposeTemplatePath string

	// PaladinConfigTemplateDir is the absolute path to provisioning/templates/
	// central-bank/paladin-config/ — templates for Paladin node config.yaml.
	PaladinConfigTemplateDir string

	// BesuRPCURL is the HTTP RPC URL of the spoke's Besu node (e.g. http://localhost:8645).
	BesuRPCURL string

	// PaladinCBURL is the HTTP RPC URL of the Paladin central-bank node
	// (e.g. http://localhost:31648). Required by steps 5-8 that communicate with Paladin.
	PaladinCBURL string

	// Timeouts overrides the default per-step timeout values.
	// Zero values use the defaults defined in DefaultTimeouts.
	Timeouts Timeouts
}

// Timeouts configures per-step execution deadlines.
type Timeouts struct {
	// GoTestStep applies to steps 1, 4, 6, 7, 8 (subprocess go test). Default: 5m.
	GoTestStep time.Duration
	// PaladinHealthCheck applies to the polling loop in step 5. Default: 5m.
	PaladinHealthCheck time.Duration
	// PaladinHealthCheckInterval is the sleep between health check attempts. Default: 2s.
	PaladinHealthCheckInterval time.Duration
	// OnboardRegistry applies to step 9 (on-chain transaction). Default: 2m.
	OnboardRegistry time.Duration
	// RelayRegistration applies to step 10 (HTTP POST to relay). Default: 30s.
	RelayRegistration time.Duration
}

// DefaultTimeouts returns the default per-step timeout configuration.
func DefaultTimeouts() Timeouts {
	return Timeouts{
		GoTestStep:                 5 * time.Minute,
		PaladinHealthCheck:         5 * time.Minute,
		PaladinHealthCheckInterval: 2 * time.Second,
		OnboardRegistry:            2 * time.Minute,
		RelayRegistration:          30 * time.Second,
	}
}

// resolved returns a Timeouts with zero values replaced by defaults.
func (t Timeouts) resolved() Timeouts {
	d := DefaultTimeouts()
	if t.GoTestStep == 0 {
		t.GoTestStep = d.GoTestStep
	}
	if t.PaladinHealthCheck == 0 {
		t.PaladinHealthCheck = d.PaladinHealthCheck
	}
	if t.PaladinHealthCheckInterval == 0 {
		t.PaladinHealthCheckInterval = d.PaladinHealthCheckInterval
	}
	if t.OnboardRegistry == 0 {
		t.OnboardRegistry = d.OnboardRegistry
	}
	if t.RelayRegistration == 0 {
		t.RelayRegistration = d.RelayRegistration
	}
	return t
}

// SpokeInfo is the payload sent to the Cacti relay when registering a new spoke.
type SpokeInfo struct {
	SpokeID      string `json:"spoke_id"`
	BesuRPCURL   string `json:"besu_rpc_url"`
	BesuWSURL    string `json:"besu_ws_url"`
	HTLCAddress  string `json:"htlc_address"`
	GRPCEndpoint string `json:"grpc_endpoint"`
}

// RelayRegistrar abstracts spoke registration with the Cacti relay.
// The relay REST endpoint does not yet exist (RL-1); this interface allows
// TK-5 to be implemented and tested independently.
type RelayRegistrar interface {
	// Register registers the spoke with the relay.
	// Returns ErrRelayUnavailable if the relay cannot be reached.
	Register(ctx context.Context, spoke SpokeInfo) error

	// IsRegistered returns true if the spoke is already registered.
	// Returns false (not error) if the relay is unreachable.
	IsRegistered(ctx context.Context, spokeID string) (bool, error)
}

// ErrRelayUnavailable is returned by the relay registrar when the relay cannot be reached.
var ErrRelayUnavailable = errors.New("orchestrator: relay unavailable")

// NoOpRelayRegistrar is a no-op implementation for testing and pre-RL-1 deployments.
// Register always returns nil; IsRegistered always returns false.
type NoOpRelayRegistrar struct{}

func (NoOpRelayRegistrar) Register(_ context.Context, _ SpokeInfo) error          { return nil }
func (NoOpRelayRegistrar) IsRegistered(_ context.Context, _ string) (bool, error) { return false, nil }

// httpRelayRegistrar is the default HTTP implementation of RelayRegistrar.
type httpRelayRegistrar struct {
	endpoint   string
	httpClient *http.Client
}

// NewHTTPRelayRegistrar creates a relay registrar targeting the given base URL.
func NewHTTPRelayRegistrar(endpoint string) RelayRegistrar {
	return &httpRelayRegistrar{
		endpoint:   strings.TrimRight(endpoint, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *httpRelayRegistrar) Register(ctx context.Context, spoke SpokeInfo) error {
	body, err := json.Marshal(spoke)
	if err != nil {
		return fmt.Errorf("relay: marshal spoke info: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.endpoint+"/api/v1/spokes", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("relay: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRelayUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return fmt.Errorf("%w: endpoint not yet implemented (RL-1)", ErrRelayUnavailable)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("relay: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (r *httpRelayRegistrar) IsRegistered(ctx context.Context, spokeID string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		r.endpoint+"/api/v1/spokes/"+spokeID, nil)
	if err != nil {
		return false, fmt.Errorf("relay: create request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		// Relay unreachable → not registered (soft fallback per R-05).
		return false, nil
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
