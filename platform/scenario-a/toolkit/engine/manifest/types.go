// SPDX-License-Identifier: Apache-2.0

// Package manifest provides types and validation for ParticipantDeployment YAML manifests.
// A manifest is the single declarative input to the Scenario A provisioning engine; it
// describes a participant's desired deployment state without containing any private keys
// or credentials. All key material is referenced via KeyProvider URI only.
package manifest

// Manifest is the top-level parsed representation of a ParticipantDeployment YAML file.
type Manifest struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

// Metadata holds the deployment's human-readable name.
type Metadata struct {
	Name string `yaml:"name"`
}

// Spec is the desired-state specification for the participant.
// Secrets (private keys, passphrases, credentials) are never expressible here;
// all key material is referenced via KeyProvider URI only.
type Spec struct {
	Scenario      string `yaml:"scenario"`
	Environment   string `yaml:"environment,omitempty"`
	Role          string `yaml:"role"`
	Mode          string `yaml:"mode"`
	Spoke         Spoke  `yaml:"spoke"`
	Node          Node   `yaml:"node"`
	Image         string `yaml:"image"`
	KeyProvider   string `yaml:"keyProvider"`
	CertSource    string `yaml:"certSource"`
	Relay         *Relay `yaml:"relay,omitempty"`
	JoinBundleRef string `yaml:"joinBundleRef,omitempty"`
}

// Spoke identifies the blockchain network this participant belongs to.
type Spoke struct {
	ID       string `yaml:"id"`
	ChainID  int    `yaml:"chainId"`
	Currency string `yaml:"currency"`
}

// Node holds the network addressing configuration for the Besu node.
// AdvertisedHost must always be set explicitly; it is never inferred from
// co-location or container IP.
type Node struct {
	AdvertisedHost string `yaml:"advertisedHost"`
	RPC            *Port  `yaml:"rpc,omitempty"`
	WS             *Port  `yaml:"ws,omitempty"`
	P2P            *Port  `yaml:"p2p,omitempty"`
}

// Port holds a single TCP port number.
type Port struct {
	Port int `yaml:"port"`
}

// Relay holds the configuration for registering the spoke with the LNET-operated relay.
type Relay struct {
	Endpoint string `yaml:"endpoint"`
}
