package orchestrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
)

// NOC provisioning derives its identifiers deterministically from public ids so
// that every participant agrees on them WITHOUT exchanging any state:
//   - the CB's found-spoke emits a NOC bundle carrying the spoke UUID,
//   - the observe-mode NOC deployment consumes the bundle and registers the
//     spoke under that same UUID + provisions the CB agent's key,
//   - a bank's join derives the SAME spoke UUID (and its own agent key) and
//     pushes to the same backend.
// Nothing here is secret: everything is a pure function of public identifiers.

const (
	// nocUUIDNamespace salts the spoke-UUID derivation so it never collides with
	// any other sha256-derived id in the toolkit.
	nocUUIDNamespace = "cbweb3b/noc/spoke/v1"
	// nocAgentKeyNamespace salts the per-entity agent-key derivation.
	nocAgentKeyNamespace = "cbweb3b/noc/agent-key/v1"
)

// deterministicUUID derives a stable RFC-4122 v5-shaped UUID from a spoke id.
// The NOC backend does a strict uuid.Parse on the registered/pushed spoke id, so
// the output is a canonical 8-4-4-4-12 hex string. Uses stdlib sha256 (the same
// hashing precedent as the genesis guard) — no new dependency.
func deterministicUUID(spokeID string) string {
	sum := sha256.Sum256([]byte(nocUUIDNamespace + ":" + spokeID))
	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = (b[6] & 0x0f) | 0x50 // version 5
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// deterministicAgentKey derives a stable per-entity NOC agent API key. Each
// entity of a spoke (the CB, each bank) gets its OWN key so the backend tracks
// their liveness independently — a bank node outage stays visible even while the
// CB keeps pushing. Local-dev value only; production keys are provisioned out of
// band. Not a secret: a pure function of public ids.
func deterministicAgentKey(spokeID, entity string) string {
	sum := sha256.Sum256([]byte(nocAgentKeyNamespace + ":" + spokeID + ":" + entity))
	return "noc-agent-" + hex.EncodeToString(sum[:16])
}

// nocBundle builds the NOC monitoring topology for this spoke: the CB's own
// Besu node, plus the Cacti relay when one is configured. Endpoints use
// container-network DNS on entity_net (the noc-agent shares that network),
// matching the toolkit's other NOC env wiring. Jurisdiction has no manifest
// field yet, so it falls back to the spoke id (a stable, non-empty label the
// backend requires).
func (c SpokeConfig) nocBundle() bundle.NOCBundle {
	e := c.ContainerPrefix
	comps := []bundle.NOCComponent{{
		Name:          "besu-" + c.Entity,
		Type:          "BESU",
		Endpoint:      fmt.Sprintf("http://%s-%s-besu:8545", e, c.Entity),
		ContainerName: fmt.Sprintf("%s-%s-besu", e, c.Entity),
	}}
	if c.RelayEndpoint != "" {
		comps = append(comps, bundle.NOCComponent{
			Name:     "cacti-relay",
			Type:     "CACTI_RELAY",
			Endpoint: c.cactiAPIURL(),
		})
	}
	return bundle.NOCBundle{
		Version:      bundle.NOCBundleVersion,
		SpokeID:      c.SpokeID,
		SpokeUUID:    deterministicUUID(c.SpokeID),
		Name:         c.SpokeID,
		CurrencyCode: c.Currency,
		Jurisdiction: c.SpokeID,
		Components:   comps,
	}
}

// nocBundle builds the NOC monitoring topology for the hub: the hub's validator
// Besu node. The relay endpoint is not modeled on HubConfig, so it is monitored
// via the spokes that carry spec.relay.endpoint.
func (c HubConfig) nocBundle() bundle.NOCBundle {
	e := c.ContainerPrefix
	return bundle.NOCBundle{
		Version:   bundle.NOCBundleVersion,
		SpokeID:   "hub",
		SpokeUUID: deterministicUUID("hub"),
		Name:      "hub",
		// The hub carries many currencies; the NOC "spoke" record is the network
		// hub itself, labelled HUB.
		CurrencyCode: "HUB",
		Jurisdiction: "hub",
		Components: []bundle.NOCComponent{{
			Name:          "besu-hub",
			Type:          "BESU",
			Endpoint:      fmt.Sprintf("http://%s-hub-validator:8545", e),
			ContainerName: fmt.Sprintf("%s-hub-validator", e),
		}},
	}
}

// nocBundle builds the monitoring topology for a JOINING bank: its own Besu node
// (and the relay when configured), keyed by the SAME spoke UUID as the CB —
// deterministicUUID(SpokeID). Component names are entity-scoped (besu-<bankId>)
// so they never collide with the CB's under the shared spoke (the backend keys
// components by (agent_id, name)). Only SpokeUUID + Components feed the rendered
// agent.yaml; the bank does not register or emit the bundle.
func (c JoinConfig) nocBundle() bundle.NOCBundle {
	e := c.ContainerPrefix
	comps := []bundle.NOCComponent{{
		Name:          "besu-" + c.Entity,
		Type:          "BESU",
		Endpoint:      fmt.Sprintf("http://%s-%s-besu:8545", e, c.Entity),
		ContainerName: fmt.Sprintf("%s-%s-besu", e, c.Entity),
	}}
	if c.RelayEndpoint != "" {
		comps = append(comps, bundle.NOCComponent{
			Name:     "cacti-relay",
			Type:     "CACTI_RELAY",
			Endpoint: relayCactiURL(c.RelayEndpoint),
		})
	}
	return bundle.NOCBundle{
		Version:      bundle.NOCBundleVersion,
		SpokeID:      c.SpokeID,
		SpokeUUID:    deterministicUUID(c.SpokeID),
		Name:         c.BankID,
		Jurisdiction: c.SpokeID,
		Components:   comps,
	}
}

// agentYAMLComponent / agentYAML mirror the noc-agent's agent.yaml schema (its
// config package lives in a separate module, so the shape is duplicated here).
type agentYAMLComponent struct {
	Name          string `yaml:"name"`
	Type          string `yaml:"type"`
	Endpoint      string `yaml:"endpoint"`
	ContainerName string `yaml:"container_name,omitempty"`
}

type agentYAML struct {
	SpokeID             string               `yaml:"spoke_id"`
	NocBackendURL       string               `yaml:"noc_backend_url"`
	APIKey              string               `yaml:"api_key"`
	PushIntervalSeconds int                  `yaml:"push_interval_seconds"`
	Components          []agentYAMLComponent `yaml:"components"`
}

// renderAgentYAML renders a per-entity agent.yaml from a NOC bundle. The pushed
// spoke_id is the bundle's UUID (the id the backend registered), so pushes match
// the provisioned key's binding. Multi-component (every component in the bundle),
// unlike the single-BESU env fallback.
func renderAgentYAML(b bundle.NOCBundle, backendURL, apiKey string, pushInterval int) ([]byte, error) {
	if pushInterval <= 0 {
		pushInterval = 15
	}
	ay := agentYAML{
		SpokeID:             b.SpokeUUID,
		NocBackendURL:       backendURL,
		APIKey:              apiKey,
		PushIntervalSeconds: pushInterval,
	}
	for _, c := range b.Components {
		ay.Components = append(ay.Components, agentYAMLComponent{
			Name: c.Name, Type: c.Type, Endpoint: c.Endpoint, ContainerName: c.ContainerName,
		})
	}
	return yaml.Marshal(ay)
}

// nocHostURL maps a container-reachable NOC backend URL (host.docker.internal)
// to a host-process-reachable one (localhost), so the toolkit — a host process —
// can call the backend admin API that a container reaches via host.docker.internal.
func nocHostURL(u string) string {
	return strings.Replace(u, "host.docker.internal", "localhost", 1)
}

// nocProvisionAgentKey provisions an agent key against a local NOC backend's
// admin API (idempotent server-side). The bearer is accepted by any backend
// running with NOC_SKIP_AUTH=true (the toolkit only provisions local backends).
// A 404 (spoke not yet registered by an observe run) surfaces as an error the
// caller treats as soft.
func nocProvisionAgentKey(ctx context.Context, backendURL, spokeUUID, rawKey, hint string) error {
	payload, err := json.Marshal(map[string]string{
		"raw_key":  rawKey,
		"spoke_id": spokeUUID,
		"hint":     hint,
	})
	if err != nil {
		return err
	}
	url := strings.TrimRight(backendURL, "/") + "/api/v1/admin/agents/provision-key"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+nocLocalAdminBearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("noc provision-key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("noc provision-key returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
}
