// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	Runner          exec.CommandRunner
	ContractsDir    string
	TemplatesDir    string
	OutDir          string
	SpokeID         string
	SpokeChainID    uint64
	SpokeRPC        string
	SpokeWS         string
	CBAddress       string
	GenesisDir      string
	VolumePrefix    string // <p>_genesis, <p>_besu_data (node state in named volumes)
	ContainerPrefix string // container name prefix (<p>-<entity>-besu, ...)
	NetPrefix       string // docker network name prefix (<p>_besu_network, <p>_infra_network)
	Entity          string // compose ENTITY label (e.g. "central-bank")
	// RelayKeyID identifies this central bank in service-to-service authentication. It must be
	// UNIQUE across the deployment, which ENTITY is not: ENTITY is the ROLE, so every CB carries
	// "central-bank". The receiver pins one public key per id, so a shared id both breaks the
	// mechanism (one entry per id in the registry) and destroys the attribution the mechanism
	// exists for. Set from the manifest name by apply; falls back to the spoke id.
	RelayKeyID string
	// InstitutionCode identifies this central bank as an INSTITUTION on-chain: the services hash
	// it into the institutionId stored on every participant they register, and the AMM
	// circuit-breaker resume quorum counts distinct institutions rather than distinct keys.
	//
	// It must be unique per entity for the same reason RelayKeyID must be, and it cannot be
	// BANK_CODE: that is the entity ROLE, so every central bank carries "central-bank" and all of
	// them would hash to one institution — leaving a 2-of-N resume unreachable and a paused AMM
	// stuck. Falls back to the spoke id, which is unique per CB.
	InstitutionCode     string
	RPCPort             int         // host port -> besu 8545; other service ports derive by offset
	WSPort              int         // host port -> besu 8546
	P2PPort             int         // host port -> besu 30303
	AdvertisedHost      string      // externally reachable host for the spoke bundle enode (default host.docker.internal)
	RelayAdvertisedHost string      // host the (external) relay uses to reach this spoke's RPC/WS/gateway (default host.docker.internal)
	RelayEndpoint       string      // the relay's OWN REST endpoint (spec.relay.endpoint, e.g. http://<hub>:7000) → CACTI_API_URL
	RelayContainerName  string      // relay container on THIS host (spec.relay.containerName), so the noc-agent can collect its logs; empty → no relay logs in the NOC
	FrontendHost        string      // browser-facing host baked into VITE_API_URL + api-gateway CORS (spec.frontendHost; default localhost)
	ProxyEnabled        bool        // spec.proxy == enable: serve portals + api behind the per-host reverse proxy
	LauncherEnabled     bool        // spec.launcher == "enable": bake VITE_LAUNCHER_URL into the CB portals
	LauncherPort        int         // spec.launcherPort: launcher host port (0 → env/default); host = FrontendHost
	AdminUsers          []AdminUser // per-role Keycloak operator accounts (from spec.adminUsers)
	Currency            string      // domestic currency (e.g. BRL) → tCeBM/fCeBM token names
	TokenName           string      // tCeBM name (default "Tokenized <Currency>")
	TokenSymbol         string      // tCeBM symbol (default "tCeBM_<Currency>")
	FiatTokenName       string      // fCeBM name (default "Fiat <Currency>")
	FiatTokenSymbol     string      // fCeBM symbol (default "fCeBM_<Currency>")
	ValidatorCount      int
	BesuImage           string
	HubBundlePath       string
	HubRPC              string // hub RPC (from the bundle unless overridden)
	HubChainID          uint64 // hub chain id (from the bundle); 0 → template default
	SpokeEnvFile        string
	KeycloakEnv         []string
	GatewayURL          string
	NOCBackendURL       string // where this CB's noc-agent pushes (spec.noc.backendURL; default host.docker.internal:8090)
	// NOCPortalOrigins are extra browser origins for the noc-portal Keycloak client
	// (spec.noc.portalOrigins) — the standalone NOC portal this CB does not serve itself.
	NOCPortalOrigins []string
	Registrar        relayregistrar.RelayRegistrar

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
	if c.NOCBackendURL == "" {
		// Single-host default: the entity's noc-agent reaches the observe NOC
		// backend published on the host (default observe backend port 8090).
		c.NOCBackendURL = "http://host.docker.internal:8090"
	}
	// Token symbols MUST carry the "<prefix>_<ISO>" shape: the currency code shown
	// in every portal is derived from the on-chain ERC-20 symbol by taking the
	// segment after the last underscore (backend: currencyCodeFromSymbol; frontend:
	// currencyFromTokenSymbol). A symbol without "_" degrades the UI to the generic
	// "fiat units" label and makes the backend read the whole symbol as the code.
	// It is also the shape the hub uses for the mirrored token ("W-tCeBM_<ISO>").
	if c.TokenSymbol == "" {
		c.TokenSymbol = "tCeBM_" + c.Currency
	}
	if c.TokenName == "" {
		c.TokenName = "Tokenized " + c.Currency
	}
	if c.FiatTokenSymbol == "" {
		c.FiatTokenSymbol = "fCeBM_" + c.Currency
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
func (c SpokeConfig) svcTLSVolume() string   { return c.VolumePrefix + "_svc_tls" }

// cbHubKey / cbHubAddress are this CB's own identity on the HUB chain, derived
// deterministically from the spoke id.
//
// The hub is the only chain every CB shares, so it is the only place where reusing one
// key erases sovereignty: with a single key, every CB's swap carried the founding CB as
// LogSwap.user and every sovereign W-token had the same CENTRAL_BANK_ROLE holder. Spoke
// keys are deliberately untouched — each spoke is its own network, and the spoke deployer
// holds roles granted at deploy time that a rotation would strand.
//
// CBAddress (the -cb-address flag) overrides the address when an operator supplies one,
// but then the matching key must be supplied out of band too; the derived pair is the
// self-consistent default.
func (c SpokeConfig) cbHubKey() string {
	key, _ := deriveCBHubKey(c.SpokeID)
	return key
}

// CBHubAddress exposes this CB's hub identity to the apply layer, which needs it to probe
// register-cb's idempotency against the address that will actually be registered.
func (c SpokeConfig) CBHubAddress() string { return c.cbHubAddress() }

// cbRelayerKey / cbRelayerAddress are the CB's SECOND hub identity, used by its bridge
// relayer so the gateway and the relayer never share a nonce counter across processes.
func (c SpokeConfig) cbRelayerKey() string {
	key, _ := deriveCBRelayerKey(c.SpokeID)
	return key
}

func (c SpokeConfig) cbRelayerAddress() string {
	_, addr := deriveCBRelayerKey(c.SpokeID)
	return addr
}

// cbServicesKey / cbServicesAddress are the identity the CB's auth and compliance containers
// sign the SPOKE IdentityRegistry with — a third identity for the same central bank, so those
// two processes no longer share the deployer's nonce counter with the payment-orchestrator.
func (c SpokeConfig) cbServicesKey() string {
	key, _ := deriveCBServicesKey(c.SpokeID)
	return key
}

func (c SpokeConfig) cbServicesAddress() string {
	_, addr := deriveCBServicesKey(c.SpokeID)
	return addr
}

func (c SpokeConfig) cbHubAddress() string {
	if a := strings.TrimSpace(c.CBAddress); a != "" {
		return a
	}
	_, addr := deriveCBHubKey(c.SpokeID)
	return addr
}

// relayKeyID resolves this CB's service-authentication id, falling back to the spoke id — still
// unique per central bank, and available without threading the manifest name through every path.
func (c SpokeConfig) relayKeyID() string {
	if id := strings.TrimSpace(c.RelayKeyID); id != "" {
		return id
	}
	return c.SpokeID
}

// institutionCode resolves this CB's institution identity, falling back to the spoke id — still
// unique per central bank, unlike the entity role.
func (c SpokeConfig) institutionCode() string {
	if code := strings.TrimSpace(c.InstitutionCode); code != "" {
		return code
	}
	return c.SpokeID
}

func (c SpokeConfig) nocAgentVolume() string { return c.VolumePrefix + "_noc_agent_cfg" }

// scenarioBDir is <repo>/scenario-b (parent of ContractsDir), the docker build
// context root for the entity's soft service images.
func (c SpokeConfig) scenarioBDir() string { return filepath.Dir(c.ContractsDir) }

func (c SpokeConfig) keycloakPort() int { return c.RPCPort + 7000 }

// bundlePublicHost is the host remote commercial banks use to reach this spoke.
// Prefers node.advertisedHost when it is a routable value; when empty /
// host.docker.internal / loopback, resolves the host LAN IP (same path as the
// enode rewrite). Local orchestration continues to use SpokeRPC (localhost).
func (c SpokeConfig) bundlePublicHost() (string, error) {
	h := strings.TrimSpace(c.AdvertisedHost)
	if h == "" || h == "host.docker.internal" || h == "127.0.0.1" || h == "localhost" {
		return resolveHostIP()
	}
	return h, nil
}

// publicSpokeRPC / publicSpokeWS / publicCBGateway are the URLs written into the
// spoke join bundle (cross-host). Mirrors HubConfig.publicHub*.
func (c SpokeConfig) publicSpokeRPC(host string) string {
	return fmt.Sprintf("http://%s:%d", host, c.RPCPort)
}

func (c SpokeConfig) publicSpokeWS(host string) string {
	return fmt.Sprintf("ws://%s:%d", host, c.WSPort)
}

func (c SpokeConfig) publicCBGateway(host string) string {
	return fmt.Sprintf("http://%s:%d", host, c.RPCPort+8000)
}

// containerReachable rewrites a URL's host to host.docker.internal ONLY when it
// is a loopback host (localhost/127.0.0.1/empty), so a container can reach a
// service the toolkit knows by a host-local URL (single-host dev). A routable
// host (LAN/public IP or DNS name) is returned unchanged — this is what lets a
// multi-host deploy point at the real hub/relay instead of the local machine.
func containerReachable(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return raw
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "":
		host := "host.docker.internal"
		if p := u.Port(); p != "" {
			host += ":" + p
		}
		u.Host = host
		return u.String()
	default:
		return raw
	}
}

// HostReachable is the opposite of containerReachable: it rewrites a URL's host to
// localhost ONLY when it is the single-host container sentinel (host.docker.internal /
// host-gateway), so the HOST-RUN toolkit can reach a service published on the local
// host. That name resolves inside containers (via --add-host host-gateway) but NOT
// from the host process, so a host-run call (register-cb / register-currency posting to
// the hub gateway) must use localhost. A routable host (a real IP/DNS — a multi-VM hub)
// is returned unchanged, so a split topology still reaches the hub at its real address.
func HostReachable(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return raw
	}
	switch u.Hostname() {
	case "host.docker.internal", "host-gateway":
		host := "localhost"
		if p := u.Port(); p != "" {
			host += ":" + p
		}
		u.Host = host
		return u.String()
	default:
		return raw
	}
}

// cactiAPIURL is the Cacti relay REST endpoint a backend container uses to reach
// the (external) relay. It is the relay's OWN endpoint (spec.relay.endpoint →
// RelayEndpoint), NOT the host the relay uses to reach this spoke — the relay
// commonly runs on the hub, a different host. Falls back to the single-host
// default when no endpoint is configured.
func (c SpokeConfig) cactiAPIURL() string {
	return relayCactiURL(c.RelayEndpoint)
}

// relayCactiURL maps a relay endpoint (spec.relay.endpoint) to the CACTI_API_URL a
// backend container uses. Container-reachable when routable; single-host default
// when empty. Shared by found-spoke and join.
func relayCactiURL(relayEndpoint string) string {
	if ep := strings.TrimSpace(relayEndpoint); ep != "" {
		return containerReachable(ep)
	}
	return "http://host.docker.internal:7000"
}

// relaySpokeID is the key the relay is registered under for cross-currency
// routing. The api-gateway derives spoke_out/spoke_in as "spoke-<currency>"
// (cross_currency_swap_orchestrator.go), so the relay registration MUST use that
// same convention — NOT the country-based spoke.id — or bridge-out lookups fail
// with "unknown spoke_out". Falls back to SpokeID when no currency is set.
func (c SpokeConfig) relaySpokeID() string {
	if cur := strings.TrimSpace(c.Currency); cur != "" {
		return "spoke-" + strings.ToLower(cur)
	}
	return c.SpokeID
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
	// nocKeycloakClient is the PUBLIC client the co-located NOC portal uses for its
	// direct password grant (no secret). Created in the same realm so a NOC user
	// (adminUsers role: NOC → ROLE_NOC_ADMIN) can log into the portal.
	nocKeycloakClient = "noc-portal"
	// Central-bank login user seeded into the realm (password grant via the
	// confidential client). Holds the roles the v2 AMM routes + governance
	// approve-kyc require. Local-dev credentials only.
	spokeCBUser = "cb-admin"
	spokeCBPass = "cb-admin-local"

	// keycloakBackendAudience is the fixed "aud" the api-gateway auth service
	// enforces (KEYCLOAK_AUDIENCE). An oidc-audience-mapper on every backend
	// login client (spoke-backend, hub-backend) stamps it into access tokens so
	// a token minted for a DIFFERENT client in the same realm (e.g. noc-portal)
	// is rejected at the gateway. Kept in sync with the KEYCLOAK_AUDIENCE default
	// in provisioning/templates/entity-backend.compose.yaml.
	keycloakBackendAudience = "cbweb3-backend"
	// keycloakNOCAudience is stamped on the public noc-portal client so a future
	// production NOC backend can enforce aud. The toolkit's LOCAL NOC runs with
	// NOC_SKIP_AUTH=true and does not validate, so this is future-proofing only.
	keycloakNOCAudience = "cbweb3-noc"
)

// audienceMapperArg returns the kcadm `-s` flag that attaches an
// oidc-audience-mapper to a client AT CREATION TIME, stamping the given custom
// audience into access tokens (not id tokens). Doing it inline with
// `create clients` avoids a second admin round-trip that would need the client
// UUID (kcadm's protocol-mappers endpoint is keyed by the internal id, not the
// clientId). Enforced-issuer note: iss is already deterministic — the auth
// service password-grants server-side against its configured KEYCLOAK_BASE_URL,
// which is the exact host Keycloak stamps into iss.
func audienceMapperArg(audience string) string {
	return fmt.Sprintf(
		`-s 'protocolMappers=[{"name":"cbweb3-audience","protocol":"openid-connect",`+
			`"protocolMapper":"oidc-audience-mapper","config":{`+
			`"included.custom.audience":"%s","access.token.claim":"true","id.token.claim":"false"}}]'`,
		audience)
}

// spokeCBRoles are the realm roles granted to the CB login user: central_bank
// (v2 AMM pairs/liquidity/currencies), plus ROLE_GOVERNANCE/ROLE_TREASURY for
// governance approve-kyc and treasury operations.
var spokeCBRoles = []string{"central_bank", "ROLE_GOVERNANCE", "ROLE_TREASURY"}

// provisionKeycloakRealm creates the realm + confidential client inside the
// running Keycloak container via kcadm (idempotent: create failures are ignored).
func (c SpokeConfig) provisionKeycloakRealm(ctx context.Context) error {
	kc := keycloakAdminCLI
	var b strings.Builder
	// Same password the compose env gave the Keycloak container; resolved from the
	// entity secrets file, not a constant.
	// Caveat, stated rather than glossed: kcadm takes the password as an argument, so
	// it transits the Keycloak container's process list for the duration of this exec.
	// There is no env equivalent for `kcadm config credentials` (unlike REDISCLI_AUTH,
	// which is why Redis is handled differently). This is not new — the value used to be
	// the constant admin — but the exposure window is real and belongs in a follow-up
	// once realm provisioning moves to an imported realm file, as Scenario A does it.
	b.WriteString(kcadmPreamble)
	fmt.Fprintf(&b, "%[1]s config credentials --server http://localhost:8080 --realm master --user admin --password %[2]s && ",
		kc, mustInfraSecret(secretsDirOf(c.SpokeEnvFile), "KC_ADMIN_PASSWORD"))
	fmt.Fprintf(&b, "(%[1]s create realms -s realm=%[2]s -s enabled=true || kcw 'create realm') && ", kc, spokeKeycloakRealm)
	// Local lab uses plain HTTP; the NOC portal does a browser-direct password
	// grant from the entity's IP, which Keycloak's default sslRequired=external
	// rejects with "HTTPS required". Relax it for local (never in production).
	fmt.Fprintf(&b, "(%[1]s update realms/%[2]s -s sslRequired=NONE -s accessTokenLifespan=%[3]d || kcw 'update realm settings') && ",
		kc, spokeKeycloakRealm, accessTokenLifespanSeconds)
	fmt.Fprintf(&b, "(%[1]s create clients -r %[2]s -s clientId=%[3]s -s secret=%[4]s -s enabled=true "+
		"-s publicClient=false -s serviceAccountsEnabled=true -s directAccessGrantsEnabled=true %[5]s || kcw 'create backend client') && ",
		kc, spokeKeycloakRealm, spokeKeycloakClient, spokeKeycloakSecret, audienceMapperArg(keycloakBackendAudience))
	// Grant the client's service account the realm-management roles the auth service
	// needs: GetAdminToken uses client_credentials, and onboarding creates + manages
	// the commercial bank's Keycloak user (manage-users) + resolves users on login
	// (view-users). Without this the CB-side credential request 403s at create-user.
	fmt.Fprintf(&b, "(%[1]s add-roles -r %[2]s --uusername service-account-%[3]s "+
		"--cclientid realm-management --rolename manage-users --rolename view-users || kcw 'grant realm-management roles to the service account') && ",
		kc, spokeKeycloakRealm, spokeKeycloakClient)
	if err := appendNOCPortalClient(&b, kc, spokeKeycloakRealm, nocPortalOrigins(c.RPCPort, c.FrontendHost, c.useProxy(), c.NOCPortalOrigins...)); err != nil {
		return err
	}
	// Per-role operator accounts from the manifest (spec.adminUsers). Fall back to a
	// single default CB admin when the manifest declares none. Each user's manifest
	// role maps to the realm roles the api-gateway checks (realmRolesForAdminRole).
	users := c.AdminUsers
	if len(users) == 0 {
		users = []AdminUser{{Role: "GOVERNANCE", Username: spokeCBUser, Password: spokeCBPass}}
	}
	appendKeycloakUsers(&b, kc, spokeKeycloakRealm, users)
	appendKeycloakAssertions(&b, kc, spokeKeycloakRealm, spokeKeycloakClient, users)
	_, err := c.Runner.Run(ctx, "docker", "exec", c.keycloakContainer(), "bash", "-c", b.String())
	return err
}

// appendNOCPortalClient appends an idempotent kcadm command creating the PUBLIC
// noc-portal client used by the co-located NOC portal's browser password grant.
// publicClient + directAccessGrants (password grant, no secret). Shared by found-spoke
// and found-hub (same realm: cbweb3).
//
// webOrigins is the portal's own origin(s), not "*" (finding R1-10.7). The client needs
// SOME web origin or the browser token fetch fails Keycloak's CORS; scoping it to the
// origin the portal is actually served from is what nocPortalOrigins computes.
//
// The previous value was written as `[\"*\"]`, and that never reached Keycloak. This
// command is handed to `bash -c` as a single argv element, so bash keeps the backslashes
// literal inside the single-quoted -s argument and kcadm answers "Cannot parse the JSON"
// — verified against keycloak:26.0. The `|| true` below then swallowed it, so the
// noc-portal client was NOT created at all on a toolkit-provisioned entity, and the NOC
// portal's password grant had no client to authenticate against. So this fixes a silent
// total failure, not a live wildcard.
//
// The `|| true` on the create keeps the command idempotent (a re-run finds the client
// already there), but on its own it also swallows a genuine failure — which is exactly
// how the escaping bug above stayed invisible. So the create is followed by an explicit
// existence check that exits non-zero when the client is absent: "created, or a named
// error", never "created, or silently missing".
func appendNOCPortalClient(b *strings.Builder, kc, realm string, origins []string) error {
	webOrigins, err := jsonStringArray(origins)
	if err != nil {
		return fmt.Errorf("noc-portal webOrigins: %w", err)
	}
	fmt.Fprintf(b, "(%[1]s create clients -r %[2]s -s clientId=%[3]s -s enabled=true "+
		"-s publicClient=true -s standardFlowEnabled=false -s directAccessGrantsEnabled=true "+
		"-s 'webOrigins=%[5]s' %[4]s || kcw 'create noc-portal client') && ",
		kc, realm, nocKeycloakClient, audienceMapperArg(keycloakNOCAudience), webOrigins)
	fmt.Fprintf(b, "({ %[1]s get clients -r %[2]s -q clientId=%[3]s --fields id | grep -q '\"id\"'; } "+
		"|| { echo 'noc-portal client %[3]s was not created in realm %[2]s' >&2; exit 1; }) && ",
		kc, realm, nocKeycloakClient)
	return nil
}

// browserOriginPattern is the only shape an origin may take on the kcadm command line:
// scheme, host, optional port. Nothing else is JSON-safe AND shell-safe at once, which is
// the property that actually matters here — the value is embedded in a JSON array inside
// a single-quoted argument inside a `bash -c` string.
var browserOriginPattern = regexp.MustCompile(`^https?://[A-Za-z0-9._-]+(:[0-9]{1,5})?$`)

// jsonStringArray renders origins as the JSON array kcadm's -s flag expects, and REFUSES
// anything it cannot prove renders as valid JSON.
//
// Plain double quotes, matching the protocolMappers argument on the same command — the
// form kcadm actually parses. The backslash-escaped form this replaces does not survive
// the single-quoted -s argument (see appendNOCPortalClient above).
//
// Validating against a whitelist rather than stripping a couple of characters is the
// point. The first version stripped `"` and `'` and let `\` through, which produces an
// invalid JSON escape, the same "Cannot parse the JSON" from kcadm, and — behind the
// `|| true` this function's caller now guards — the same silent absence of the client.
// Enumerating the characters that break it is how that class of bug survives; proving
// the ones that work is how it does not. An empty list is an error too: a client with no
// web origin cannot serve the browser grant, and there is no wildcard to fall back to.
func jsonStringArray(values []string) (string, error) {
	if len(values) == 0 {
		return "", errors.New("no origins given; the noc-portal client needs at least one explicit origin")
	}
	for _, v := range values {
		if !browserOriginPattern.MatchString(v) {
			return "", fmt.Errorf("origin %q is not a plain scheme://host[:port] value", v)
		}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode origins: %w", err)
	}
	return string(encoded), nil
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
				fmt.Fprintf(b, "(%[1]s create roles -r %[2]s -s name=%[3]s || kcw 'create realm role') && ", kc, realm, r)
			}
		}
	}
	for _, u := range users {
		fmt.Fprintf(b, "(%[1]s create users -r %[2]s -s username=%[3]s -s enabled=true "+
			"-s emailVerified=true -s email=%[3]s -s firstName=%[4]s -s lastName=Operator || kcw 'create operator user') && ",
			kc, realm, u.Username, strings.ToLower(u.Role))
		fmt.Fprintf(b, "(%[1]s set-password -r %[2]s --username %[3]s --new-password %[4]s || kcw 'set operator password')", kc, realm, u.Username, u.Password)
		for _, r := range realmRolesForAdminRole(u.Role) {
			fmt.Fprintf(b, " && (%[1]s add-roles -r %[2]s --uusername %[3]s --rolename %[4]s || kcw 'grant role to operator')", kc, realm, u.Username, r)
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
// useProxy reports whether this entity serves its portals + api behind the per-host
// reverse proxy (spec.proxy == enable and a browser-facing host is known).
func (c SpokeConfig) useProxy() bool { return c.ProxyEnabled && c.FrontendHost != "" }

// frontendVariant tags a per-entity frontend image so a proxy (base-path-aware) build
// is never confused with a non-proxy one under the same gateway-port tag.
func (c SpokeConfig) frontendVariant() string {
	if c.useProxy() {
		return proxyImageVariant(c.FrontendHost)
	}
	return ""
}

// corsOrigins is the CB api-gateway's allowed browser origins: the single proxy origin
// when behind the proxy (all portals share it), else the four host-port portal origins.
func (c SpokeConfig) corsOrigins() string {
	if c.useProxy() {
		return proxyOrigin(c.FrontendHost)
	}
	return corsOriginsCB(c.RPCPort, c.FrontendHost)
}

// NetName is this entity's external docker network (created by the infra step).
func (c SpokeConfig) NetName() string { return c.NetPrefix + "_net" }

// frontendContainer is the container name of a CB operator portal on the entity network
// (must match cb-frontend.compose.yaml: <CONTAINER_PREFIX>-<ENTITY>-<role>-frontend).
func (c SpokeConfig) frontendContainer(role string) string {
	return fmt.Sprintf("%s-%s-%s-frontend", c.ContainerPrefix, c.Entity, role)
}

// apiGatewayContainer is the api-gateway container name on the entity network
// (must match entity-backend.compose.yaml: <CONTAINER_PREFIX>-<ENTITY>-api-gateway).
func (c SpokeConfig) apiGatewayContainer() string {
	return fmt.Sprintf("%s-%s-api-gateway", c.ContainerPrefix, c.Entity)
}

// ProxyRoutes are the path routes the reverse proxy exposes for this CB: its three
// operator portals + the api-gateway. NOC is intentionally excluded (hub-owned, still
// port-based); see proxy step docs.
func (c SpokeConfig) ProxyRoutes() []ProxyRoute {
	return []ProxyRoute{
		{Segment: "governance", Upstream: c.frontendContainer("governance") + ":80"},
		{Segment: "treasury", Upstream: c.frontendContainer("treasury") + ":80"},
		{Segment: "supervisor", Upstream: c.frontendContainer("supervisor") + ":80"},
		{Segment: "api", Upstream: c.apiGatewayContainer() + ":8080", IsAPI: true},
	}
}

// portalViteArgs are the build-time VITE_* args of this CB's three operator
// portals (governance, treasury, supervisor). api is the browser-facing
// api-gateway URL for a host-port deployment; behind the proxy every portal is
// same-origin and base-path-aware instead.
//
// VITE_FIAT_SYMBOL is the portals' fallback currency label: the displayed code
// normally comes from the on-chain token symbol returned with a balance, and
// without this fallback the SPA shows the generic "fiat units" until (or unless)
// that response arrives. The supervisor portal has no fiat label, so it is
// deliberately not passed there.
func (c SpokeConfig) portalViteArgs(api string) (gov, tre, sup map[string]string) {
	lu := frontendLauncherURL(c.LauncherEnabled, c.useProxy(), c.FrontendHost, c.LauncherPort)
	// VITE_USE_MOCKS is read as `!== "false"`, so it must be this exact string: a toolkit
	// stack is chain-wired and every governance screen has a real endpoint behind it, so
	// serving fixtures here would only hide the live system.
	gov = map[string]string{"VITE_API_URL": api, "VITE_SCENARIO": "scenario-b", "VITE_USE_MOCKS": "false", "VITE_INSTITUTION_NAME": c.Entity, "VITE_LAUNCHER_URL": lu, "VITE_FIAT_SYMBOL": c.Currency}
	tre = map[string]string{"VITE_API_BASE_URL": api, "VITE_INSTITUTION_NAME": c.Entity, "VITE_LAUNCHER_URL": lu, "VITE_FIAT_SYMBOL": c.Currency}
	sup = map[string]string{"VITE_API_BASE_URL": api, "VITE_SPOKE_NAME": c.SpokeID, "VITE_LAUNCHER_URL": lu}
	if c.useProxy() {
		proxied := proxyAPIURL(c.FrontendHost)
		gov["VITE_API_URL"] = proxied
		tre["VITE_API_BASE_URL"] = proxied
		sup["VITE_API_BASE_URL"] = proxied
		gov["VITE_BASE_PATH"] = proxyPortalBase("governance")
		tre["VITE_BASE_PATH"] = proxyPortalBase("treasury")
		sup["VITE_BASE_PATH"] = proxyPortalBase("supervisor")
	}
	return gov, tre, sup
}

// nativeAssetSymbol is the ERC-20 symbol this spoke's tCeBM is actually deployed
// with — the value the backend must see as NATIVE_ASSET_SYMBOL so env and chain
// never disagree. Mirrors the WithDefaults derivation so a caller that builds a
// SpokeConfig by hand (tests) still gets the "<prefix>_<ISO>" shape.
func (c SpokeConfig) nativeAssetSymbol() string {
	if s := strings.TrimSpace(c.TokenSymbol); s != "" {
		return s
	}
	return "tCeBM_" + c.Currency
}

// hubChainIDEnv renders the hub chain id for the compose env, leaving it empty when
// unknown so the template default applies rather than a silent wrong value.
func hubChainIDEnv(id uint64) string {
	if id == 0 {
		return ""
	}
	return fmt.Sprintf("%d", id)
}

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
		// hub RPC: HUB_BESU_RPC_URL is the routable hub RPC (from the hub bundle),
		// mapped container-reachable — this works cross-VM. HUB_RPC_PORT stays for
		// the single-host host.docker.internal fallback in the compose templates.
		"HUB_RPC_PORT":     hubPort,
		"HUB_BESU_RPC_URL": containerReachable(c.HubRPC),
		// Hub chain id: needed so the gateway signs hub transactions for the right chain
		// instead of assuming the local default. Empty when unknown (hand-built configs in
		// tests) so the compose template's own default applies.
		"HUB_CHAIN_ID": hubChainIDEnv(c.HubChainID),
		// infra: postgres + redis (single DB doubles as the keycloak DB locally)
		"POSTGRES_USER": "cbweb3",
		// Per-entity, generated on first provisioning and read back after; the
		// operator can override via the environment. Never a constant again.
		"POSTGRES_PASSWORD": mustInfraSecret(secretsDirOf(c.SpokeEnvFile), "POSTGRES_PASSWORD"),
		"REDIS_PASSWORD":    mustInfraSecret(secretsDirOf(c.SpokeEnvFile), "REDIS_PASSWORD"),
		"POSTGRES_DB":       "keycloak",
		"POSTGRES_PORT":     itoa(c.RPCPort + 5000),
		"REDIS_PORT":        itoa(c.RPCPort + 6000),
		// keycloak (joins the entity infra network; DB is the infra postgres)
		"KC_ADMIN_USER":     "admin",
		"KC_ADMIN_PASSWORD": mustInfraSecret(secretsDirOf(c.SpokeEnvFile), "KC_ADMIN_PASSWORD"),
		"KC_DB_URL":         "jdbc:postgresql://" + e + "-" + c.Entity + "-postgres:5432/keycloak",
		"KEYCLOAK_PORT":     itoa(c.RPCPort + 7000),
		// backend / frontend (images shared with the hub; must be pre-built)
		"GATEWAY_PORT":               itoa(c.RPCPort + 8000),
		"GATEWAY_URL":                fmt.Sprintf("http://localhost:%d", c.RPCPort+8000),
		"BACKEND_IMAGE":              hubBackendImage,
		"COMPLIANCE_IMAGE":           hubComplianceImage,
		"AUTH_IMAGE":                 hubAuthImage,
		"PAYMENT_ORCHESTRATOR_IMAGE": hubPaymentOrchestratorImage,
		// CB operator portals (governance/treasury/supervisor). Each SPA bakes the CB
		// api-gateway URL at build time, so the image is tagged per gateway port. The
		// NOC portal is deployed by the noc template.
		"GOVERNANCE_FRONTEND_IMAGE": cbFrontendImage("governance", c.RPCPort+8000, c.frontendVariant()),
		"GOVERNANCE_FRONTEND_PORT":  itoa(c.RPCPort + 9000),
		// Browser CORS: the single proxy origin (path routing) or the four operator-portal
		// host-port origins (routable host when set) when not behind the proxy.
		"CORS_ALLOW_ORIGINS":        c.corsOrigins(),
		"TREASURY_FRONTEND_IMAGE":   cbFrontendImage("treasury", c.RPCPort+8000, c.frontendVariant()),
		"TREASURY_FRONTEND_PORT":    itoa(c.RPCPort + 13000),
		"SUPERVISOR_FRONTEND_IMAGE": cbFrontendImage("supervisor", c.RPCPort+8000, c.frontendVariant()),
		"SUPERVISOR_FRONTEND_PORT":  itoa(c.RPCPort + 14000),
		// app stack (compliance + auth): the CB is the local signer/deployer, and
		// the Keycloak realm/client are provisioned by provision-keycloak-spoke.
		"SPOKE_CHAIN_ID": fmt.Sprintf("%d", c.SpokeChainID),
		"CB_PRIVATE_KEY": devDeployerKey,
		// Signing identity for auth + compliance. Separate from CB_PRIVATE_KEY so those two
		// containers do not share a nonce counter with the payment-orchestrator; the spoke
		// deploy grants this address GOVERNANCE_ROLE and VERIFIER_ROLE.
		"CB_SERVICES_PRIVATE_KEY": c.cbServicesKey(),
		// Service-to-service authentication id (X-Relay-Key-Id). Distinct from BANK_CODE, which is
		// the entity ROLE and identical on every CB; see SpokeConfig.RelayKeyID.
		"RELAY_KEY_ID": c.relayKeyID(),
		// Institution identity for on-chain participant registration (institutionId =
		// keccak256(INSTITUTION_CODE)). Also distinct from BANK_CODE, and for the same reason.
		"INSTITUTION_CODE": c.institutionCode(),
		// Hub signing key: a CB IS a verified Hub participant, so its gateway signs Hub acts
		// directly — including the AMM swaps it executes on behalf of its member banks
		// (POST /internal/amm/cross-currency-hub-swap). Per-CB and distinct from the spoke
		// deployer key, so this CB's acts on the shared hub are attributable to it alone.
		// Left empty on a bank.
		"HUB_SIGNER_PRIVATE_KEY": c.cbHubKey(),
		// The same identity as an address: the gateway grants it LiquidityProvider on the hub
		// IdentityRegistry at boot (idempotent) and uses it for hub balance reads.
		"LOCAL_CB_HUB_SIGNER": c.cbHubAddress(),
		// The relayer's own hub identity. Separate key so the two processes never claim the
		// same nonce; the gateway grants it CENTRAL_BANK_ROLE at boot, which it can do because
		// the currency handover made the CB the token's administrator.
		"HUB_RELAYER_PRIVATE_KEY": c.cbRelayerKey(),
		"HUB_RELAYER_ADDRESS":     c.cbRelayerAddress(),
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
		// The service images are non-root by default (uid 10001, finding R2-M-12), but the
		// two services that mount the PKI read the CA and persist issued certificates there.
		// On a bank that path is a host bind owned by whoever ran this toolkit, at mode 0700,
		// so no other uid can read it — those containers therefore run as the invoking user.
		// Same reasoning as HOST_UID for the Besu containers (ADR-001 / T028); the compose
		// default keeps a hand-run stack on the image's non-root uid.
		"ENTITY_RUN_UID": strconv.Itoa(os.Getuid()),
		"ENTITY_RUN_GID": strconv.Itoa(os.Getgid()),
		// PKI_DIR points the gateway at that same mount so it can read peer identities. On a CB the
		// peers come from the participants table (the certificates it issued at onboarding, which
		// carry the ACTIVE status and therefore revocation); this path additionally allows pinning a
		// peer that is never onboarded — the Cacti relay — by dropping its <key-id>.crt here.
		//
		// Safe to set even though the CB's own directory holds no peer certificate: the gateway
		// builds the registry from BOTH sources before reporting, so it never sits in the state
		// where a registry is non-empty (its own cert) but has no peer, which would reject every
		// signed request from a bank with 401 instead of falling back to the shared secret.
		"PKI_DIR": "/workspace/backend/config/pki",
		// R2-H-8 service-mesh mTLS: the per-entity service CA + leaf certs live in
		// this volume, mounted read-only at /svc-tls in every backend container.
		// mTLS activates only when GRPC_MTLS_ENABLE is exported (gated in the
		// templates); default-off keeps the existing plaintext transport.
		"SVC_TLS_VOLUME": c.svcTLSVolume(),
		"CA_CERT_FILE":   "/workspace/backend/config/pki/central-bank.crt",
		"CA_KEY_FILE":    "/workspace/backend/config/pki/central-bank.key",
		// Shared secret for the hub-mediated M2M endpoints + cross-currency bridge
		// delegation (a bank delegates bridge-in lock-mint to its CB; bridge-out to CB-B).
		"INTERNAL_RELAY_AUTH_SECRET": HubRelayAuthSecret,
		// Cacti relay endpoint: the cross-currency swap orchestrator delegates the
		// Step 3 bridge-out to the beneficiary CB (CB-B) through it. The relay runs
		// in its own stack, reached from a container via host.docker.internal.
		"CACTI_API_URL": c.cactiAPIURL(),
		// This CB's own spoke id + native tCeBM symbol: the bridge-out receiver
		// enqueues the W-<target> burn against its spoke (e.g. spoke-ars / tCeBM_ARS).
		// The symbol MUST be the one actually deployed on-chain (TokenSymbol), not a
		// string recomposed from the currency — otherwise the env and the ERC-20
		// disagree whenever the manifest overrides spoke.tokenSymbol.
		"SPOKE_NETWORK":       c.SpokeID,
		"NATIVE_ASSET_SYMBOL": c.nativeAssetSymbol(),
		// noc (observability — soft). The agent config is a rendered agent.yaml
		// mounted from NOC_AGENT_VOLUME (seeded by add-noc-agent); the entity joins
		// its own ENTITY_NET_PREFIX network to probe besu by container DNS.
		"NOC_AGENT_BESU_RPC": fmt.Sprintf("http://%s-%s-besu:8545", e, c.Entity),
		"NOC_AGENT_ENTITY":   c.Entity,
		// Supplementary group for the read-only Docker socket the agent tails logs
		// from. The image is non-root (uid 65532) and the socket is root:docker 0660,
		// so without this every log read is denied — silently, because the agent
		// discards that error. Empty here → the compose default → the agent says so at
		// startup (finding R2-M-12).
		"NOC_DOCKER_GID":    dockerSocketGID(),
		"NOC_AGENT_VOLUME":  c.nocAgentVolume(),
		"NOC_AGENT_IMAGE":   hubNocAgentImage,
		"NOC_BACKEND_IMAGE": hubNocBackendImage,
		"NOC_BACKEND_PORT":  itoa(c.RPCPort + 11000),
		"NOC_DB_NAME":       "noc",
		"NOC_DB_USER":       "cbweb3",
		"NOC_DB_PASSWORD":   "cbweb3",
		"NOC_NET_PREFIX":    c.NetPrefix,
		"NOC_PORTAL_IMAGE":  hubNocPortalImage,
		"NOC_PORTAL_PORT":   itoa(c.RPCPort + 12000),
		"NOC_VOLUME_PREFIX": c.VolumePrefix,
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
				// The hub registers THIS CB's own hub identity, not the founder's.
				cbAddr := c.cbHubAddress()
				// bank_code is what the hub's compliance service hashes into the on-chain
				// institutionId, so it must be the SAME code this CB uses on its own spoke
				// registry (INSTITUTION_CODE). Sending the spoke id here instead gave one central
				// bank two institution ids — one per registry. Both were unique, so no quorum was
				// ever satisfiable by a single institution, but "one institution, one id" is the
				// invariant this control rests on, and two ids make it unverifiable by inspection.
				// institutionCode() falls back to SpokeID, so an unconfigured caller keeps the
				// previous value; already-registered CBs keep theirs (registration is idempotent
				// and never rewrites the id), so this aligns fresh provisioning.
				payload, err := json.Marshal(map[string]string{
					"spoke_id":         c.SpokeID,
					"cb_address":       cbAddr,
					"institution_name": c.SpokeID,
					"role":             "ROLE_CENTRAL_BANK",
					"bank_code":        c.institutionCode(),
				})
				if err != nil {
					return err
				}
				url := strings.TrimRight(HostReachable(hub.HubGateway), "/") + "/internal/v1/spokes/register"
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
				if err != nil {
					return err
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Relay-Auth", HubRelayAuthSecret)
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
				// CENTRAL_BANK_ROLE on the sovereign W-token is handed to THIS CB's hub
				// identity, so no other CB can mint or burn this currency.
				cbAddr := c.cbHubAddress()
				payload, err := json.Marshal(map[string]string{
					"currency":   c.Currency,
					"cb_address": cbAddr,
					"spoke_id":   c.SpokeID,
				})
				if err != nil {
					return err
				}
				url := strings.TrimRight(HostReachable(hub.HubGateway), "/") + "/internal/v1/spokes/register-currency"
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
				if err != nil {
					return err
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Relay-Auth", HubRelayAuthSecret)
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
			// Split the W-token's ADMINISTRATION from its ISSUANCE, once the handover has made this
			// CB's gateway the administrator. The gateway keeps CENTRAL_BANK_ROLE (it mints when
			// provisioning liquidity) and loses DEFAULT_ADMIN_ROLE to an identity whose key is never
			// handed to a container. The relayer's issuance grant moves here from the gateway's boot
			// sequence, because after the revoke the gateway can no longer grant anything.
			//
			// Idempotent by reading the chain: an already separated token yields no transaction.
			Name: "separate-token-admin",
			Deps: []string{"register-currency"},
			Run: func(ctx context.Context) error {
				token := addrs.ReadAddr(c.SpokeEnvFile, "W_TOKEN_ADDRESS")
				if token == "" {
					return fmt.Errorf("separate-token-admin: W_TOKEN_ADDRESS not found in %s — register-currency must run first", c.SpokeEnvFile)
				}
				gatewayKey, gatewayAddr := deriveCBHubKey(c.SpokeID)
				_, relayerAddr := deriveCBRelayerKey(c.SpokeID)
				_, adminAddr := deriveCBTokenAdminKey(c.SpokeID)
				e := tokenAdminExec{
					Runner: c.Runner, RPCURL: HostReachable(c.HubRPC), Token: token,
					Signer: gatewayKey, Gateway: gatewayAddr, Relayer: relayerAddr, Admin: adminAddr,
				}
				if _, err := e.apply(ctx); err != nil {
					return fmt.Errorf("separate-token-admin: %w", err)
				}
				// The administrator ADDRESS is what an operator needs to audit the split, and the only
				// part of that identity that may be published. Written on every run, not just when
				// acts were applied: a stack separated by an earlier version would otherwise never
				// record it.
				return addrs.AppendAddr(c.SpokeEnvFile, "W_TOKEN_ADMIN_ADDRESS", adminAddr)
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
				// CENTRAL_BANK_ROLE on the SPOKE tokens must belong to the address that
				// actually signs spoke transactions — the deployer, which is what the CB's
				// relayer and gateway use (CB_PRIVATE_KEY / BESU_OPERATOR_KEY). This is NOT
				// the CB's hub identity: granting it here would leave the relayer unable to
				// mint or burn tCeBM on its own spoke. --legacy for the zero-gas chain.
				cbAddr := devDeployerAddr
				// SERVICES_ADDRESS gets GOVERNANCE_ROLE + VERIFIER_ROLE so auth and compliance
				// can sign with their own identity instead of the deployer key. Sharing that key
				// put three containers on one nonce counter, and concurrent writes replaced each
				// other in the mempool.
				cmd := fmt.Sprintf("cd %q && "+
					"DEPLOYER_PRIVATE_KEY=%s ADMIN_ADDRESS=%s CENTRAL_BANK_ADDRESS=%s SERVICES_ADDRESS=%s "+
					"TOKEN_NAME=%q TOKEN_SYMBOL=%q FIAT_TOKEN_NAME=%q FIAT_TOKEN_SYMBOL=%q "+
					"forge script script/CBWeb3Spoke.s.sol:DeployCBWeb3Spoke --rpc-url %s --broadcast --legacy",
					c.ContractsDir, devDeployerKey, devDeployerAddr, cbAddr, c.cbServicesAddress(),
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
					"HUB_IDENTITY_REGISTRY_ADDRESS": hub.Contracts["identityRegistry"],
					// No HUB_TOKEN_A/B_ADDRESS: the hub deploys no tCeBM (CBWeb3Hub.s.sol),
					// every corridor's mirrored tokens are created per currency registration
					// and resolved from the PairRegistry at runtime. Wiring a fixed pair of
					// currencies here would hardcode a bilateral BRL/EUR assumption.
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
					if addr == "" {
						continue
					}
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
			// provision-keycloak-spoke is skipped once KEYCLOAK_CLIENT_SECRET exists, so an
			// origin newly declared in spec.noc.portalOrigins would never reach an entity that
			// is already provisioned — the standalone NOC portal stays dead behind a CORS
			// refusal until a from-scratch redeploy. Registering origins is declarative and
			// cheap, so it converges on its own every run.
			Name: "reconcile-noc-origins",
			Deps: []string{"provision-keycloak-spoke"},
			Check: func(ctx context.Context) (bool, error) {
				return nocOriginsAlreadyRegistered(ctx, c.Runner, c.keycloakContainer(),
					keycloakAdminCLI, spokeKeycloakRealm,
					mustInfraSecret(secretsDirOf(c.SpokeEnvFile), "KC_ADMIN_PASSWORD"),
					nocPortalOrigins(c.RPCPort, c.FrontendHost, c.useProxy(), c.NOCPortalOrigins...))
			},
			Run: func(ctx context.Context) error {
				// Self-sufficient: the Check reaches this Run when Keycloak could not be asked
				// at all, which on a provisioned entity with its containers down is ordinary.
				if _, err := c.Runner.Run(ctx, "docker", c.composeUpArgs("entity-keycloak")...); err != nil {
					return err
				}
				if err := c.WaitKeycloak(ctx); err != nil {
					return err
				}
				return reconcileNOCOrigins(ctx, c.Runner, c.keycloakContainer(),
					keycloakAdminCLI, spokeKeycloakRealm,
					mustInfraSecret(secretsDirOf(c.SpokeEnvFile), "KC_ADMIN_PASSWORD"),
					nocPortalOrigins(c.RPCPort, c.FrontendHost, c.useProxy(), c.NOCPortalOrigins...))
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
		{
			// R2-H-8 service-mesh mTLS: per-entity service CA + one leaf per gRPC
			// service, seeded into the svc_tls volume (mounted at /svc-tls). Harmless
			// on its own — mTLS only activates when GRPC_MTLS_ENABLE is exported.
			Name: "gen-svc-tls-spoke",
			Check: func(ctx context.Context) (bool, error) {
				return volumeHasFile(ctx, c.Runner, c.svcTLSVolume(), "svc-ca.crt"), nil
			},
			Run: func(ctx context.Context) error { return genServiceTLS(ctx, c.Runner, c.svcTLSVolume()) },
		},
		{
			// Pin the Cacti relay's certificate in this CB's PKI volume. The relay calls this CB's
			// internal bridge-out endpoint but is never an onboarded participant, so a file pin is the
			// only source available for it — which is why the registry keeps files as a second source.
			//
			// SOFT: the relay is deployed outside the toolkit (provisioning/scripts/start-cacti.sh), so
			// its identity may legitimately not exist yet. Missing it costs nothing today — the relay
			// falls back to the shared secret — and it is what RELAY_REQUIRE_SIGNATURE will require.
			Name: "pin-relay-cert",
			Deps: []string{"gen-relay-identity"},
			Soft: true,
			Check: func(ctx context.Context) (bool, error) {
				return volumeHasFile(ctx, c.Runner, c.caVolume(), relayPeerKeyID+".crt"), nil
			},
			Run: func(ctx context.Context) error {
				return pinPeerCert(ctx, c.Runner, relayDataVolume(), c.caVolume(), relayPeerKeyID)
			},
		},
		{
			// The CB's own SERVICE identity: the key it signs with and the certificate peers pin.
			// It cannot come from onboarding (a CB does not onboard itself), and two consumers need
			// it — relay auth when this CB is the sender, and the circuit-breaker institutional
			// attestation, which the gateway previously skipped entirely because PKI_DIR was unset.
			//
			// A step of its own rather than part of gen-tls-spoke: that step's Check is satisfied by
			// the CA's presence, so anything folded into it is skipped on every stack that already
			// has a CA — which is every existing one.
			Name: "gen-relay-identity",
			Deps: []string{"gen-tls-spoke"},
			Check: func(ctx context.Context) (bool, error) {
				return volumeHasFile(ctx, c.Runner, c.caVolume(), c.relayKeyID()+".crt"), nil
			},
			Run: func(ctx context.Context) error {
				return ensureRelayIdentity(ctx, c.Runner, c.caVolume(), c.relayKeyID())
			},
		},
		{Name: "start-spoke-infra", Deps: []string{"render-spoke-env"}, Run: compose("entity-infra")},
		{Name: "start-spoke-backend", Deps: []string{"start-spoke-infra", "render-spoke-env", "provision-keycloak-spoke", "gen-tls-spoke", "gen-relay-identity", "pin-relay-cert", "gen-svc-tls-spoke"}, Run: func(ctx context.Context) error {
			// Build the backend images before `compose up`. The hub host builds these
			// too, but a spoke on a SEPARATE Docker daemon (multi-VM lab) never has
			// them, so compose would try to PULL a local-only tag and fail. Idempotent
			// via imageExists, so this is a no-op when the hub already built them
			// (single-host). Mirrors start-spoke-relayer / start-spoke-frontend.
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
			// api-gateway URL at build time. The browser reaches the gateway at
			// frontendHost:<gwPort> (spec.frontendHost — a routable IP/DNS for remote
			// access, else localhost); behind the proxy every portal is instead same-origin
			// at http://<frontendHost>/<scn>/api/v1/ and each SPA is built base-path-aware
			// (VITE_BASE_PATH) so it is served under /<scn>/<role>/.
			gwPort := c.RPCPort + 8000
			api := fmt.Sprintf("http://%s:%d", frontendHostOrLocal(c.FrontendHost), gwPort)
			variant := c.frontendVariant()
			sb := c.scenarioBDir()
			gov, tre, sup := c.portalViteArgs(api)
			if err := buildFrontendImage(ctx, c.Runner, sb, cbFrontendImage("governance", gwPort, variant), "governance", gov); err != nil {
				return err
			}
			if err := buildFrontendImage(ctx, c.Runner, sb, cbFrontendImage("treasury", gwPort, variant), "treasury", tre); err != nil {
				return err
			}
			if err := buildFrontendImage(ctx, c.Runner, sb, cbFrontendImage("supervisor", gwPort, variant), "supervisor", sup); err != nil {
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
					ID:         c.relaySpokeID(),
					BesuRPC:    fmt.Sprintf("http://%s:%d", h, c.RPCPort),
					BesuWS:     fmt.Sprintf("ws://%s:%d", h, c.WSPort),
					GatewayURL: fmt.Sprintf("http://%s:%d", h, c.RPCPort+8000),
				})
			},
		},
		{
			// add-noc-agent runs THIS CB's own noc-agent: it renders a
			// multi-component agent.yaml from the spoke's NOC bundle (spoke UUID +
			// the CB's deterministic agent key), seeds it into a named volume, and
			// starts the agent, which pushes to the observe NOC backend. Node-level
			// monitoring of the CB; Soft (observability never blocks provisioning).
			Name: "add-noc-agent",
			Deps: []string{"start-besu-spoke"},
			Soft: true,
			Run: func(ctx context.Context) error {
				if err := buildImageIn(ctx, c.Runner, c.scenarioBDir(), hubNocAgentImage,
					"backend/services/noc-agent/Dockerfile", "backend/services/noc-agent"); err != nil {
					return err
				}
				agentCfg, err := renderAgentYAML(c.nocBundle(), c.NOCBackendURL,
					deterministicAgentKey(c.SpokeID, nocFoundingAgentLabel), 15)
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

	// The sovereign FX corridor (propose/confirm pair + cooperative liquidity +
	// oracle rate) is NOT part of provisioning: it is opened at runtime by each
	// central bank through its governance portal (POST /api/v2/amm/pairs/propose
	// + /confirm, /api/v2/amm/liquidity/*, /api/v2/hub/currencies — CB-role, via
	// Keycloak). The manifest declares no corridor: the toolkit never holds
	// sovereign signing keys.
	steps = append(steps, Step{
		Name: "emit-spoke-bundle",
		Deps: []string{"deploy-spoke-contracts", "start-besu-spoke"},
		// Skip only when the on-disk bundle matches the LIVE node: the public
		// endpoints (AdvertisedHost), the genesis, and the node identity. A stale
		// localhost / host.docker.internal bundle from an older toolkit must re-emit
		// so remote banks can dial the CB.
		//
		// Endpoints alone are not enough. Re-founding a spoke keeps the same host and
		// ports but produces a FRESH node key and genesis, and the emitted bundle
		// lives outside the wiped Docker volumes — so it survives. A joining bank
		// would then write the previous genesis (different genesis hash ⇒ peering is
		// impossible) and dial a bootnode that no longer exists, and the only symptom
		// is wait-sync timing out after 10 minutes at block 0 with no error.
		Check: func(ctx context.Context) (bool, error) {
			b, err := bundle.LoadSpoke(filepath.Join(c.OutDir, "bundles", c.SpokeID+".bundle.yaml"))
			if err != nil {
				return false, nil
			}
			host, err := c.bundlePublicHost()
			if err != nil {
				return false, nil
			}
			if b.SpokeRPC != c.publicSpokeRPC(host) ||
				b.SpokeWS != c.publicSpokeWS(host) ||
				b.CBGateway != c.publicCBGateway(host) {
				return false, nil
			}
			genesisBytes, err := readVolumeFile(ctx, c.Runner, c.genesisVolume(), "genesis.json")
			if err != nil || len(genesisBytes) == 0 || string(genesisBytes) != b.Genesis {
				return false, nil // unreadable or diverged → re-emit (safe)
			}
			live := capturedEnode
			if live == "" {
				if live, err = c.EnodeReader(ctx, c.SpokeRPC); err != nil {
					return false, nil
				}
			}
			id := enodeNodeID(live)
			return id != "" && id == enodeNodeID(b.Enode), nil
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
			// The same host is baked into spokeRpc / spokeWs / cbGateway.
			advHost, err := c.bundlePublicHost()
			if err != nil {
				return err
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
				// Public endpoints for remote banks (node.advertisedHost). Local
				// orchestration keeps using c.SpokeRPC (typically localhost).
				SpokeRPC: c.publicSpokeRPC(advHost), SpokeWS: c.publicSpokeWS(advHost),
				Genesis: string(genesisBytes), Contracts: m,
				CBGateway:    c.publicCBGateway(advHost),
				HubContracts: hubForBundle.Contracts,
				HubRPCPort:   hubPort,
				// Routable hub RPC (from the hub bundle) so a joining bank reaches the
				// hub cross-VM (HUB_BESU_RPC_URL) instead of host.docker.internal.
				HubRPC: hubForBundle.HubRPC,
				// Sovereign currency of this spoke: the join cross-checks it against the
				// joining bank's manifest so a bank cannot attach to a BRL spoke while
				// claiming another currency.
				Currency: c.Currency,
			}
			_, err = bundle.EmitSpoke(b, c.OutDir)
			return err
		},
	})

	// emit-noc-bundle publishes this spoke's monitoring topology (the public,
	// no-secrets NOC bundle) so an observe-mode NOC deployment can register the
	// spoke and drive its agent. Pure config → no runtime deps beyond the node
	// being up; Soft (observability never blocks provisioning).
	steps = append(steps, Step{
		Name: "emit-noc-bundle",
		Deps: []string{"start-besu-spoke"},
		Soft: true,
		Check: func(context.Context) (bool, error) {
			b, err := bundle.LoadNOC(filepath.Join(c.OutDir, "bundles", c.SpokeID+".noc.bundle.yaml"))
			if err != nil {
				return false, nil
			}
			return b.SpokeUUID == deterministicUUID(c.SpokeID), nil
		},
		Run: func(context.Context) error {
			_, err := bundle.EmitNOC(c.nocBundle(), c.OutDir)
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
