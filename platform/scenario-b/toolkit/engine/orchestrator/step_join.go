// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/pki"
)

// JoinConfig parametrizes the join mode (a commercial bank attaching to its
// spoke as a non-validating full node). External gates/reads are injectable so
// the step set is testable without a live Besu/Keycloak.
type JoinConfig struct {
	Runner          exec.CommandRunner
	TemplatesDir    string
	OutDir          string
	BankID          string
	Institution     string
	AdminUsers      []AdminUser // per-role Keycloak operator accounts (from spec.adminUsers)
	SpokeID         string
	SpokeChainID    uint64
	Currency        string // spoke's ISO 4217 code (spec.spoke.currency); cross-checked against the spoke bundle
	BankRPC         string // RPC of the bank's own node (wait-sync gate)
	SpokeBundlePath string
	DataDir         string
	BankEnvFile     string
	KeycloakEnv     []string
	VolumePrefix    string // <p>_genesis, <p>_besu_data (node state in named volumes)
	ContainerPrefix string // container name prefix (<p>-<entity>-besu, ...)
	NetPrefix       string // docker network name prefix
	Entity          string // compose ENTITY label (e.g. the bank id)
	RPCPort         int    // host port -> besu 8545; other service ports derive by offset
	WSPort          int    // host port -> besu 8546
	P2PPort         int    // host port -> besu 30303
	HubRPC          string // hub RPC (routable, from the spoke bundle) → HUB_BESU_RPC_URL
	RelayEndpoint   string // the relay's OWN REST endpoint (spec.relay.endpoint, e.g. http://<hub>:7000) → CACTI_API_URL
	// RelayContainerName is the relay container on THIS host (spec.relay.containerName),
	// used only so this bank's noc-agent can collect the relay's logs. Empty → no relay
	// logs in the NOC.
	RelayContainerName string
	NOCBackendURL   string // where this bank's noc-agent pushes (spec.noc.backendURL; default host.docker.internal:8090)
	BesuImage       string
	GatewayURL      string
	FrontendHost    string // browser-facing host baked into VITE_API_URL + api-gateway CORS (spec.frontendHost; default localhost)
	ProxyEnabled    bool   // spec.proxy == enable: serve the bank portal + api behind the per-host reverse proxy
	LauncherEnabled bool   // spec.launcher == "enable": bake VITE_LAUNCHER_URL into the bank portal
	LauncherPort    int    // spec.launcherPort: launcher host port (0 → env/default); host = FrontendHost

	// Injectable seams (defaults wired by WithDefaults).
	WaitRPC          func(ctx context.Context) error
	WaitSync         func(ctx context.Context) error
	WaitKeycloak     func(ctx context.Context) error
	ReadClientSecret func(ctx context.Context) (string, error)
	EthSyncing       EthSyncing
}

// joinWaitSyncTimeout is how long the join waits for the bank's Besu to peer with the
// CB bootnode and import its first block. Peering is via UDP discovery (--bootnodes),
// and on a busy single host the SECOND+ bank can take a few minutes to bond its first
// peer, so the default is generous (10m). Override with CBWEB3_WAIT_SYNC_TIMEOUT (a Go
// duration, e.g. "20m") for very loaded hosts.
func joinWaitSyncTimeout() time.Duration {
	if v := os.Getenv("CBWEB3_WAIT_SYNC_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 10 * time.Minute
}

func (c *JoinConfig) WithDefaults() {
	if c.BesuImage == "" {
		c.BesuImage = "hyperledger/besu:25.8.0"
	}
	if c.VolumePrefix == "" {
		c.VolumePrefix = c.BankID
	}
	if c.ContainerPrefix == "" {
		c.ContainerPrefix = "sc-b-cbweb3-" + c.BankID
	}
	if c.NetPrefix == "" {
		c.NetPrefix = c.VolumePrefix
	}
	if c.Entity == "" {
		c.Entity = c.BankID
	}
	if c.NOCBackendURL == "" {
		c.NOCBackendURL = "http://host.docker.internal:8090"
	}
	if c.RPCPort == 0 {
		c.RPCPort = 10545
	}
	if c.WSPort == 0 {
		c.WSPort = c.RPCPort + 1
	}
	if c.P2PPort == 0 {
		c.P2PPort = 30303
	}
	if c.GatewayURL == "" {
		c.GatewayURL = fmt.Sprintf("http://localhost:%d", c.RPCPort+8000)
	}
	if c.EthSyncing == nil {
		c.EthSyncing = ethSyncing
	}
	if c.WaitRPC == nil {
		c.WaitRPC = func(ctx context.Context) error { return waitRPC(ctx, c.BankRPC, 60*time.Second) }
	}
	if c.WaitSync == nil {
		c.WaitSync = func(ctx context.Context) error {
			return waitSync(ctx, c.BankRPC, joinWaitSyncTimeout(), 2*time.Second, c.EthSyncing)
		}
	}
	if c.WaitKeycloak == nil {
		c.WaitKeycloak = func(ctx context.Context) error {
			return waitHTTPOK(ctx, fmt.Sprintf("http://localhost:%d/realms/master", c.keycloakPort()), 180*time.Second)
		}
	}
	if c.ReadClientSecret == nil {
		c.ReadClientSecret = func(context.Context) (string, error) { return bankKeycloakSecret, nil }
	}
}

func (c JoinConfig) genesisVolume() string  { return c.VolumePrefix + "_genesis" }
func (c JoinConfig) besuDataVolume() string { return c.VolumePrefix + "_besu_data" }
func (c JoinConfig) caVolume() string       { return c.VolumePrefix + "_cb_tls" }
func (c JoinConfig) svcTLSVolume() string   { return c.VolumePrefix + "_svc_tls" }
func (c JoinConfig) keycloakPort() int      { return c.RPCPort + 7000 }

// keycloakContainer matches entity-keycloak.compose.yaml's container_name.
func (c JoinConfig) keycloakContainer() string {
	return c.ContainerPrefix + "-" + c.Entity + "-keycloak"
}

// Local Keycloak realm/client provisioned by join (OIDC for the bank backend).
const (
	bankKeycloakRealm  = "cbweb3"
	bankKeycloakClient = "bank-backend"
	bankKeycloakSecret = "bank-backend-local-secret" // local-only, not a real secret
	// Commercial-bank login user seeded into the realm. Holds commercial_bank,
	// the role the v2 AMM swap routes require. Local-dev credentials only.
	bankUser = "bank-admin"
	bankPass = "bank-admin-local"
)

// bankRoles are the realm roles granted to the bank login user: commercial_bank
// (v2 AMM swap/quote) plus ROLE_COMMERCIAL_BANK for the v1 onboarding path.
var bankRoles = []string{"commercial_bank", "ROLE_COMMERCIAL_BANK"}

// provisionKeycloakRealm creates the realm + confidential client via kcadm
// (idempotent: create failures are ignored).
func (c JoinConfig) provisionKeycloakRealm(ctx context.Context) error {
	kc := "/opt/keycloak/bin/kcadm.sh"
	var b strings.Builder
	fmt.Fprintf(&b, "%[1]s config credentials --server http://localhost:8080 --realm master --user admin --password admin && ", kc)
	fmt.Fprintf(&b, "(%[1]s create realms -s realm=%[2]s -s enabled=true || true) && ", kc, bankKeycloakRealm)
	fmt.Fprintf(&b, "(%[1]s create clients -r %[2]s -s clientId=%[3]s -s secret=%[4]s -s enabled=true "+
		"-s publicClient=false -s serviceAccountsEnabled=true -s directAccessGrantsEnabled=true %[5]s || true) && ",
		kc, bankKeycloakRealm, bankKeycloakClient, bankKeycloakSecret, audienceMapperArg(keycloakBackendAudience))
	// The bank auth's GetAdminToken (client_credentials) resolves users on login, so
	// its service account needs the realm-management view/manage user roles.
	fmt.Fprintf(&b, "(%[1]s add-roles -r %[2]s --uusername service-account-%[3]s "+
		"--cclientid realm-management --rolename manage-users --rolename view-users || true) && ",
		kc, bankKeycloakRealm, bankKeycloakClient)
	// Per-role operator accounts from the manifest (spec.adminUsers); fall back to a
	// single default bank admin when none are declared.
	users := c.AdminUsers
	if len(users) == 0 {
		users = []AdminUser{{Role: "BANK", Username: bankUser, Password: bankPass}}
	}
	appendKeycloakUsers(&b, kc, bankKeycloakRealm, users)
	_, err := c.Runner.Run(ctx, "docker", "exec", c.keycloakContainer(), "bash", "-c", b.String())
	return err
}

// useProxy reports whether the bank serves its portal + api behind the per-host proxy.
func (c JoinConfig) useProxy() bool { return c.ProxyEnabled && c.FrontendHost != "" }

// frontendVariant tags the bank frontend image so a proxy (base-path-aware) build is
// never confused with a non-proxy one under the same gateway-port tag.
func (c JoinConfig) frontendVariant() string {
	if c.useProxy() {
		return proxyImageVariant(c.FrontendHost)
	}
	return ""
}

// portalViteArgs are the build-time VITE_* args of the bank portal. gwPort is the
// bank's api-gateway host port; behind the proxy the portal is same-origin and
// base-path-aware instead.
//
// VITE_FIAT_SYMBOL is the portal's fallback currency label: the displayed code
// normally comes from the on-chain token symbol returned with a balance, and
// without this fallback the SPA shows the generic "fiat units" until (or unless)
// that response arrives.
func (c JoinConfig) portalViteArgs(gwPort int) map[string]string {
	lu := frontendLauncherURL(c.LauncherEnabled, c.useProxy(), c.FrontendHost, c.LauncherPort)
	args := map[string]string{
		"VITE_API_URL":          fmt.Sprintf("http://%s:%d", frontendHostOrLocal(c.FrontendHost), gwPort),
		"VITE_SCENARIO":         "scenario-b",
		"VITE_INSTITUTION_NAME": c.Entity,
		"VITE_LAUNCHER_URL":     lu,
		"VITE_FIAT_SYMBOL":      c.Currency,
	}
	if c.useProxy() {
		args["VITE_API_URL"] = proxyAPIURL(c.FrontendHost)
		args["VITE_BASE_PATH"] = proxyPortalBase("bank")
	}
	return args
}

// corsOrigins is the bank api-gateway's allowed browser origin(s): the single proxy
// origin behind the proxy, else the bank portal's host-port origin.
func (c JoinConfig) corsOrigins() string {
	if c.useProxy() {
		return proxyOrigin(c.FrontendHost)
	}
	return corsOriginSingle(c.RPCPort, c.FrontendHost)
}

// NetName is this bank's external docker network (created by the infra step).
func (c JoinConfig) NetName() string { return c.NetPrefix + "_net" }

// bankFrontendContainer is the bank portal container name on the entity network
// (must match entity-frontend.compose.yaml: <CONTAINER_PREFIX>-<ENTITY>-frontend).
func (c JoinConfig) bankFrontendContainer() string {
	return fmt.Sprintf("%s-%s-frontend", c.ContainerPrefix, c.Entity)
}

// apiGatewayContainer is the api-gateway container name on the entity network.
func (c JoinConfig) apiGatewayContainer() string {
	return fmt.Sprintf("%s-%s-api-gateway", c.ContainerPrefix, c.Entity)
}

// ProxyRoutes are the path routes the reverse proxy exposes for this bank: its portal +
// the api-gateway.
func (c JoinConfig) ProxyRoutes() []ProxyRoute {
	return []ProxyRoute{
		{Segment: "bank", Upstream: c.bankFrontendContainer() + ":80"},
		{Segment: "api", Upstream: c.apiGatewayContainer() + ":8080", IsAPI: true},
	}
}

// ComposeEnv returns the bank's compose-template interpolation vars as process
// environment (KEY=VALUE), handed to the runner so `docker compose` resolves
// every ${...} WITHOUT persisting plumbing to disk (scenario-a parity: plumbing
// via cmd.Env, state via a slim env file). It carries image names,
// container/network/volume prefixes, host port mappings, the CB's advertised
// enode (--bootnodes, from the spoke bundle), and static local infra creds —
// none discovered at runtime. Runtime-discovered values (contract addresses,
// the Keycloak client secret) are NOT here: those land in BankEnvFile mid-run
// and merge via `--env-file`. Ports derive from RPCPort by fixed offsets.
func (c JoinConfig) ComposeEnv() []string {
	b, err := bundle.LoadSpoke(c.SpokeBundlePath)
	if err != nil {
		// The spoke bundle is validated by consume-spoke-bundle before any compose
		// runs; a read failure here leaves BOOTNODE_ENODE empty (besu then fails
		// fast with a clear error) rather than aborting env assembly.
		b = bundle.SpokeBundle{}
	}
	e := c.ContainerPrefix
	// Per-bank onboarding key (deterministic; seeds the auth KMS so the bank onboards
	// as a distinct on-chain participant — mirrors scenario-a). The derived address is
	// the bank's on-chain identity, injected by the payment proxy as
	// requester_besu_address (ENTITY_BESU_ADDRESS).
	bankKey, bankAddr := deriveBankKey(c.Entity)
	// Prefer the hub RPC port published in the spoke bundle (the CB knows the hub's
	// real port); fall back to the join HubRPC / default. Without this the bank's
	// per-pair resolver dials the wrong hub port and swaps fail.
	hubPort := b.HubRPCPort
	if hubPort == "" {
		hubPort = "8545"
		if u, err := url.Parse(c.HubRPC); err == nil && u.Port() != "" {
			hubPort = u.Port()
		}
	}
	vars := map[string]string{
		// besu (join template)
		"BESU_IMAGE":           c.BesuImage,
		"CONTAINER_PREFIX":     c.ContainerPrefix,
		"ENTITY":               c.Entity,
		"ENTITY_NET_PREFIX":    c.NetPrefix,
		"ENTITY_VOLUME_PREFIX": c.VolumePrefix,
		"ENTITY_RPC_PORT":      itoa(c.RPCPort),
		"ENTITY_WS_PORT":       itoa(c.WSPort),
		"ENTITY_P2P_PORT":      itoa(c.P2PPort),
		"BOOTNODE_ENODE":       b.Enode,
		// hub RPC: HUB_BESU_RPC_URL is the routable hub RPC (from the spoke bundle),
		// mapped container-reachable — works cross-VM. HUB_RPC_PORT stays for the
		// single-host host.docker.internal fallback in the compose templates.
		"HUB_RPC_PORT":     hubPort,
		"HUB_BESU_RPC_URL": containerReachable(c.HubRPC),
		// infra: postgres + redis (single DB doubles as the keycloak DB locally)
		"POSTGRES_USER":     "cbweb3",
		"POSTGRES_PASSWORD": "cbweb3",
		"POSTGRES_DB":       "keycloak",
		"POSTGRES_PORT":     itoa(c.RPCPort + 5000),
		"REDIS_PORT":        itoa(c.RPCPort + 6000),
		// keycloak
		"KC_ADMIN_USER":     "admin",
		"KC_ADMIN_PASSWORD": "admin",
		"KC_DB_URL":         "jdbc:postgresql://" + e + "-" + c.Entity + "-postgres:5432/keycloak",
		"KEYCLOAK_PORT":     itoa(c.keycloakPort()),
		// backend / frontend (images shared with the hub; must be pre-built)
		"GATEWAY_PORT":               itoa(c.RPCPort + 8000),
		"GATEWAY_URL":                c.GatewayURL,
		"BACKEND_IMAGE":              hubBackendImage,
		"COMPLIANCE_IMAGE":           hubComplianceImage,
		"AUTH_IMAGE":                 hubAuthImage,
		"PAYMENT_ORCHESTRATOR_IMAGE": hubPaymentOrchestratorImage,
		"FRONTEND_IMAGE":             cbFrontendImage("bank", c.RPCPort+8000, c.frontendVariant()),
		"FRONTEND_PORT":              itoa(c.RPCPort + 9000),
		// Browser CORS: the single proxy origin (path routing) or this bank's portal
		// origin (host-port; routable host when set) when not behind the proxy.
		"CORS_ALLOW_ORIGINS": c.corsOrigins(),
		// app stack (compliance + auth): the bank is the local signer; the Keycloak
		// realm/client are provisioned by provision-keycloak-bank.
		"SPOKE_CHAIN_ID": fmt.Sprintf("%d", c.SpokeChainID),
		"CB_PRIVATE_KEY": devDeployerKey,
		// Hub signing key: deliberately EMPTY on a commercial bank. The Hub AMM admits only
		// verified Hub participants and a bank is not one, so every Hub act (bridge-in mint,
		// AMM swap, bridge-out burn, residue return) is delegated to its CB over the internal
		// relay channel. Handing the bank the CB's key so it could sign on the Hub itself puts
		// the sovereign key — which is also the Hub governance admin and CENTRAL_BANK_ROLE
		// holder — inside a member bank's container. Quotes and reserve reads need no key.
		"HUB_SIGNER_PRIVATE_KEY": "",
		"KEYCLOAK_REALM":         bankKeycloakRealm,
		"KEYCLOAK_CLIENT_ID":     bankKeycloakClient,
		// CA (scenario-a commercial-bank strategy): the bank has NO CA (only the CB
		// CA signs). Compliance mounts the bank's own host pki dir (its gen-csr
		// key/csr) and runs in dev mode (CA_CERT_FILE empty). CA_VOLUME is still set
		// so the template's cb_tls volume decl interpolates, but it stays
		// uninstantiated (ENTITY_PKI_DIR is a host bind, not the named volume).
		"CA_VOLUME":      c.caVolume(),
		"ENTITY_PKI_DIR": c.pkiDir(),
		// R2-H-8 service-mesh mTLS: the bank's own service CA + leaf certs (separate
		// from the consortium CA, which the bank does not hold) live in this volume,
		// mounted at /svc-tls. mTLS activates only when GRPC_MTLS_ENABLE is exported.
		"SVC_TLS_VOLUME": c.svcTLSVolume(),
		// Commercial banks resolve the sovereign AMM from their CB gateway.
		"CENTRAL_BANK_API_URL": b.CBGateway,
		// The bank's own on-chain address — the payment proxy stamps it as
		// requester_besu_address so the CB registers the deposit against this bank.
		"ENTITY_BESU_ADDRESS": bankAddr,
		// The bank's operator key for its payment-orchestrator's read-only token
		// clients (fCeBM/tCeBM balance). The bank never mints — this only signs the
		// From field of balance calls; it is the bank's derived on-chain key.
		"ENTITY_BESU_OPERATOR_KEY": bankKey,
		// Hub contract addresses (published by the CB in the spoke bundle) so the
		// bank runs the same on-chain per-pair AMM resolver as its CB — dynamic swap
		// on any corridor. TD-001: there is no default AMM; the PairRegistry enables
		// the v2 routes and the resolver picks the AMM + tokens per pool_pair, so no
		// fixed HUB_TOKEN_A/B is wired here (the bank does not mint — only quote + swap).
		"PAIR_REGISTRY_CONTRACT_ADDRESS":     b.HubContracts["pairRegistry"],
		"CURRENCY_REGISTRY_CONTRACT_ADDRESS": b.HubContracts["currencyRegistry"],
		"HUB_IDENTITY_REGISTRY_ADDRESS":      b.HubContracts["identityRegistry"],
		"LIQUIDITY_COMMIT_REGISTRY_ADDRESS":  b.HubContracts["liquidityCommitRegistry"],
		// Shared secret so the bank delegates the cross-currency bridge-in lock-mint
		// to its CB (only CBs hold CENTRAL_BANK_ROLE to mint W-tokens) and bridge-out.
		"INTERNAL_RELAY_AUTH_SECRET": hubRelayAuthSecret,
		// Cacti relay endpoint: the bank's cross-currency swap orchestrator delegates
		// the Step 3 bridge-out to the beneficiary CB (CB-B) through it. Fixed relay
		// Cacti relay REST endpoint: the relay's OWN endpoint (spec.relay.endpoint,
		// commonly on the hub), mapped container-reachable — NOT the local host.
		// Falls back to the single-host default when no endpoint is configured.
		"CACTI_API_URL": relayCactiURL(c.RelayEndpoint),
		// The bank's own spoke id (for bridge lock-mint derivation).
		"SPOKE_NETWORK": b.SpokeID,
		// Governance-portal onboarding (mirrors scenario-a): the api-gateway smart
		// proxy reads the bank's CSR from PKI_DIR/<bankCode>.csr, and the bank's auth
		// KMS is seeded with a per-bank key so onboarding registers a DISTINCT wallet.
		"PKI_DIR": "/workspace/backend/config/pki",
		// Enforcement is a RECEIVER-side setting, and a commercial bank hosts no internal relay
		// routes: /internal/amm/*, /internal/v1/payments/* and /internal/v2/transfer-limits/* are all
		// registered on central-bank gateways only. A bank is purely a sender.
		//
		// Forced empty rather than inherited, because inheriting it would make a bank refuse to start:
		// its PKI dir holds only its own key and its -participant certificate (which the pin loader
		// skips by design), so its registry is empty — and empty plus enforcement is exactly the
		// combination the gateway refuses. An operator exporting the flag for the CBs must not take
		// the banks down with it.
		"RELAY_REQUIRE_SIGNATURE": "",
		"KMS_SEED_KEY_ID":         c.Entity,
		"KMS_SEED_PRIVATE_KEY":    bankKey,
		// noc (observability — soft). The bank runs its own agent (node-level
		// monitoring), mounting a rendered agent.yaml from NOC_AGENT_VOLUME and
		// joining its own ENTITY_NET_PREFIX network to probe besu by container DNS.
		"NOC_AGENT_ENTITY": c.Entity,
		"NOC_AGENT_VOLUME": c.nocAgentVolume(),
		"NOC_AGENT_IMAGE":  hubNocAgentImage,
	}
	env := make([]string, 0, len(vars))
	for k, v := range vars {
		env = append(env, k+"="+v)
	}
	sort.Strings(env) // deterministic order (stable across runs / for tests)
	return env
}

func (c JoinConfig) bankTemplate(name string) string {
	return filepath.Join(c.TemplatesDir, name+".compose.yaml")
}

// composeUpArgs builds `compose -p <prefix> -f <tmpl> [--env-file <file>] up -d`.
// The --env-file is added only once BankEnvFile exists (it holds runtime-
// discovered addresses + the Keycloak client secret); all plumbing comes from
// the runner's process env (ComposeEnv), so the join besu can start before the
// file is ever written.
func (c JoinConfig) composeUpArgs(tmpl string) []string {
	args := []string{"compose", "-p", c.ContainerPrefix, "-f", c.bankTemplate(tmpl)}
	if fileExists(c.BankEnvFile) {
		args = append(args, "--env-file", c.BankEnvFile)
	}
	return append(args, "up", "-d")
}

// scenarioBDir is <repo>/scenario-b, the docker build context root for the bank's
// soft service images. Derived from TemplatesDir (<scenario-b>/provisioning/templates).
func (c JoinConfig) scenarioBDir() string { return filepath.Dir(filepath.Dir(c.TemplatesDir)) }

func (c JoinConfig) nocAgentVolume() string { return c.VolumePrefix + "_noc_agent_cfg" }

// JoinSteps builds the ordered join step set (canonical flow, roadmap §6): no
// relay step (the spoke chain is registered at found-spoke), plus a Soft
// add-noc-agent so the bank's OWN node is monitored (node-level, not just the
// shared chain via the CB's agent).
func JoinSteps(c JoinConfig) []Step {
	c.WithDefaults()

	compose := func(tmpl string) func(context.Context) error {
		return func(ctx context.Context) error {
			_, err := c.Runner.Run(ctx, "docker", c.composeUpArgs(tmpl)...)
			return err
		}
	}

	return []Step{
		{
			Name: "consume-spoke-bundle",
			Run: func(context.Context) error {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return err
				}
				// The currency is the routing key (relay id "spoke-<currency>", hub
				// currency registration, mirrored "W-tCeBM_<ISO>"), so a bank claiming a
				// currency other than the one its spoke was founded with would mislabel
				// balances and route to a corridor that does not exist. Bundles emitted
				// before the field existed carry no currency → nothing to check.
				if b.Currency != "" && c.Currency != "" && !strings.EqualFold(b.Currency, c.Currency) {
					return fmt.Errorf("consume-spoke-bundle: spec.spoke.currency %q does not match spoke %s currency %q (from %s)",
						c.Currency, b.SpokeID, b.Currency, c.SpokeBundlePath)
				}
				return nil
			},
		},
		{
			Name: "write-genesis",
			Deps: []string{"consume-spoke-bundle"},
			// Node state lives in a named volume (roadmap §7), so the bundle genesis
			// is seeded into <prefix>_genesis, not the host. Non-destructive guard:
			// skip when the volume genesis already matches the bundle; a divergent
			// genesis is a hard error (the bank must run exactly the spoke genesis —
			// same chainId, CB-only QBFT validator set).
			Check: func(ctx context.Context) (bool, error) {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return false, err
				}
				existing, err := readVolumeFile(ctx, c.Runner, c.genesisVolume(), "genesis.json")
				if err != nil || len(existing) == 0 {
					return false, nil // absent/unreadable → seed
				}
				if sha256.Sum256(existing) == sha256.Sum256([]byte(b.Genesis)) {
					return true, nil
				}
				return false, fmt.Errorf("write-genesis: genesis in volume %s diverges from the spoke bundle; "+
					"the bank must run the spoke genesis", c.genesisVolume())
			},
			Run: func(ctx context.Context) error {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return err
				}
				// Seed straight into the named volume from memory — no host genesis
				// file (volumefs invariant: node state lives only in the volume; the
				// only host bind mount is the bank pki/ dir).
				return writeVolumeFile(ctx, c.Runner, c.genesisVolume(), "genesis.json", []byte(b.Genesis), "0644")
			},
		},
		{
			// Static peer: seed static-nodes.json (the CB's enode, from the spoke
			// bundle) into the Besu DATA volume so the bank holds a DIRECT, persistent
			// connection to the CB — Besu reads <data-path>/static-nodes.json on start.
			// The join otherwise relies only on UDP discovery (--bootnodes), and on a
			// busy single host that can take minutes to bond the 2nd+ bank's first peer
			// (every node advertises 127.0.0.1 in its discovery record, so the churn
			// wastes bonding cycles) — long enough to race the wait-sync gate. A static
			// node connects immediately, so the node syncs deterministically. Must be
			// written BEFORE start-besu-join so it is present at first boot (a fresh/
			// --clean run); an already-running besu picks it up on its next recreate.
			Name: "write-static-nodes",
			Deps: []string{"write-genesis"},
			Check: func(ctx context.Context) (bool, error) {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return false, err
				}
				existing, err := readVolumeFile(ctx, c.Runner, c.besuDataVolume(), "static-nodes.json")
				if err != nil || len(existing) == 0 {
					return false, nil
				}
				return strings.Contains(string(existing), b.Enode), nil
			},
			Run: func(ctx context.Context) error {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return err
				}
				content := fmt.Sprintf("[%q]\n", b.Enode)
				return writeVolumeFile(ctx, c.Runner, c.besuDataVolume(), "static-nodes.json", []byte(content), "0644")
			},
		},
		{
			// Non-validating full node: the node is not in the spoke's QBFT
			// validator set (genesis lists only the CB), so it syncs without
			// producing blocks — no besu "non-validator" flag is needed. Uses the
			// join template (--bootnodes = the CB's advertised enode) plus the
			// static-nodes.json seeded above for a deterministic direct connection.
			// Plumbing (${...}) comes from the runner's process env (ComposeEnv).
			Name: "start-besu-join",
			Deps: []string{"write-static-nodes"},
			Run: func(ctx context.Context) error {
				if _, err := c.Runner.Run(ctx, "docker", c.composeUpArgs("entity-besu-join")...); err != nil {
					return err
				}
				return c.WaitRPC(ctx)
			},
		},
		{
			Name: "wait-sync",
			Deps: []string{"start-besu-join"},
			Run:  func(ctx context.Context) error { return c.WaitSync(ctx) },
		},
		{
			Name: "wire-addresses",
			Deps: []string{"consume-spoke-bundle"},
			Run: func(context.Context) error {
				b, err := bundle.LoadSpoke(c.SpokeBundlePath)
				if err != nil {
					return err
				}
				for key, name := range map[string]string{
					"SPOKE_IDENTITY_REGISTRY_ADDRESS": "identityRegistry",
					"SPOKE_TCEBM_ADDRESS":             "tCeBM",
					"SPOKE_BRIDGE_ADDRESS":            "spokeBridge",
					"SPOKE_FCEBM_ADDRESS":             "fCeBM",
				} {
					if err := addrs.AppendAddr(c.BankEnvFile, key, b.Contracts[name]); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Name: "provision-keycloak-bank",
			// Infra creates the external infra network + postgres that keycloak joins.
			Deps: []string{"start-bank-infra"},
			Check: func(context.Context) (bool, error) {
				if len(c.KeycloakEnv) == 0 {
					return false, nil
				}
				for _, env := range c.KeycloakEnv {
					if !addrs.HasAddrKey(env, "KEYCLOAK_CLIENT_SECRET") {
						return false, nil
					}
				}
				return true, nil
			},
			Run: func(ctx context.Context) error {
				if _, err := c.Runner.Run(ctx, "docker", c.composeUpArgs("entity-keycloak")...); err != nil {
					return err
				}
				if err := c.WaitKeycloak(ctx); err != nil {
					return err
				}
				if err := c.provisionKeycloakRealm(ctx); err != nil {
					return err
				}
				secret, err := c.ReadClientSecret(ctx)
				if err != nil {
					return err
				}
				for _, env := range c.KeycloakEnv {
					if err := addrs.AppendAddr(env, "KEYCLOAK_CLIENT_SECRET", secret); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{Name: "start-bank-infra", Deps: []string{"wait-sync"}, Run: compose("entity-infra")},
		{
			// R2-H-8 service-mesh mTLS: the bank generates its own service CA + leaf
			// certs (it holds no consortium CA). Harmless until GRPC_MTLS_ENABLE is set.
			Name: "gen-svc-tls-bank",
			Check: func(ctx context.Context) (bool, error) {
				return volumeHasFile(ctx, c.Runner, c.svcTLSVolume(), "svc-ca.crt"), nil
			},
			Run: func(ctx context.Context) error { return genServiceTLS(ctx, c.Runner, c.svcTLSVolume()) },
		},
		// The bank's payment-orchestrator backs the api-gateway's payment gRPC so the
		// deposit/redeem/escrow proxy routes register. No Hub relayer (bank mints
		// nothing — the CB does); image is shared with the hub, built on demand.
		{
			Name: "start-bank-payment",
			// wire-addresses populates SPOKE_FCEBM_ADDRESS/SPOKE_TCEBM_ADDRESS in the
			// bank env-file, which the orchestrator's read-path token clients need.
			Deps: []string{"start-bank-infra", "wire-addresses", "gen-svc-tls-bank"},
			Run: func(ctx context.Context) error {
				if err := buildImageIn(ctx, c.Runner, c.scenarioBDir(), hubPaymentOrchestratorImage,
					"backend/services/payment-orchestrator/Dockerfile", "backend"); err != nil {
					return err
				}
				return compose("entity-payment")(ctx)
			},
		},
		// Deps gen-csr so the host pki dir exists (host-owned) BEFORE compliance
		// bind-mounts it; otherwise Docker auto-creates it root-owned and gen-csr
		// later fails to write (the known join gen-csr permission bug).
		{Name: "start-bank-backend", Deps: []string{"start-bank-infra", "start-bank-payment", "wire-addresses", "provision-keycloak-bank", "gen-csr"}, Run: func(ctx context.Context) error {
			// Build the backend images before `compose up`. A joining bank runs on its
			// own Docker daemon (multi-VM lab) and never had the hub build these, so
			// compose would try to PULL a local-only tag and fail. Idempotent via
			// imageExists (no-op on a single-host lab). Mirrors start-bank-payment.
			for _, b := range []struct{ image, dockerfile, context string }{
				{hubComplianceImage, "backend/services/compliance/Dockerfile", "backend"},
				{hubAuthImage, "backend/services/auth/Dockerfile", "backend"},
				{hubBackendImage, "backend/services/api-gateway/Dockerfile", "backend"},
			} {
				if err := buildImageIn(ctx, c.Runner, c.scenarioBDir(), b.image, b.dockerfile, b.context); err != nil {
					return err
				}
			}
			return compose("entity-backend")(ctx)
		}},
		{Name: "start-bank-frontend", Deps: []string{"start-bank-backend"}, Soft: true, Run: func(ctx context.Context) error {
			// The bank portal bakes this bank's api-gateway URL at build time. The browser
			// reaches the gateway at frontendHost:<gwPort> (spec.frontendHost — a routable
			// IP/DNS for remote access, else localhost); behind the proxy it is instead
			// same-origin at http://<frontendHost>/<scn>/api/v1/ and the SPA is built
			// base-path-aware (VITE_BASE_PATH) under /<scn>/bank/.
			gwPort := c.RPCPort + 8000
			args := c.portalViteArgs(gwPort)
			if err := buildFrontendImage(ctx, c.Runner, c.scenarioBDir(), cbFrontendImage("bank", gwPort, c.frontendVariant()), "bank", args); err != nil {
				return err
			}
			return compose("entity-frontend")(ctx)
		}},
		{
			// Deferred PKI tail: the ONLY PKI step of the toolkit. Generates the
			// bank keypair + CSR locally (key 0600, never transmitted). The
			// toolkit never signs the CSR, never generates CA material, and never
			// registers on-chain — those are runtime (FR-009).
			Name: "gen-csr",
			Check: func(context.Context) (bool, error) {
				keyOK := fileExists(filepath.Join(c.pkiDir(), c.BankID+".key"))
				csrOK := fileExists(filepath.Join(c.pkiDir(), c.BankID+".csr"))
				return keyOK && csrOK, nil
			},
			Run: func(ctx context.Context) error {
				// Refuse to mint a second identity for a bank that is already running from another
				// directory (a run started from a different working directory resolves the relative
				// node.dataDir elsewhere). Generating here would replace the identity the central bank
				// certified, and every internal call would stop verifying.
				if err := ensureIdentityDirUnchanged(ctx, c.Runner, c.ContainerPrefix, c.BankID, c.pkiDir()); err != nil {
					return err
				}
				// FR-010: pre-create the pki dir as the host user before any
				// bind-mount, or Docker creates it root-owned and gen-csr fails.
				if err := os.MkdirAll(c.pkiDir(), 0o700); err != nil {
					return err
				}
				_, _, err := pki.GenerateBankCSR(c.BankID, c.Institution, c.pkiDir())
				return err
			},
		},
		{
			// add-noc-agent runs the BANK's own noc-agent (node-level monitoring of
			// the bank's Besu, distinct from the CB's). The bank self-provisions its
			// per-entity key against the observe NOC backend (idempotent), renders a
			// multi-component agent.yaml, and starts the agent. It pushes under the
			// SAME spoke UUID as the CB (multi-agent per spoke). Soft: a NOC backend
			// that is not up yet (observe not run) never blocks the bank join.
			Name: "add-noc-agent",
			Deps: []string{"wait-sync"},
			Soft: true,
			Run: func(ctx context.Context) error {
				if err := nocProvisionAgentKey(ctx, nocHostURL(c.NOCBackendURL),
					deterministicUUID(c.SpokeID),
					deterministicAgentKey(c.SpokeID, c.BankID),
					c.SpokeID+"-"+c.BankID); err != nil {
					return err
				}
				if err := buildImageIn(ctx, c.Runner, c.scenarioBDir(), hubNocAgentImage,
					"backend/services/noc-agent/Dockerfile", "backend/services/noc-agent"); err != nil {
					return err
				}
				agentCfg, err := renderAgentYAML(c.nocBundle(), c.NOCBackendURL,
					deterministicAgentKey(c.SpokeID, c.BankID), 15)
				if err != nil {
					return err
				}
				if err := writeVolumeFile(ctx, c.Runner, c.nocAgentVolume(), "agent.yaml", agentCfg, "0644"); err != nil {
					return err
				}
				return compose("noc-agent")(ctx)
			},
		},
	}
}

func (c JoinConfig) pkiDir() string { return filepath.Join(c.DataDir, "pki") }

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
