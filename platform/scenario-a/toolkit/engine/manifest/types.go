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

// BankCode returns the resolved commercial-bank identifier: spec.bankId when set,
// otherwise metadata.name (which is always required). Used for mode:join.
func (m *Manifest) BankCode() string {
	if m.Spec.BankID != "" {
		return m.Spec.BankID
	}
	return m.Metadata.Name
}

// DisplayNameOr returns the friendly institution label shown in the portal header:
// spec.displayName when set, otherwise the provided fallback (the entity name for a
// central bank, or the resolved bank code for a commercial bank). Cosmetic only —
// it feeds VITE_INSTITUTION_NAME and nothing else (never the CSR/registry identity).
func (m *Manifest) DisplayNameOr(fallback string) string {
	if m.Spec.DisplayName != "" {
		return m.Spec.DisplayName
	}
	return fallback
}

// Spec is the desired-state specification for the participant.
// Secrets (private keys, passphrases, credentials) are never expressible here;
// all key material is referenced via KeyProvider URI only.
type Spec struct {
	Scenario    string `yaml:"scenario"`
	Environment string `yaml:"environment,omitempty"`
	Role        string `yaml:"role"`
	Mode        string `yaml:"mode"`
	// BankID identifies a commercial bank within a spoke (mode:join). Optional:
	// when empty, the bank code is derived from metadata.name. It feeds the CSR
	// subject CN, the IdentityRegistry name, and the BANK_ID compose variable.
	BankID string `yaml:"bankId,omitempty"`
	// DisplayName is the friendly institution label shown in the portal header
	// (VITE_INSTITUTION_NAME), e.g. "Itaú" instead of the slug "bank-itau".
	// Optional and purely cosmetic: when empty, the label falls back to the entity
	// name (central bank) or the resolved bank code (commercial bank).
	DisplayName   string `yaml:"displayName,omitempty"`
	Spoke         Spoke  `yaml:"spoke"`
	Node          Node   `yaml:"node"`
	Image         string `yaml:"image"`
	KeyProvider   string `yaml:"keyProvider"`
	CertSource    string `yaml:"certSource"`
	Relay         *Relay `yaml:"relay,omitempty"`
	JoinBundleRef string `yaml:"joinBundleRef,omitempty"`
	// CBEndpoint is the central bank's credential-request URL (mode:found). It is
	// embedded into the emitted join bundle so a joining commercial bank knows
	// where to submit its CSR. Required for a usable found→join chain.
	CBEndpoint string `yaml:"cbEndpoint,omitempty"`
	// AdminUsers are the per-role human operator accounts provisioned in the
	// entity's Keycloak realm(s). Portal/operator login uses these (ROPC password
	// grant) instead of the confidential client credentials, so audit logs carry a
	// real actor. Required (one per role the entity hosts).
	AdminUsers []AdminUser `yaml:"adminUsers"`
}

// AdminUser is a per-role operator account created in the entity's Keycloak realm.
//
// Role is the Keycloak realm role the user is granted and also selects the realm
// the user lands in (central-bank realm for ROLE_GOVERNANCE/ROLE_TREASURY/
// ROLE_SUPERVISOR, the shared cbweb3/NOC realm for ROLE_NOC_ADMIN, the bank realm
// for ROLE_BANK).
//
// NOTE: Password is read from the manifest for the local profile. A future
// iteration MUST source it from a secret store (KeyProvider/SecretSource) and use
// a temporary password (force reset on first login) for non-local environments;
// it must never be committed for staging/prod.
type AdminUser struct {
	Role     string `yaml:"role"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
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
// DataDir is the absolute path on the host where the provisioning engine stores
// all spoke-specific runtime data: genesis files, TLS certs, Paladin configs,
// provisioning state, and deployed contract addresses.
type Node struct {
	AdvertisedHost string `yaml:"advertisedHost"`
	RPC            *Port  `yaml:"rpc,omitempty"`
	WS             *Port  `yaml:"ws,omitempty"`
	P2P            *Port  `yaml:"p2p,omitempty"`
	DataDir        string `yaml:"dataDir,omitempty"`
}

// Port holds a single TCP port number.
type Port struct {
	Port int `yaml:"port"`
}

// Relay holds the configuration for registering the spoke with the LNET-operated relay.
type Relay struct {
	Endpoint string `yaml:"endpoint"`
}
