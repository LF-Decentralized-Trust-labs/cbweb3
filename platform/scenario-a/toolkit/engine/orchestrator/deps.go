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

	// CentralBankComposePath is the absolute path to the TK-4 central-bank Besu
	// compose template (provisioning/templates/central-bank/docker-compose.yaml).
	// Used by the start-besu step (mode:found) to bring up the spoke bootnode.
	CentralBankComposePath string

	// BesuImage is the pinned Besu Docker image for the bootnode
	// (e.g. hyperledger/besu:25.8.0). Distinct from manifest.spec.image, which
	// selects the backend service image source, not the Besu image.
	BesuImage string

	// PaladinImage is the pinned Paladin Docker image for the spoke's Paladin
	// nodes (e.g. docker.io/lfdecentralizedtrust/paladin:v0.15.0-rc.1).
	PaladinImage string

	// ContractsOutDir is the Foundry build output dir (contracts/out) from which
	// the engine reads the IdentityRegistry.sol artifact to deploy the participant
	// whitelist natively (onboard-registry / FR-018).
	ContractsOutDir string

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

// JoinDeps holds all external dependencies injected into the mode:join engine
// (RunJoin). All fields are required unless marked optional.
type JoinDeps struct {
	// KeyProvider manages the commercial bank's blockchain secp256k1 key. Never nil.
	KeyProvider keyprovider.KeyProvider

	// BankCode is the resolved identifier for this commercial bank (CSR subject CN,
	// IdentityRegistry name, BANK_ID compose variable). Never empty.
	BankCode string

	// Institution is the bank's legal name for the CSR subject O= field.
	Institution string

	// BesuRPCURL is the HTTP RPC URL of the joining bank's own Besu node.
	BesuRPCURL string

	// ComposeTemplatePath is the absolute path to the commercial-bank docker-compose.yaml.
	ComposeTemplatePath string

	// BackendComposePath is the absolute path to the bank's backend docker-compose file.
	// Optional: when empty, the start-backend step is skipped (logged).
	BackendComposePath string

	// PaladinComposePath is the commercial-bank Paladin compose template (US2).
	PaladinComposePath string
	// PaladinConfigTemplateDir is provisioning/templates/central-bank/paladin-config
	// (the bank/config.yaml.tmpl lives there). PaladinImage is the pinned image.
	PaladinConfigTemplateDir string
	PaladinImage             string
	// BesuRPCPort / BesuWSPort are the bank's Besu host ports (for the bank Paladin
	// config and to derive the bank Paladin host ports).
	BesuRPCPort int
	BesuWSPort  int
	// ContractsOutDir is the Foundry build output dir (for the FXAgreement artifact,
	// US3 deploy-fxa-pente).
	ContractsOutDir string

	// Timeouts overrides the default per-step timeout values. Zero values use defaults.
	Timeouts JoinTimeouts
}

// JoinTimeouts configures per-step execution deadlines for mode:join.
type JoinTimeouts struct {
	// WaitSync is the maximum time to wait for the node to sync to the network. Default: 3m.
	WaitSync time.Duration
	// WaitSyncInterval is the polling interval for sync progress. Default: 5s.
	WaitSyncInterval time.Duration
	// VoteQBFT is the maximum time to wait for validator-set activation. Default: 5m.
	VoteQBFT time.Duration
	// VoteQBFTInterval is the polling interval for activation. Default: 3s.
	VoteQBFTInterval time.Duration
	// RequestCert is the timeout for the HTTP POST to cbEndpoint. Default: 30s.
	RequestCert time.Duration
	// ReceiveCert is the maximum polling time for the signed certificate. Default: 5m.
	ReceiveCert time.Duration
	// ReceiveCertInterval is the polling interval for the signed cert. Default: 10s.
	ReceiveCertInterval time.Duration
	// ProofOfPossession is the timeout for the IdentityRegistry transaction. Default: 2m.
	ProofOfPossession time.Duration
	// PenteFXSetup is the budget for the Pente FX-context steps (create-pente-context and
	// deploy-fxa-pente). These run multiple SEQUENTIAL private transactions that each require
	// cross-node endorsement (in-group IdentityRegistry deploy, then register + verify for each
	// participant, FXAgreement deploy); a single Paladin peer-transport reconnect during the new node's initial
	// mesh can stall one endorsement for minutes, so the whole sequence needs generous headroom.
	// Default: 20m.
	PenteFXSetup time.Duration
}

// DefaultJoinTimeouts returns the default per-step timeout configuration for mode:join.
func DefaultJoinTimeouts() JoinTimeouts {
	return JoinTimeouts{
		WaitSync:            3 * time.Minute,
		WaitSyncInterval:    5 * time.Second,
		VoteQBFT:            5 * time.Minute,
		VoteQBFTInterval:    3 * time.Second,
		RequestCert:         30 * time.Second,
		ReceiveCert:         5 * time.Minute,
		ReceiveCertInterval: 10 * time.Second,
		ProofOfPossession:   2 * time.Minute,
		PenteFXSetup:        20 * time.Minute,
	}
}

// resolved returns a JoinTimeouts with zero values replaced by defaults.
func (t JoinTimeouts) resolved() JoinTimeouts {
	d := DefaultJoinTimeouts()
	if t.WaitSync == 0 {
		t.WaitSync = d.WaitSync
	}
	if t.WaitSyncInterval == 0 {
		t.WaitSyncInterval = d.WaitSyncInterval
	}
	if t.VoteQBFT == 0 {
		t.VoteQBFT = d.VoteQBFT
	}
	if t.VoteQBFTInterval == 0 {
		t.VoteQBFTInterval = d.VoteQBFTInterval
	}
	if t.RequestCert == 0 {
		t.RequestCert = d.RequestCert
	}
	if t.ReceiveCert == 0 {
		t.ReceiveCert = d.ReceiveCert
	}
	if t.ReceiveCertInterval == 0 {
		t.ReceiveCertInterval = d.ReceiveCertInterval
	}
	if t.ProofOfPossession == 0 {
		t.ProofOfPossession = d.ProofOfPossession
	}
	if t.PenteFXSetup == 0 {
		t.PenteFXSetup = d.PenteFXSetup
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
	// InternalApiURL is the spoke coordinator's (central bank) api-gateway base URL. The relay
	// polls {InternalApiURL}/internal/v1/payments/fx/agreements for the CB's aggregate of its
	// banks' on-chain FX agreements. GRPCEndpoint is the CB payment-orchestrator (for the
	// on_behalf mirror of the destination leg).
	InternalApiURL string `json:"internal_api_url"`
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
