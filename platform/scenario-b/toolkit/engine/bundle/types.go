// SPDX-License-Identifier: Apache-2.0

// Package bundle emits/loads/validates the hub bundle (TK-B6) — the public
// hand-off artifact from found-hub to found-spoke. It carries the hub contract
// addresses + consumption config (RPC-only); it MUST NOT contain secrets.
package bundle

// HubBundle is the versioned public artifact describing the founded hub.
type HubBundle struct {
	Version    string            `yaml:"version" json:"version"`
	ChainID    uint64            `yaml:"chainId" json:"chainId"`
	HubRPC     string            `yaml:"hubRpc" json:"hubRpc"`
	HubWS      string            `yaml:"hubWs" json:"hubWs"`
	HubGateway string            `yaml:"hubGateway,omitempty" json:"hubGateway,omitempty"` // hub API gateway URL (spoke self-registration)
	Contracts  map[string]string `yaml:"contracts" json:"contracts"`                       // name -> 0x-address
}

// SpokeBundle is the versioned public artifact describing a founded spoke.
// Unlike the hub bundle (RPC-only), it carries genesis + enode so commercial
// banks can join the spoke (TK-B8). Public network data only — no secrets.
type SpokeBundle struct {
	Version   string            `yaml:"version" json:"version"`
	SpokeID   string            `yaml:"spokeId" json:"spokeId"`
	ChainID   uint64            `yaml:"chainId" json:"chainId"`
	Enode     string            `yaml:"enode" json:"enode"`
	SpokeRPC  string            `yaml:"spokeRpc" json:"spokeRpc"`
	SpokeWS   string            `yaml:"spokeWs" json:"spokeWs"`
	Genesis   string            `yaml:"genesis" json:"genesis"` // genesis.json contents (network config)
	Contracts map[string]string `yaml:"contracts" json:"contracts"`
	// CBGateway is the founding CB's api-gateway URL (routable host from
	// node.advertisedHost), used by a joining bank as CENTRAL_BANK_API_URL to
	// resolve the sovereign AMM + pool status from its CB.
	CBGateway string `yaml:"cbGateway,omitempty" json:"cbGateway,omitempty"`
	// HubContracts carries the hub contract addresses (identityRegistry,
	// pairRegistry, currencyRegistry, amm, tCeBM_*) so a joining bank can run the
	// same on-chain per-pair AMM resolver as its CB (dynamic swap on any corridor).
	// The bank reaches the hub RPC via host.docker.internal:<HubRPCPort> unless
	// the compose extra_hosts map is remapped cross-VM.
	HubContracts map[string]string `yaml:"hubContracts,omitempty" json:"hubContracts,omitempty"`
	HubRPCPort   string            `yaml:"hubRpcPort,omitempty" json:"hubRpcPort,omitempty"`
	// HubRPC is the ROUTABLE hub Besu RPC URL (from the hub bundle, e.g.
	// http://<hub-host>:8845). A joining bank uses it for HUB_BESU_RPC_URL so it
	// reaches the hub cross-VM instead of the single-host host.docker.internal
	// fallback (HubRPCPort). Empty on old bundles → bank falls back to HubRPCPort.
	HubRPC string `yaml:"hubRpc,omitempty" json:"hubRpc,omitempty"`
	// Currency is the spoke's ISO 4217 code (the founding CB's spec.spoke.currency).
	// A joining bank declares the same code in its own manifest; the join cross-checks
	// the two so a bank cannot attach to a BRL spoke while claiming COP. Empty on
	// bundles emitted before this field existed → the check is skipped.
	Currency string `yaml:"currency,omitempty" json:"currency,omitempty"`
}

// NOCComponent describes a single monitorable component (a Besu node, a Cacti
// relay, a Paladin node) that the NOC agent probes and reports on. It carries
// only public addressing — never keys or credentials.
type NOCComponent struct {
	Name          string `yaml:"name" json:"name"`
	Type          string `yaml:"type" json:"type"` // BESU | CACTI_RELAY | PALADIN
	Endpoint      string `yaml:"endpoint" json:"endpoint"`
	ContainerName string `yaml:"containerName,omitempty" json:"containerName,omitempty"` // Docker container for log collection
}

// NOCBundle is the versioned public artifact describing a spoke's (or the hub's)
// monitoring topology. It is emitted by found-hub/found-spoke and consumed by an
// observe-mode NOC deployment to register the spoke and drive its agent config.
// Public data only — no secrets (no keys, no agent API keys).
type NOCBundle struct {
	Version string `yaml:"version" json:"version"`
	// SpokeID is the logical id ("spoke-brl", "hub"); used for the bundle filename.
	SpokeID string `yaml:"spokeId" json:"spokeId"`
	// SpokeUUID is the deterministic UUID the NOC backend registers the spoke
	// under, derived from SpokeID so every participant (CB, bank, NOC) agrees on
	// the same id without exchanging it.
	SpokeUUID    string         `yaml:"spokeUuid" json:"spokeUuid"`
	Name         string         `yaml:"name" json:"name"`
	CurrencyCode string         `yaml:"currencyCode" json:"currencyCode"`
	Jurisdiction string         `yaml:"jurisdiction" json:"jurisdiction"`
	Components   []NOCComponent `yaml:"components" json:"components"`
}
