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
// Pointer fields (Hub, Spoke, Node, Relay) and empty-string scalar fields
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
	// NOCBundleRef is required in observe, forbidden in the other modes. It points
	// at the NOC bundle emitted by a CB's found-spoke (or found-hub) describing the
	// monitoring topology this NOC deployment consumes.
	NOCBundleRef string `yaml:"nocBundleRef,omitempty" json:"nocBundleRef,omitempty"`
	// NOC is the optional NOC observability block. In observe it tunes the NOC
	// deployment; in found-*/join it tells this entity's agent where to push
	// (backendURL). Absent falls back to the local single-host convention.
	NOC *NOC `yaml:"noc,omitempty" json:"noc,omitempty"`

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
	// Proxy controls the per-host reverse proxy (Caddy) that fronts this entity's
	// portals + api-gateway on port 80 with path-based routing (/<scenario>/<role>/,
	// /<scenario>/api/), so nothing is reached by port. "enable" runs the generic proxy
	// image (if not already up), writes this scenario's route fragment and reloads;
	// "disable" removes the fragment and tears the proxy down when no fragment remains.
	// Empty (absent) is treated as "disable". When enabled, the entity's portal SPAs are
	// built base-path-aware and their api/CORS/launcher URLs point at spec.frontendHost.
	Proxy string `yaml:"proxy,omitempty" json:"proxy,omitempty"`
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
//
// Currency is the ISO 4217 code and remains the only routing key (relay id
// "spoke-<currency>", hub currency registration, mirrored "W-tCeBM_<ISO>").
// The four token fields are presentation-only overrides for the spoke's own
// tCeBM/fCeBM ERC-20 metadata; absent, they derive from Currency as
// "Tokenized <ISO>"/"tCeBM_<ISO>" and "Fiat <ISO>"/"fCeBM_<ISO>".
type Spoke struct {
	ID       string `yaml:"id" json:"id"`
	ChainID  int    `yaml:"chainId" json:"chainId"`
	Currency string `yaml:"currency" json:"currency"`
	// TokenName overrides the tCeBM ERC-20 name (default "Tokenized <currency>").
	TokenName string `yaml:"tokenName,omitempty" json:"tokenName,omitempty"`
	// TokenSymbol overrides the tCeBM ERC-20 symbol (default "tCeBM_<currency>").
	// Must keep the "<prefix>_<ISO>" shape: portals and the api-gateway derive the
	// displayed currency code from the segment after the last underscore.
	TokenSymbol string `yaml:"tokenSymbol,omitempty" json:"tokenSymbol,omitempty"`
	// FiatTokenName overrides the fCeBM ERC-20 name (default "Fiat <currency>").
	FiatTokenName string `yaml:"fiatTokenName,omitempty" json:"fiatTokenName,omitempty"`
	// FiatTokenSymbol overrides the fCeBM ERC-20 symbol (default "fCeBM_<currency>");
	// same "<prefix>_<ISO>" constraint as TokenSymbol.
	FiatTokenSymbol string `yaml:"fiatTokenSymbol,omitempty" json:"fiatTokenSymbol,omitempty"`
}

// NOC configures the NOC observability integration. Optional in every mode and
// never carries secrets. In observe mode it tunes the NOC deployment that
// consumes the referenced bundle; in found-*/join it configures this entity's
// noc-agent (where to push, how often, which components to report).
type NOC struct {
	// BackendURL is the noc-backend this entity's agent pushes to. Empty falls
	// back to the local single-host convention (host.docker.internal:<port>).
	BackendURL string `yaml:"backendURL,omitempty" json:"backendURL,omitempty"`
	// PushIntervalSeconds overrides the agent push cadence (default 15).
	PushIntervalSeconds int `yaml:"pushIntervalSeconds,omitempty" json:"pushIntervalSeconds,omitempty"`
	// Components optionally filters the monitored component set by type
	// (BESU, CACTI_RELAY, PALADIN). Empty means "all components in the bundle".
	Components []string `yaml:"components,omitempty" json:"components,omitempty"`
	// KeycloakURL is the routable Keycloak the NOC portal password-grants against
	// (the co-located CB/hub realm, e.g. http://<host>:15845). observe bakes it as
	// the portal's VITE_KEYCLOAK_URL. Empty → portal login is unconfigured.
	KeycloakURL string `yaml:"keycloakURL,omitempty" json:"keycloakURL,omitempty"`
	// LauncherURL is the launcher the NOC portal's "back to launcher" affordance
	// returns to (VITE_LAUNCHER_URL). Empty → the affordance hides.
	LauncherURL string `yaml:"launcherURL,omitempty" json:"launcherURL,omitempty"`
	// PortalOrigins are EXTRA browser origins to register on the public noc-portal
	// Keycloak client, on top of the co-located portal's own origin.
	//
	// The NOC portal password-grants against Keycloak from the browser, so Keycloak
	// must know its origin or the token request is refused by CORS before any
	// credential is checked. found-spoke/found-hub register the origin of the portal
	// they serve themselves (RPC+12000). A standalone NOC — the observe mode, which
	// publishes the portal on its own fixed port (3030) and may run on another host
	// entirely — is served from an origin nothing else can know, and observe never
	// touches Keycloak. Declare it here, on the entity that owns the realm.
	//
	// Each value must be a plain scheme://host[:port] with no path or trailing slash;
	// that is the only shape that is both JSON-safe and shell-safe on the kcadm
	// command line. Wildcards are rejected on purpose (finding R1-10.7).
	PortalOrigins []string `yaml:"portalOrigins,omitempty" json:"portalOrigins,omitempty"`
	// AMMGatewayURL is the api-gateway the NOC backend reads AMM pool status from,
	// as reachable FROM THE NOC CONTAINER (the NOC runs on its own docker network,
	// so a compose service name of another stack does not resolve — use the host
	// and published port, e.g. http://host.docker.internal:41645). Empty leaves the
	// backend default, and Pool Stability stays empty while reporting why.
	AMMGatewayURL string `yaml:"ammGatewayURL,omitempty" json:"ammGatewayURL,omitempty"`
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
	// ContainerName is the relay's docker container on THIS host. It is only used to
	// let this entity's noc-agent collect the relay's container logs (the agent reads
	// the docker socket); the relay itself is never touched. Empty → the NOC shows
	// "No logs available." for the relay component.
	ContainerName string `yaml:"containerName,omitempty" json:"containerName,omitempty"`
}

// AdminUser is a per-role operator account created in the entity's Keycloak
// realm. Password is a local-only bootstrap value; it is never a secret key.
type AdminUser struct {
	Role     string `yaml:"role" json:"role"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}
