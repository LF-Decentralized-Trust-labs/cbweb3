// Package bundle emits/loads/validates the hub bundle (TK-B6) — the public
// hand-off artifact from found-hub to found-spoke. It carries the hub contract
// addresses + consumption config (RPC-only); it MUST NOT contain secrets.
package bundle

// HubBundle is the versioned public artifact describing the founded hub.
type HubBundle struct {
	Version   string            `yaml:"version" json:"version"`
	ChainID   uint64            `yaml:"chainId" json:"chainId"`
	HubRPC    string            `yaml:"hubRpc" json:"hubRpc"`
	HubWS     string            `yaml:"hubWs" json:"hubWs"`
	Contracts map[string]string `yaml:"contracts" json:"contracts"` // name -> 0x-address
}
