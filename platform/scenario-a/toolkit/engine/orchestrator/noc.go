// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
)

// NOC provisioning derives its identifiers deterministically from public ids so
// every participant agrees WITHOUT exchanging state:
//   - the CB's found emits a NOC bundle carrying the spoke UUID,
//   - the observe-mode NOC deployment consumes it and registers the spoke under
//     that same UUID + provisions the CB agent's key,
//   - a joining bank derives the SAME spoke UUID (and its own agent key) and
//     pushes to the same backend.
// Nothing here is secret: everything is a pure function of public identifiers.

const (
	nocUUIDNamespace     = "cbweb3/noc/spoke/v1"
	nocAgentKeyNamespace = "cbweb3/noc/agent-key/v1"
	// NOCFoundingAgentLabel is the fixed entity label for the founding CB's agent
	// key. observe (which provisions it) and found (which uses it) both derive the
	// SAME key from the spoke id + this label, with no exchange.
	NOCFoundingAgentLabel = "cb"
	// NOCBackendPort is the fixed host port the observe-deployed NOC backend
	// publishes. found bakes VITE_NOC_BACKEND_URL for the co-located NOC portal
	// against this port; observe publishes the backend on it. Single source of truth.
	//
	// Scenario A sits in its own 645 family (Besu RPC 8645 + 20000) so it never
	// collides with Scenario B's NOC backend (:8090) when both run on the same
	// founding VM: KC 24645 → NOC backend 28645 → NOC portal 32645.
	NOCBackendPort = 28645
)

// DeterministicUUID derives a stable RFC-4122 v5-shaped UUID from a spoke id.
// The NOC backend does a strict uuid.Parse on the registered/pushed spoke id, so
// the output is a canonical 8-4-4-4-12 hex string. Uses stdlib sha256 — no dep.
func DeterministicUUID(spokeID string) string {
	sum := sha256.Sum256([]byte(nocUUIDNamespace + ":" + spokeID))
	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = (b[6] & 0x0f) | 0x50 // version 5
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// DeterministicAgentKey derives a stable per-entity NOC agent API key. Each
// entity of a spoke (the CB, each bank) gets its OWN key so the backend tracks
// their liveness independently. Local-dev value only; a pure function of public
// ids (not a secret).
func DeterministicAgentKey(spokeID, entity string) string {
	sum := sha256.Sum256([]byte(nocAgentKeyNamespace + ":" + spokeID + ":" + entity))
	return "noc-agent-" + hex.EncodeToString(sum[:16])
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

// RenderAgentYAML renders a per-entity agent.yaml from a NOC bundle. The pushed
// spoke_id is the bundle's UUID (the id the backend registered), so pushes match
// the provisioned key's binding. Multi-component (every component in the bundle).
func RenderAgentYAML(b bundle.NOCBundle, backendURL, apiKey string, pushInterval int) ([]byte, error) {
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
