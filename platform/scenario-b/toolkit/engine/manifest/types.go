// SPDX-License-Identifier: Apache-2.0

// Package manifest provides types and validation for Scenario B
// ParticipantDeployment YAML manifests (apiVersion cbweb3b/v1).
//
// A manifest is the single declarative input to the Scenario B provisioning
// toolkit. It describes a participant's desired deployment for one of three
// modes (found-hub, found-spoke, join) without ever containing secret material:
// all key/cert material is referenced via keyProvider/certSource URIs only.
//
// This package is the entry point of the Scenario B toolkit (TK-B1): it parses
// and validates manifests and reports findings. It does not run steps, emit
// bundles, render compose templates, or execute anything — later phases (TK-B2+)
// consume this model.
//
// Per the project constitution (Principle I, scenario-scoped independence) this
// is an independent reimplementation of the Scenario A manifest/validation
// pattern; it does not import scenario-a/toolkit.
package manifest

// ParticipantDeployment is the top-level parsed representation of a
// ParticipantDeployment YAML document.
//
// Pointer fields (Hub, Spoke, Pair, Node, Relay) and empty-string scalar fields
// are used to distinguish "absent" from "zero value", which the per-mode
// required/forbidden matrix relies on.
type ParticipantDeployment struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
}

// Metadata holds the deployment's logical name.
type Metadata struct {
	Name string `yaml:"name" json:"name"`
}

// Spec is the desired-state specification for the participant. Secrets are never
// expressible here; key/cert material is referenced via keyProvider/certSource.
type Spec struct {
	Scenario    string   `yaml:"scenario" json:"scenario"`
	Environment string   `yaml:"environment" json:"environment"`
	Mode        string   `yaml:"mode" json:"mode"`
	Topology    Topology `yaml:"topology" json:"topology"`
	DisplayName string   `yaml:"displayName,omitempty" json:"displayName,omitempty"`

	// Hub is required in found-hub and forbidden in the other modes.
	Hub *Hub `yaml:"hub,omitempty" json:"hub,omitempty"`
	// Spoke is required in found-spoke and join, forbidden in found-hub.
	Spoke *Spoke `yaml:"spoke,omitempty" json:"spoke,omitempty"`
	// HubBundleRef is required in found-spoke, forbidden in the other modes.
	HubBundleRef string `yaml:"hubBundleRef,omitempty" json:"hubBundleRef,omitempty"`
	// JoinBundleRef is required in join, forbidden in the other modes.
	JoinBundleRef string `yaml:"joinBundleRef,omitempty" json:"joinBundleRef,omitempty"`
	// BankID is required in join, forbidden in the other modes.
	BankID string `yaml:"bankId,omitempty" json:"bankId,omitempty"`
	// Pair is optional in found-spoke, forbidden in the other modes.
	Pair *Pair `yaml:"pair,omitempty" json:"pair,omitempty"`

	Node        *Node  `yaml:"node,omitempty" json:"node,omitempty"`
	Image       string `yaml:"image" json:"image"`
	KeyProvider string `yaml:"keyProvider" json:"keyProvider"`
	CertSource  string `yaml:"certSource" json:"certSource"`
	Relay       *Relay `yaml:"relay,omitempty" json:"relay,omitempty"`
	// CBEndpoint is optional in found-hub/found-spoke, forbidden in join.
	CBEndpoint   string      `yaml:"cbEndpoint,omitempty" json:"cbEndpoint,omitempty"`
	FrontendHost string      `yaml:"frontendHost" json:"frontendHost"`
	AdminUsers   []AdminUser `yaml:"adminUsers" json:"adminUsers"`
	// Launcher controls the per-entity "launcher" SPA on this entity's host (the
	// distributed A/B entry point). "enable" runs the generic launcher image (if not
	// already up) and writes this scenario's portal fragment; "disable" removes the
	// fragment and tears the launcher down when no scenario fragment remains. Empty
	// (absent) is treated as "disable". Applies to commercial banks and central banks
	// (found-spoke / join); it is ignored for the network operator's hub.
	Launcher string `yaml:"launcher,omitempty" json:"launcher,omitempty"`
	// LauncherPort is the host port this entity's launcher is published on. One
	// launcher per host, so each entity on a shared host declares a distinct port
	// (both scenarios of the same entity use the same port and share one launcher).
	// Optional: 0/absent falls back to the LAUNCHER_PORT env, then 5190.
	LauncherPort int `yaml:"launcherPort,omitempty" json:"launcherPort,omitempty"`
}

// Topology carries the participant's role discriminator.
type Topology struct {
	Role string `yaml:"role" json:"role"`
}

// Hub is the hub network specification (found-hub).
type Hub struct {
	ChainID  int      `yaml:"chainId" json:"chainId"`
	Currency []string `yaml:"currency" json:"currency"`
}

// Spoke identifies the spoke network this participant belongs to.
type Spoke struct {
	ID       string `yaml:"id" json:"id"`
	ChainID  int    `yaml:"chainId" json:"chainId"`
	Currency string `yaml:"currency" json:"currency"`
}

// Pair describes the sovereign currency pair (optional, found-spoke).
type Pair struct {
	ProposerCB  string `yaml:"proposerCB" json:"proposerCB"`
	ConfirmerCB string `yaml:"confirmerCB" json:"confirmerCB"`
	SymbolA     string `yaml:"symbolA" json:"symbolA"`
	SymbolB     string `yaml:"symbolB" json:"symbolB"`
}

// Node holds the network addressing configuration for the Besu node.
// AdvertisedHost must always be set explicitly; it is never inferred.
type Node struct {
	AdvertisedHost string `yaml:"advertisedHost" json:"advertisedHost"`
	RPC            *Port  `yaml:"rpc,omitempty" json:"rpc,omitempty"`
	WS             *Port  `yaml:"ws,omitempty" json:"ws,omitempty"`
	P2P            *Port  `yaml:"p2p,omitempty" json:"p2p,omitempty"`
	DataDir        string `yaml:"dataDir,omitempty" json:"dataDir,omitempty"`
	// Validator is a pointer so an absent value is distinguishable from false.
	// In join, an explicit true produces a warning (FR-013): the model is a
	// non-validating full node, but promotion is left open (vote-qbft).
	Validator *bool `yaml:"validator,omitempty" json:"validator,omitempty"`
}

// Port holds a single TCP port number.
type Port struct {
	Port int `yaml:"port" json:"port"`
}

// Relay holds the configuration for registering the spoke with the relay.
type Relay struct {
	Endpoint       string `yaml:"endpoint" json:"endpoint"`
	AdvertisedHost string `yaml:"advertisedHost,omitempty" json:"advertisedHost,omitempty"`
}

// AdminUser is a per-role operator account created in the entity's Keycloak
// realm. Password is a local-only bootstrap value; it is never a secret key.
type AdminUser struct {
	Role     string `yaml:"role" json:"role"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}
