package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/relayregistrar"
)

// SpokeConfig parametrizes the found-spoke mode. External gates/reads and the
// relay registrar are injectable so the step set is testable without a live
// Besu/Keycloak/relay.
type SpokeConfig struct {
	Runner              exec.CommandRunner
	ContractsDir        string
	TemplatesDir        string
	OutDir              string
	SpokeID             string
	SpokeChainID        uint64
	SpokeRPC            string
	SpokeWS             string
	CBAddress           string
	GenesisDir          string
	VolumePrefix        string // <p>_genesis, <p>_besu_data (node state in named volumes)
	ContainerPrefix     string // container name prefix (<p>-<entity>-besu, ...)
	NetPrefix           string // docker network name prefix (<p>_besu_network, <p>_infra_network)
	Entity              string // compose ENTITY label (e.g. "central-bank")
	RPCPort             int    // host port -> besu 8545; other service ports derive by offset
	WSPort              int    // host port -> besu 8546
	P2PPort             int    // host port -> besu 30303
	AdvertisedHost      string // externally reachable host for the spoke bundle enode (default host.docker.internal)
	RelayAdvertisedHost string // host the (external) relay uses to reach this spoke's RPC/WS/gateway (default host.docker.internal)
	AdminUsers          []AdminUser // per-role Keycloak operator accounts (from spec.adminUsers)
	Currency            string      // domestic currency (e.g. BRL) → tCeBM/fCeBM token names
	TokenName           string // tCeBM name (default "Tokenized <Currency>")
	TokenSymbol         string // tCeBM symbol (default "t<Currency>")
	FiatTokenName       string // fCeBM name (default "Fiat <Currency>")
	FiatTokenSymbol     string // fCeBM symbol (default "f<Currency>")
	ValidatorCount      int
	BesuImage           string
	HubBundlePath       string
	HubRPC              string // hub RPC (from the bundle unless overridden)
	SpokeEnvFile        string
	KeycloakEnv         []string
	GatewayURL          string
	Registrar           relayregistrar.RelayRegistrar

	// Injectable seams (defaults wired by WithDefaults).
	WaitRPC          func(ctx context.Context) error
	WaitKeycloak     func(ctx context.Context) error
	ReadClientSecret func(ctx context.Context) (string, error)
	EnodeReader      EnodeReader
	CBRegistered     func(ctx context.Context) (bool, error) // idempotency for register-cb
}

func (c *SpokeConfig) WithDefaults() {
	if c.ValidatorCount == 0 {
		c.ValidatorCount = 1
	}
	if c.BesuImage == "" {
		c.BesuImage = "hyperledger/besu:25.8.0"
	}
	if c.GenesisDir == "" {
		c.GenesisDir = filepath.Join(c.OutDir, "genesis-"+c.SpokeID)
	}
	if c.VolumePrefix == "" {
		c.VolumePrefix = c.SpokeID + "_central-bank"
	}
	if c.ContainerPrefix == "" {
		c.ContainerPrefix = "sc-b-cbweb3-" + c.SpokeID
	}
	if c.NetPrefix == "" {
		c.NetPrefix = c.VolumePrefix
	}
	if c.Entity == "" {
		c.Entity = "central-bank"
	}
	if c.RPCPort == 0 {
		c.RPCPort = 9545
	}
	if c.WSPort == 0 {
		c.WSPort = c.RPCPort + 1
	}
	if c.P2PPort == 0 {
		c.P2PPort = 30303
	}
	if c.AdvertisedHost == "" {
		c.AdvertisedHost = "host.docker.internal"
	}
	if c.RelayAdvertisedHost == "" {
		c.RelayAdvertisedHost = "host.docker.internal"
	}
	if c.GatewayURL == "" {
		c.GatewayURL = fmt.Sprintf("http://localhost:%d", c.RPCPort+8000)
	}
	if c.Currency == "" {
		c.Currency = "LOC"
	}
	if c.TokenSymbol == "" {
		c.TokenSymbol = "t" + c.Currency
	}
	if c.TokenName == "" {
		c.TokenName = "Tokenized " + c.Currency
	}
	if c.FiatTokenSymbol == "" {
		c.FiatTokenSymbol = "f" + c.Currency
	}
	if c.FiatTokenName == "" {
		c.FiatTokenName = "Fiat " + c.Currency
	}
	if c.WaitRPC == nil {
		c.WaitRPC = func(ctx context.Context) error { return waitRPC(ctx, c.SpokeRPC, 60*time.Second) }
	}
	if c.WaitKeycloak == nil {
		c.WaitKeycloak = func(ctx context.Context) error {
			return waitHTTPOK(ctx, fmt.Sprintf("http://localhost:%d/realms/master", c.keycloakPort()), 180*time.Second)
		}
	}
	if c.EnodeReader == nil {
		c.EnodeReader = adminNodeInfoEnode
	}
	if c.ReadClientSecret == nil {
		c.ReadClientSecret = func(context.Context) (string, error) { return spokeKeycloakSecret, nil }
	}
}

func (c SpokeConfig) genesisVolume() string  { return c.VolumePrefix + "_genesis" }
func (c SpokeConfig) besuDataVolume() string { return c.VolumePrefix + "_besu_data" }
func (c SpokeConfig) caVolume() string       { return c.VolumePrefix + "_cb_tls" }

// scenarioBDir is <repo>/scenario-b (parent of ContractsDir), the docker build
// context root for the entity's soft service images.
func (c SpokeConfig) scenarioBDir() string { return filepath.Dir(c.ContractsDir) }

func (c SpokeConfig) keycloakPort() int { return c.RPCPort + 7000 }

// cactiAPIURL is the Cacti relay REST endpoint a backend container uses to reach
// the (external) relay. The relay runs in its own stack on the fixed port 4000;
// containers reach it via the advertised host (default host.docker.internal),
// the same host the relay uses to reach this spoke's published endpoints.
func (c SpokeConfig) cactiAPIURL() string {
	host := c.RelayAdvertisedHost
	if host == "" {
		host = "host.docker.internal"
	}
	return fmt.Sprintf("http://%s:7000", host)
}

// keycloakContainer matches entity-keycloak.compose.yaml's container_name
// (${CONTAINER_PREFIX}-${ENTITY}-keycloak).
func (c SpokeConfig) keycloakContainer() string {
	return c.ContainerPrefix + "-" + c.Entity + "-keycloak"
}

// Local Keycloak realm/client provisioned by found-spoke (OIDC for the spoke
// backend). Mirrors the hub; the client secret is a fixed local-dev value.
const (
	spokeKeycloakRealm  = "cbweb3"
	spokeKeycloakClient = "spoke-backend"
	spokeKeycloakSecret = "spoke-backend-local-secret" // local-only, not a real secret
	// Central-bank login user seeded into the realm (password grant via the
	// confidential client). Holds the roles the v2 AMM routes + governance
	// approve-kyc require. Local-dev credentials only.
	spokeCBUser = "cb-admin"
	spokeCBPass = "cb-admin-local"
)

// spokeCBRoles are the realm roles granted to the CB login user: central_bank
// (v2 AMM pairs/liquidity/currencies), plus ROLE_GOVERNANCE/ROLE_TREASURY for
// governance approve-kyc and treasury operations.
var spokeCBRoles = []string{"central_bank", "ROLE_GOVERNANCE", "ROLE_TREASURY"}

// provisionKeycloakRealm creates the realm + confidential client inside the
// running Keycloak container via kcadm (idempotent: create failures are ignored).
func (c SpokeConfig) provisionKeycloakRealm(ctx context.Context) error {
	kc := "/opt/keycloak/bin/kcadm.sh"
	var b strings.Builder
	fmt.Fprintf(&b, "%[1]s config credentials --server http://localhost:8080 --realm master --user admin --password admin && ", kc)
	fmt.Fprintf(&b, "(%[1]s create realms -s realm=%[2]s -s enabled=true || true) && ", kc, spokeKeycloakRealm)
	fmt.Fprintf(&b, "(%[1]s create clients -r %[2]s -s clientId=%[3]s -s secret=%[4]s -s enabled=true "+
		"-s publicClient=false -s serviceAccountsEnabled=true -s directAccessGrantsEnabled=true || true) && ",
		kc, spokeKeycloakRealm, spokeKeycloakClient, spokeKeycloakSecret)
	// Grant the client's service account the realm-management roles the auth service
	// needs: GetAdminToken uses client_credentials, and onboarding creates + manages
	// the commercial bank's Keycloak user (manage-users) + resolves users on login
	// (view-users). Without this the CB-side credential request 403s at create-user.
	fmt.Fprintf(&b, "(%[1]s add-roles -r %[2]s --uusername service-account-%[3]s "+
		"--cclientid realm-management --rolename manage-users --rolename view-users || true) && ",
		kc, spokeKeycloakRealm, spokeKeycloakClient)
	// Per-role operator accounts from the manifest (spec.adminUsers). Fall back to a
	// single default CB admin when the manifest declares none. Each user's manifest
	// role maps to the realm roles the api-gateway checks (realmRolesForAdminRole).
	users := c.AdminUsers
	if len(users) == 0 {
		users = []AdminUser{{Role: "GOVERNANCE", Username: spokeCBUser, Password: spokeCBPass}}
	}
	appendKeycloakUsers(&b, kc, spokeKeycloakRealm, users)
	_, err := c.Runner.Run(ctx, "docker", "exec", c.keycloakContainer(), "bash", "-c", b.String())
	return err
}

// appendKeycloakUsers appends idempotent kcadm commands that create each admin user
// (username == email; firstName/lastName/emailVerified are REQUIRED so Keycloak 26's
// declarative user profile accepts the password grant), set its password, and grant
// the realm roles mapped from its manifest role. Roles are created idempotently first.
func appendKeycloakUsers(b *strings.Builder, kc, realm string, users []AdminUser) {
	seen := map[string]bool{}
	for _, u := range users {
		for _, r := range realmRolesForAdminRole(u.Role) {
			if !seen[r] {
				seen[r] = true
				fmt.Fprintf(b, "(%[1]s create roles -r %[2]s -s name=%[3]s || true) && ", kc, realm, r)
			}
		}
	}
	for _, u := range users {
		fmt.Fprintf(b, "(%[1]s create users -r %[2]s -s username=%[3]s -s enabled=true "+
			"-s emailVerified=true -s email=%[3]s -s firstName=%[4]s -s lastName=Operator || true) && ",
			kc, realm, u.Username, strings.ToLower(u.Role))
		fmt.Fprintf(b, "(%[1]s set-password -r %[2]s --username %[3]s --new-password %[4]s || true)", kc, realm, u.Username, u.Password)
		for _, r := range realmRolesForAdminRole(u.Role) {
			fmt.Fprintf(b, " && (%[1]s add-roles -r %[2]s --uusername %[3]s --rolename %[4]s || true)", kc, realm, u.Username, r)
		}
		fmt.Fprintf(b, " && ")
	}
	// Trim the trailing " && " so the command chain is well-formed.
	s := b.String()
	b.Reset()
	b.WriteString(strings.TrimSuffix(s, " && "))
}

// ComposeEnv returns the spoke's compose-template interpolation vars as process
// environment (KEY=VALUE), handed to the runner so `docker compose` resolves
// every ${...} WITHOUT persisting plumbing to disk (scenario-a parity: plumbing
// via cmd.Env, state via a slim env file). It carries image names,
// container/network/volume prefixes, host port mappings, and static local infra
// credentials — none discovered at runtime. Runtime-discovered values (contract
// addresses, the Keycloak client secret) are NOT here: those are written to
// SpokeEnvFile mid-run and merged via `--env-file`. The founding CB is its own
// bootnode, so no BOOTNODE_ENODE is emitted (the founder besu template omits
// --bootnodes). Ports derive from RPCPort by fixed offsets.
func (c SpokeConfig) ComposeEnv() []string {
	e := c.ContainerPrefix
	// The spoke backend reaches the hub via host.docker.internal:<HUB_RPC_PORT>;
	// derive the hub's host port from the hub RPC URL (default 8545).
	hubPort := "8545"
	if u, err := url.Parse(c.HubRPC); err == nil && u.Port() != "" {
		hubPort = u.Port()
	}
	vars := map[string]string{
		// besu (founder template)
		"BESU_IMAGE":           c.BesuImage,
		"CONTAINER_PREFIX":     c.ContainerPrefix,
		"ENTITY":               c.Entity,
		"ENTITY_NET_PREFIX":    c.NetPrefix,
		"ENTITY_VOLUME_PREFIX": c.VolumePrefix,
		"ENTITY_RPC_PORT":      itoa(c.RPCPort),
		"ENTITY_WS_PORT":       itoa(c.WSPort),
		"ENTITY_P2P_PORT":      itoa(c.P2PPort),
		// hub RPC (spoke backend → hub via host.docker.internal:<HUB_RPC_PORT>)
		"HUB_RPC_PORT": hubPort,
		// infra: postgres + redis (single DB doubles as the keycloak DB locally)
		"POSTGRES_USER":     "cbweb3",
		"POSTGRES_PASSWORD": "cbweb3",
		"POSTGRES_DB":       "keycloak",
		"POSTGRES_PORT":     itoa(c.RPCPort + 5000),
		"REDIS_PORT":        itoa(c.RPCPort + 6000),
		// keycloak (joins the entity infra network; DB is the infra postgres)
		"KC_ADMIN_USER":     "admin",
		"KC_ADMIN_PASSWORD": "admin",
		"KC_DB_URL":         "jdbc:postgresql://" + e + "-" + c.Entity + "-postgres:5432/keycloak",
		"KEYCLOAK_PORT":     itoa(c.RPCPort + 7000),
		// backend / frontend (images shared with the hub; must be pre-built)
		"GATEWAY_PORT":     itoa(c.RPCPort + 8000),
		"GATEWAY_URL":      fmt.Sprintf("http://localhost:%d", c.RPCPort+8000),
		"BACKEND_IMAGE":              hubBackendImage,
		"COMPLIANCE_IMAGE":           hubComplianceImage,
		"AUTH_IMAGE":                 hubAuthImage,
		"PAYMENT_ORCHESTRATOR_IMAGE": hubPaymentOrchestratorImage,
		// CB operator portals (governance/treasury/supervisor). Each SPA bakes the CB
		// api-gateway URL at build time, so the image is tagged per gateway port. The
		// NOC portal is deployed by the noc template.
		"GOVERNANCE_FRONTEND_IMAGE":  cbFrontendImage("governance", c.RPCPort+8000),
		"GOVERNANCE_FRONTEND_PORT":   itoa(c.RPCPort + 9000),
		// Browser CORS: allow this CB's four operator-portal origins on its gateway.
		"CORS_ALLOW_ORIGINS": corsOriginsCB(c.RPCPort),
		"TREASURY_FRONTEND_IMAGE":    cbFrontendImage("treasury", c.RPCPort+8000),
		"TREASURY_FRONTEND_PORT":     itoa(c.RPCPort + 13000),
		"SUPERVISOR_FRONTEND_IMAGE":  cbFrontendImage("supervisor", c.RPCPort+8000),
		"SUPERVISOR_FRONTEND_PORT":   itoa(c.RPCPort + 14000),
		// app stack (compliance + auth): the CB is the local signer/deployer, and
		// the Keycloak realm/client are provisioned by provision-keycloak-spoke.
		"SPOKE_CHAIN_ID":     fmt.Sprintf("%d", c.SpokeChainID),
		"CB_PRIVATE_KEY":     devDeployerKey,
		// The CB's own on-chain address (the dev deployer). Wired into the api-gateway
		// so its payment routes (backed by the entity-relayer orchestrator via
		// PAYMENT_GRPC_ADDR) resolve the CB's account.
		"ENTITY_BESU_ADDRESS": devDeployerAddr,
		"KEYCLOAK_REALM":      spokeKeycloakRealm,
		"KEYCLOAK_CLIENT_ID":  spokeKeycloakClient,
		// CA (scenario-a standard): the CB CA lives in the cb_tls volume, mounted
		// into compliance at /workspace/backend/config/pki; compliance signs
		// participant CSRs with it.
		"CA_VOLUME":      c.caVolume(),
		"ENTITY_PKI_DIR": "cb_tls", // named volume (holds the generated CA)
		"CA_CERT_FILE":   "/workspace/backend/config/pki/central-bank.crt",
		"CA_KEY_FILE":    "/workspace/backend/config/pki/central-bank.key",
		// Shared secret for the hub-mediated M2M endpoints + cross-currency bridge
		// delegation (a bank delegates bridge-in lock-mint to its CB; bridge-out to CB-B).
		"INTERNAL_RELAY_AUTH_SECRET": hubRelayAuthSecret,
		// Cacti relay endpoint: the cross-currency swap orchestrator delegates the
		// Step 3 bridge-out to the beneficiary CB (CB-B) through it. The relay runs
		// in its own stack, reached from a container via host.docker.internal.
		"CACTI_API_URL": c.cactiAPIURL(),
		// This CB's own spoke id + native tCeBM symbol: the bridge-out receiver
		// enqueues the W-<target> burn against its spoke (e.g. spoke-ars / tCeBM_ARS).
		"SPOKE_NETWORK":       c.SpokeID,
		"NATIVE_ASSET_SYMBOL": "tCeBM_" + c.Currency,
		// noc (observability — soft)
		"NOC_AGENT_BESU_RPC": fmt.Sprintf("http://%s-%s-besu:8545", e, c.Entity),
		"NOC_AGENT_ENTITY":   c.Entity,
		"NOC_AGENT_IMAGE":    hubNocAgentImage,
		"NOC_BACKEND_IMAGE":  hubNocBackendImage,
		"NOC_BACKEND_PORT":   itoa(c.RPCPort + 11000),
		"NOC_DB_NAME":        "noc",
		"NOC_DB_USER":        "cbweb3",
		"NOC_DB_PASSWORD":    "cbweb3",
		"NOC_NET_PREFIX":     c.NetPrefix,
		"NOC_PORTAL_IMAGE":   hubNocPortalImage,
		"NOC_PORTAL_PORT":    itoa(c.RPCPort + 12000),
		"NOC_VOLUME_PREFIX":  c.VolumePrefix,
	}
	env := make([]string, 0, len(vars))
	for k, v := range vars {
		env = append(env, k+"="+v)
	}
	sort.Strings(env) // deterministic order (stable across runs / for tests)
	return env
}

// composeUpArgs builds `compose -p <prefix> -f <tmpl> [--env-file <file>] up -d`.
// The --env-file is added only once SpokeEnvFile exists (it holds runtime-
// discovered addresses + the Keycloak client secret); all plumbing comes from
// the runner's process env (ComposeEnv), so the founder besu can start before
// the file is ever written.
func (c SpokeConfig) composeUpArgs(tmpl string) []string {
	args := []string{"compose", "-p", c.ContainerPrefix, "-f", c.spokeTemplate(tmpl)}
	if fileExists(c.SpokeEnvFile) {
		args = append(args, "--env-file", c.SpokeEnvFile)
	}
	return append(args, "up", "-d")
}

func (c SpokeConfig) spokeTemplate(name string) string {
	return filepath.Join(c.TemplatesDir, name+".compose.yaml")
}

func (c SpokeConfig) spokeBroadcastPath() string {
	return filepath.Join(c.ContractsDir, "broadcast", "CBWeb3Spoke.s.sol",
		fmt.Sprintf("%d", c.SpokeChainID), "run-latest.json")
}

// FoundSpokeSteps builds the ordered found-spoke step set.
func FoundSpokeSteps(c SpokeConfig) []Step {
	c.WithDefaults()
	var capturedEnode string // written by start-besu-spoke, read by emit-spoke-bundle

	compose := func(tmpl string) func(context.Context) error {
		return func(ctx context.Context) error {
			_, err := c.Runner.Run(ctx, "docker", c.composeUpArgs(tmpl)...)
			return err
		}
	}

	steps := []Step{
		{
			Name: "consume-hub-bundle",
			Run: func(context.Context) error {
				_, err := bundle.LoadHub(c.HubBundlePath)
				return err
			},
		},
		{
			Name: "register-cb",
			Deps: []string{"consume-hub-bundle"},
			Check: func(ctx context.Context) (bool, error) {
				if c.CBRegistered != nil {
					return c.CBRegistered(ctx)
				}
				return false, nil
			},
			Run: func(ctx context.Context) error {
				// Self-registration via the hub API (no direct cast/forge): the hub
				// compliance service holds the GOVERNANCE signer and performs the
				// on-chain registerParticipant. Idempotent + guarded by X-Relay-Auth.
				hub, err := bundle.LoadHub(c.HubBundlePath)
				if err != nil {
					return err
				}
				if hub.HubGateway == "" {
					return fmt.Errorf("register-cb: hub bundle has no hubGateway URL")
				}
				cbAddr := c.CBAddress
				if cbAddr == "" {
					cbAddr = devDeployerAddr
				}
				payload, err := json.Marshal(map[string]string{
					"spoke_id":         c.SpokeID,
					"cb_address":       cbAddr,
					"institution_name": c.SpokeID,
					"role":             "ROLE_CENTRAL_BANK",
					"bank_code":        c.SpokeID,
				})
				if err != nil {
					return err
				}
				url := strings.TrimRight(hub.HubGateway, "/") + "/internal/v1/spokes/register"
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
				if err != nil {
					return err
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Relay-Auth", hubRelayAuthSecret)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return fmt.Errorf("register-cb: POST %s: %w", url, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					b, _ := io.ReadAll(resp.Body)
					return fmt.Errorf("register-cb: hub returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
				}
				return nil
			},
		},
		{
			// Ask the HUB to create this spoke's sovereign BRIDGE TOKEN (W-tCeBM_<CUR>)
			// and register the currency. The hub compliance service — which operates
			// the hub chain and holds the governance key — deploys the W-token +
			// setCentralBankOf + registerCurrency on the hub. The spoke NEVER touches
			// the hub chain directly, so this works across a split topology (the spoke
			// need not reach the hub RPC — only the hub gateway). Idempotent.
			// This is the hub-side counterpart of the spoke's SpokeBridge (lock on the
			// spoke → mint W-token on the hub). The FX pair + cooperative liquidity are
			// NOT created here — a CB opens the corridor later from its portal.
			Name: "register-currency",
			Deps: []string{"register-cb"},
			Run: func(ctx context.Context) error {
				hub, err := bundle.LoadHub(c.HubBundlePath)
				if err != nil {
					return err
				}
				if hub.HubGateway == "" {
					return fmt.Errorf("register-currency: hub bundle has no hubGateway URL")
				}
				cbAddr := c.CBAddress
				if cbAddr == "" {
					cbAddr = devDeployerAddr
				}
				payload, err := json.Marshal(map[string]string{
					"currency":   c.Currency,
					"cb_address": cbAddr,
					"spoke_id":   c.SpokeID,
				})
				if err != nil {
					return err
				}
				url := strings.TrimRight(hub.HubGateway, "/") + "/internal/v1/spokes/register-currency"
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
				if err != nil {
					return err
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Relay-Auth", hubRelayAuthSecret)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return fmt.Errorf("register-currency: POST %s: %w", url, err)
				}
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					return fmt.Errorf("register-currency: hub returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
				}
				// Capture the sovereign W-token address so the CB gateway can wire
				// W_TOKEN_ADDRESS (the source→W-token to mint on the hub bridge-in).
				var out struct {
					TokenAddress string `json:"token_address"`
				}
				if json.Unmarshal(body, &out) == nil && out.TokenAddress != "" {
					if err := addrs.AppendAddr(c.SpokeEnvFile, "W_TOKEN_ADDRESS", out.TokenAddress); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Name:  "build-contracts",
			Check: func(context.Context) (bool, error) { return dirNonEmpty(filepath.Join(c.ContractsDir, "out")), nil },
			Run: func(ctx context.Context) error {
				_, err := c.Runner.Run(ctx, "forge", "build", "--root", c.ContractsDir)
				return err
			},
		},
		genGenesisStep("gen-genesis-spoke", c.SpokeChainID, c.genesisVolume(), c.besuDataVolume(), c.ValidatorCount, c.Runner, c.BesuImage),
		{
			Name: "start-besu-spoke",
			Deps: []string{"gen-genesis-spoke"},
			Run: func(ctx context.Context) error {
				// The founding CB is the spoke's sole validator and its own bootnode,
				// so it uses the bootnode-less founder template (later banks join via
				// entity-besu with BOOTNODE_ENODE from the spoke bundle). Plumbing
				// (${...}) comes from the runner's process env (ComposeEnv).
				if _, err := c.Runner.Run(ctx, "docker", c.composeUpArgs("entity-besu-founder")...); err != nil {
					return err
				}
				if err := c.WaitRPC(ctx); err != nil {
					return err
				}
				enode, err := c.EnodeReader(ctx, c.SpokeRPC)
				if err != nil {
					return err
				}
				capturedEnode = enode
				return nil
			},
		},
		{
			Name: "deploy-spoke-contracts",
			Deps: []string{"build-contracts", "start-besu-spoke"},
			// Skip only when the broadcast parses AND its identityRegistry actually
			// has code on the live spoke chain — a stale broadcast over a recreated
			// volume must re-deploy, not skip.
			Check: func(ctx context.Context) (bool, error) {
				m, err := spokeContractMap(c.spokeBroadcastPath())
				if err != nil || m["identityRegistry"] == "" {
					return false, nil
				}
				has, err := contractHasCode(ctx, c.SpokeRPC, m["identityRegistry"])
				if err != nil {
					return false, nil // chain unreachable → re-deploy (safe)
				}
				return has, nil
			},
			Run: func(ctx context.Context) error {
				if err := c.WaitRPC(ctx); err != nil {
					return err
				}
				// Local dev: the CB is admin+deployer unless a distinct CB address is
				// supplied. --legacy for the zero-gas spoke chain; token names derive
				// from the manifest currency.
				cbAddr := c.CBAddress
				if cbAddr == "" {
					cbAddr = devDeployerAddr
				}
				cmd := fmt.Sprintf("cd %q && "+
					"DEPLOYER_PRIVATE_KEY=%s ADMIN_ADDRESS=%s CENTRAL_BANK_ADDRESS=%s "+
					"TOKEN_NAME=%q TOKEN_SYMBOL=%q FIAT_TOKEN_NAME=%q FIAT_TOKEN_SYMBOL=%q "+
					"forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke --rpc-url %s --broadcast --legacy",
					c.ContractsDir, devDeployerKey, devDeployerAddr, cbAddr,
					c.TokenName, c.TokenSymbol, c.FiatTokenName, c.FiatTokenSymbol, c.SpokeRPC)
				_, err := c.Runner.Run(ctx, "sh", "-c", cmd)
				return err
			},
		},
		{
			Name: "wire-hub-addresses",
			Deps: []string{"consume-hub-bundle"},
			Run: func(context.Context) error {
				hub, err := bundle.LoadHub(c.HubBundlePath)
				if err != nil {
					return err
				}
				for key, addr := range map[string]string{
					"HUB_IDENTITY_REGISTRY_ADDRESS":      hub.Contracts["identityRegistry"],
					"HUB_TOKEN_A_ADDRESS":                hub.Contracts["tCeBM_BRL"],
					"HUB_TOKEN_B_ADDRESS":                hub.Contracts["tCeBM_EUR"],
					"FX_AGREEMENT_CONTRACT_ADDRESS":      hub.Contracts["fxAgreement"],
					"PAIR_REGISTRY_CONTRACT_ADDRESS":     hub.Contracts["pairRegistry"],
					"CURRENCY_REGISTRY_CONTRACT_ADDRESS": hub.Contracts["currencyRegistry"],
					// TD-001: no default AMM. The v2 AMM routes are enabled by the
					// PairRegistry (above) and every corridor's AMM is resolved per
					// pool_pair at runtime.
					// Enables SovereignLiquidityService → the sovereign-add + cross-currency
					// bridge-in/out endpoints (registerSovereignRoutes gates on it).
					"LIQUIDITY_COMMIT_REGISTRY_ADDRESS": hub.Contracts["liquidityCommitRegistry"],
				} {
					if err := addrs.AppendAddr(c.SpokeEnvFile, key, addr); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Name: "provision-keycloak-spoke",
			// Infra creates the external infra network + postgres that keycloak joins.
			Deps: []string{"start-spoke-infra"},
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
		{
			Name: "render-spoke-env",
			Deps: []string{"deploy-spoke-contracts"},
			Run: func(context.Context) error {
				m, err := spokeContractMap(c.spokeBroadcastPath())
				if err != nil {
					return err
				}
				for key, addr := range map[string]string{
					"SPOKE_IDENTITY_REGISTRY_ADDRESS": m["identityRegistry"],
					"SPOKE_TCEBM_ADDRESS":             m["tCeBM"],
					"SPOKE_BRIDGE_ADDRESS":            m["spokeBridge"],
					"SPOKE_FCEBM_ADDRESS":             m["fCeBM"],
				} {
					if err := addrs.AppendAddr(c.SpokeEnvFile, key, addr); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			// CB CA (scenario-a standard): seeded into the cb_tls volume, mounted
			// into compliance for CSR signing. Non-destructive (preserves an
			// existing CA); volume-only (no host file).
			Name: "gen-tls-spoke",
			Check: func(ctx context.Context) (bool, error) {
				return volumeHasFile(ctx, c.Runner, c.caVolume(), "central-bank.crt"), nil
			},
			Run: func(ctx context.Context) error { return genCBCA(ctx, c.Runner, c.caVolume()) },
		},
		{Name: "start-spoke-infra", Deps: []string{"render-spoke-env"}, Run: compose("entity-infra")},
		{Name: "start-spoke-backend", Deps: []string{"start-spoke-infra", "render-spoke-env", "provision-keycloak-spoke", "gen-tls-spoke"}, Run: compose("entity-backend")},
		{
			// Bridge RelayerWorker (CB-only): shares the spoke's Postgres DB with the
			// api-gateway and drives cross-currency bridge positions LOCKING→ACTIVE by
			// minting the W-<source> on the hub (hub-only mode). A joining bank has no
			// relayer — it delegates bridge-in lock-mint to its CB.
			Name: "start-spoke-relayer",
			Deps: []string{"start-spoke-backend"},
			Run: func(ctx context.Context) error {
				if err := buildImageIn(ctx, c.Runner, c.scenarioBDir(), hubPaymentOrchestratorImage,
					"backend/services/payment-orchestrator/Dockerfile", "backend"); err != nil {
					return err
				}
				return compose("entity-relayer")(ctx)
			},
		},
		{Name: "start-spoke-frontend", Deps: []string{"start-spoke-backend"}, Soft: true, Run: func(ctx context.Context) error {
			// CB operator portals: governance/treasury/supervisor, each baking this CB's
			// api-gateway URL (browser reaches it on the host at localhost:<gwPort>).
			gwPort := c.RPCPort + 8000
			api := fmt.Sprintf("http://localhost:%d", gwPort)
			sb := c.scenarioBDir()
			if err := buildFrontendImage(ctx, c.Runner, sb, cbFrontendImage("governance", gwPort), "governance",
				map[string]string{"VITE_API_URL": api, "VITE_SCENARIO": "scenario-b", "VITE_INSTITUTION_NAME": c.Entity}); err != nil {
				return err
			}
			if err := buildFrontendImage(ctx, c.Runner, sb, cbFrontendImage("treasury", gwPort), "treasury",
				map[string]string{"VITE_API_BASE_URL": api, "VITE_INSTITUTION_NAME": c.Entity}); err != nil {
				return err
			}
			if err := buildFrontendImage(ctx, c.Runner, sb, cbFrontendImage("supervisor", gwPort), "supervisor",
				map[string]string{"VITE_API_BASE_URL": api, "VITE_SPOKE_NAME": c.SpokeID}); err != nil {
				return err
			}
			return compose("cb-frontend")(ctx)
		}},
		{
			Name: "register-relay-spoke",
			Deps: []string{"start-besu-spoke"},
			Run: func(ctx context.Context) error {
				if c.Registrar == nil {
					return fmt.Errorf("register-relay-spoke: no RelayRegistrar configured")
				}
				// Register endpoints the (external) relay can actually reach: the
				// toolkit runs on the host so its SpokeRPC is localhost, but the relay
				// runs in its own stack, so advertise host.docker.internal:<port>
				// (RelayAdvertisedHost). Mirrors scenario-a's register-relay.
				h := c.RelayAdvertisedHost
				return c.Registrar.Register(ctx, relayregistrar.Spoke{
					ID:         c.SpokeID,
					BesuRPC:    fmt.Sprintf("http://%s:%d", h, c.RPCPort),
					BesuWS:     fmt.Sprintf("ws://%s:%d", h, c.WSPort),
					GatewayURL: fmt.Sprintf("http://%s:%d", h, c.RPCPort+8000),
				})
			},
		},
		{
			Name: "add-noc-agent",
			Deps: []string{"start-besu-spoke"},
			Soft: true, // observability — non-blocking
			Run: func(ctx context.Context) error {
				for _, b := range nocImages {
					if err := buildImageIn(ctx, c.Runner, c.scenarioBDir(), b.image, b.dockerfile, b.context); err != nil {
						return err
					}
				}
				return compose("noc")(ctx)
			},
		},
	}

	// The sovereign FX corridor (propose/confirm pair + cooperative liquidity +
	// oracle rate) is NOT part of provisioning: it is opened at runtime by each
	// central bank through its governance portal (POST /api/v2/amm/pairs/propose
	// + /confirm, /api/v2/amm/liquidity/*, /api/v2/hub/currencies — CB-role, via
	// Keycloak). spec.pair documents the intended corridor; the toolkit never
	// holds sovereign signing keys.
	steps = append(steps, Step{
		Name: "emit-spoke-bundle",
		Deps: []string{"deploy-spoke-contracts", "start-besu-spoke"},
		Check: func(context.Context) (bool, error) {
			_, err := os.Stat(filepath.Join(c.OutDir, "bundles", c.SpokeID+".bundle.yaml"))
			return err == nil, nil
		},
		Run: func(ctx context.Context) error {
			m, err := spokeContractMap(c.spokeBroadcastPath())
			if err != nil {
				return err
			}
			genesisBytes, err := readVolumeFile(ctx, c.Runner, c.genesisVolume(), "genesis.json")
			if err != nil {
				return err
			}
			// On a resumed run start-besu-spoke may be state-skipped, so the
			// captured enode is empty; the node is up (emit deps start-besu-spoke)
			// so read it fresh via admin_nodeInfo.
			enode := capturedEnode
			if enode == "" {
				enode, err = c.EnodeReader(ctx, c.SpokeRPC)
				if err != nil {
					return err
				}
			}
			// admin_nodeInfo advertises 127.0.0.1:30303; rewrite to the externally
			// reachable endpoint so a joining bank can dial it as --bootnodes. A
			// hostname (or the placeholder host.docker.internal) is not accepted by
			// besu's --bootnodes, so resolve the host's LAN IP (scenario-a parity).
			advHost := c.AdvertisedHost
			if advHost == "" || advHost == "host.docker.internal" {
				advHost, err = resolveHostIP()
				if err != nil {
					return err
				}
			}
			enode = rewriteEnodeHost(enode, advHost, c.P2PPort)
			if privateDockerIP.MatchString(enode) {
				return fmt.Errorf("spoke bundle enode %q is loopback/docker-internal (unusable cross-stack); set HOST_IP or node.advertisedHost", enode)
			}
			// Publish the hub contract addresses + RPC port so a joining bank can run
			// the same on-chain per-pair AMM resolver (dynamic swap on any corridor).
			hubForBundle, err := bundle.LoadHub(c.HubBundlePath)
			if err != nil {
				return err
			}
			hubPort := "8545"
			if u, perr := url.Parse(c.HubRPC); perr == nil && u.Port() != "" {
				hubPort = u.Port()
			}
			b := bundle.SpokeBundle{
				SpokeID: c.SpokeID, ChainID: c.SpokeChainID, Enode: enode,
				SpokeRPC: c.SpokeRPC, SpokeWS: c.SpokeWS, Genesis: string(genesisBytes), Contracts: m,
				// Container-reachable CB gateway for a joining bank's CENTRAL_BANK_API_URL.
				CBGateway:    fmt.Sprintf("http://host.docker.internal:%d", c.RPCPort+8000),
				HubContracts: hubForBundle.Contracts,
				HubRPCPort:   hubPort,
			}
			_, err = bundle.EmitSpoke(b, c.OutDir)
			return err
		},
	})
	return steps
}

// spokeContractMap maps the CBWeb3Spoke broadcast deployments to bundle keys.
func spokeContractMap(broadcastPath string) (map[string]string, error) {
	list, err := addrs.ParseBroadcastList(broadcastPath)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, d := range list {
		switch d.Name {
		case "IdentityRegistry":
			out["identityRegistry"] = d.Address
		case "TokenizedCentralBankMoney":
			out["tCeBM"] = d.Address
		case "SpokeBridge":
			out["spokeBridge"] = d.Address
		case "FiatCentralBankMoney":
			out["fCeBM"] = d.Address
		}
	}
	return out, nil
}
