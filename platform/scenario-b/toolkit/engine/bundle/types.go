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
}
