// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// HubConfig parametrizes the found-hub mode. External readiness gates and the
// Keycloak client-secret read are injectable so the step set is testable with a
// FakeRunner and no live Besu/Keycloak.
type HubConfig struct {
	Runner       exec.CommandRunner
	ContractsDir string // <repo>/scenario-b/contracts
	TemplatesDir string // <repo>/scenario-b/provisioning/templates
	OutDir       string // bundle destination
	ChainID      uint64
	HubRPC       string
	HubWS        string
	HubEnvFile   string   // hub .env (render target)
	KeycloakEnv  []string // backend .env files for client-secret write-back

	GenesisDir     string // legacy; node state now lives in named volumes
	ValidatorCount int    // QBFT validators (default 1)
	BesuImage      string // besu image for genesis generation (default hyperledger/besu:25.8.0)

	// Compose interpolation for the hub template (rendered into HubEnvFile before
	// start-besu-hub). Node state is seeded into the `<VolumePrefix>_*` volumes.
	VolumePrefix    string // e.g. "hub-cbweb3"  → <p>_genesis, <p>_besu_data
	ContainerPrefix string // e.g. "sc-b-cbweb3-hub"  → HUB_CONTAINER_PREFIX
	NetPrefix       string // e.g. "hub-cbweb3"  → HUB_NET_PREFIX
	RPCPort         int    // HUB_RPC_PORT (host)
	WSPort          int    // HUB_WS_PORT (host)
	P2PPort         int    // HUB_P2P_PORT (host)
	// AdvertisedHost is the routable host baked into the hub bundle for remote
	// spokes (from manifest node.advertisedHost). Local WaitRPC / forge keep
	// using HubRPC (typically localhost on the hub VM).
	AdvertisedHost string
	// FrontendHost is the browser-facing host baked into the hub governance portal's
	// VITE_API_URL + api-gateway CORS (spec.frontendHost; default localhost).
	FrontendHost string
	// NOCPortalOrigins are extra browser origins for the noc-portal Keycloak client
	// (spec.noc.portalOrigins) — the standalone NOC portal the hub does not serve itself.
	NOCPortalOrigins []string
	// NOCBackendURL is where the hub's noc-agent pushes (spec.noc.backendURL;
	// default host.docker.internal:8090).
	NOCBackendURL string
	ProxyEnabled  bool // spec.proxy == enable: serve the hub governance portal + api behind the per-host reverse proxy

	// Injectable seams (defaults wired by WithDefaults).
	WaitRPC          func(ctx context.Context) error
	WaitKeycloak     func(ctx context.Context) error
	ReadClientSecret func(ctx context.Context) (string, error)
}

// WithDefaults fills real implementations for the injectable seams when unset.
func (c *HubConfig) WithDefaults() {
	if c.ValidatorCount == 0 {
		c.ValidatorCount = 1
	}
	if c.BesuImage == "" {
		c.BesuImage = "hyperledger/besu:25.8.0"
	}
	if c.GenesisDir == "" {
		c.GenesisDir = filepath.Join(c.OutDir, "genesis")
	}
	if c.VolumePrefix == "" {
		c.VolumePrefix = "hub-cbweb3"
	}
	if c.ContainerPrefix == "" {
		c.ContainerPrefix = "sc-b-cbweb3-hub"
	}
	if c.NetPrefix == "" {
		c.NetPrefix = c.VolumePrefix
	}
	if c.NOCBackendURL == "" {
		c.NOCBackendURL = "http://host.docker.internal:8090"
	}
	if c.WaitRPC == nil {
		c.WaitRPC = func(ctx context.Context) error { return waitRPC(ctx, c.HubRPC, 60*time.Second) }
	}
	if c.WaitKeycloak == nil {
		c.WaitKeycloak = func(ctx context.Context) error {
			return waitHTTPOK(ctx, fmt.Sprintf("http://localhost:%d/realms/master", c.keycloakPort()), 180*time.Second)
		}
	}
	if c.ReadClientSecret == nil {
		// The client is created with a fixed local secret (provisionKeycloakRealm).
		c.ReadClientSecret = func(context.Context) (string, error) { return hubKeycloakSecret, nil }
	}
}

func (c HubConfig) broadcastPath() string {
	return filepath.Join(c.ContractsDir, "broadcast", "CBWeb3Hub.s.sol",
		fmt.Sprintf("%d", c.ChainID), "run-latest.json")
}

func (c HubConfig) template(name string) string {
	return filepath.Join(c.TemplatesDir, name+".compose.yaml")
}

func (c HubConfig) genesisVolume() string  { return c.VolumePrefix + "_genesis" }
func (c HubConfig) besuDataVolume() string { return c.VolumePrefix + "_besu_data" }
func (c HubConfig) nocAgentVolume() string { return c.VolumePrefix + "_noc_agent_cfg" }
func (c HubConfig) svcTLSVolume() string   { return c.VolumePrefix + "_svc_tls" }
func (c HubConfig) keycloakPort() int      { return c.RPCPort + 7000 }
func (c HubConfig) keycloakContainer() string {
	return c.ContainerPrefix + "-hub-keycloak"
}

// publicHost is the host remote spokes use to reach this hub. Prefers
// AdvertisedHost; falls back to localhost when unset (single-host labs).
func (c HubConfig) publicHost() string {
	h := strings.TrimSpace(c.AdvertisedHost)
	if h == "" || h == "127.0.0.1" {
		return "localhost"
	}
	return h
}

// publicHubRPC / publicHubWS / publicHubGateway are the URLs written into the
// hub bundle (cross-host). Local orchestration continues to use c.HubRPC.
func (c HubConfig) publicHubRPC() string {
	if c.RPCPort == 0 {
		return c.HubRPC
	}
	return fmt.Sprintf("http://%s:%d", c.publicHost(), c.RPCPort)
}

func (c HubConfig) publicHubWS() string {
	if c.WSPort == 0 {
		return c.HubWS
	}
	return fmt.Sprintf("ws://%s:%d", c.publicHost(), c.WSPort)
}

func (c HubConfig) publicHubGateway() string {
	return fmt.Sprintf("http://%s:%d", c.publicHost(), c.RPCPort+8000)
}

// Local Keycloak realm/client provisioned by found-hub (OIDC for the hub
// backend). The client secret is a fixed local-dev value written back into the
// hub .env; production issues real secrets via the CB/KMS (deferred).
const (
	hubKeycloakRealm  = "cbweb3"
	hubKeycloakClient = "hub-backend"
	hubKeycloakSecret = "hub-backend-local-secret" // local-only, not a real secret
)

// Local images the toolkit builds (image: build) for the hub services.
const (
	hubBackendImage    = "cbweb3b/api-gateway:local"
	hubFrontendImage   = "cbweb3b/governance-frontend:local"
	hubRelayImage      = "cbweb3b/cacti-relay:local"
	hubNocBackendImage = "cbweb3b/noc-backend:local"
	hubNocAgentImage   = "cbweb3b/noc-agent:local"
	hubNocPortalImage  = "cbweb3b/noc-portal:local"
	hubComplianceImage = "cbweb3b/compliance:local"
	hubAuthImage       = "cbweb3b/auth:local"
	// hubPaymentOrchestratorImage runs the bridge RelayerWorker (Scenario B): it
	// polls the shared bridge outbox and drives positions LOCKING→ACTIVE by minting
	// the W-<source> on the hub (hub-only mode). Deployed per CB (found-spoke).
	hubPaymentOrchestratorImage = "cbweb3b/payment-orchestrator:local"

	// HubRelayAuthSecret is the X-Relay-Auth credential shared by an entity's internal
	// endpoints and the relay's inbound routes, including the relay's own
	// POST /api/v1/spokes, which authenticates it since finding R2-M-10. Exported because
	// apply must hand it to the RelayRegistrar: the toolkit is that route's only caller, so
	// the guard and the registrar have to agree on one value.
	//
	// Local-dev value. Deployments override it by setting INTERNAL_RELAY_AUTH_SECRET, which
	// both the relay's compose default and the registrar read; overriding one side alone
	// leaves spokes unable to register.
	HubRelayAuthSecret = "cbweb3-relay-shared-secret"

	// spokeFrontendImage is the per-entity (bank/CB) frontend image built for a
	// spoke; distinct from the hub's governance frontend.
	spokeFrontendImage = "cbweb3b/bank-frontend:local"
)

// scenarioBDir is <repo>/scenario-b (parent of ContractsDir), the docker build
// context root for the services.
func (c HubConfig) scenarioBDir() string { return filepath.Dir(c.ContractsDir) }

// useProxy reports whether the hub serves its portal + api behind the per-host proxy.
func (c HubConfig) useProxy() bool { return c.ProxyEnabled && c.FrontendHost != "" }

// frontendVariant tags the hub frontend image so a proxy (base-path-aware) build is
// never confused with a non-proxy one under the same gateway-port tag.
func (c HubConfig) frontendVariant() string {
	if c.useProxy() {
		return proxyImageVariant(c.FrontendHost)
	}
	return ""
}

// corsOrigins is the hub api-gateway's allowed browser origin(s).
func (c HubConfig) corsOrigins() string {
	if c.useProxy() {
		return proxyOrigin(c.FrontendHost)
	}
	return corsOriginSingle(c.RPCPort, c.FrontendHost)
}

// NetName is the hub's external docker network (created by the infra step).
func (c HubConfig) NetName() string { return c.NetPrefix + "_net" }

// ProxyRoutes are the path routes the reverse proxy exposes for the hub: its governance
// portal + the api-gateway (container names match entity-frontend/entity-backend with
// ENTITY=hub). The hub has no launcher, so the proxy also root-redirects to governance.
func (c HubConfig) ProxyRoutes() []ProxyRoute {
	return []ProxyRoute{
		// entity-frontend.compose.yaml has a single `frontend` service: <PREFIX>-hub-frontend.
		{Segment: "governance", Upstream: fmt.Sprintf("%s-hub-frontend", c.ContainerPrefix) + ":80"},
		{Segment: "api", Upstream: fmt.Sprintf("%s-hub-api-gateway", c.ContainerPrefix) + ":8080", IsAPI: true},
	}
}

// buildImage builds image from dockerfileRel within contextRel (both relative to
// scenario-b), skipping when the image already exists.
func (c HubConfig) buildImage(ctx context.Context, image, dockerfileRel, contextRel string) error {
	return buildImageIn(ctx, c.Runner, c.scenarioBDir(), image, dockerfileRel, contextRel)
}

// accessTokenLifespanSeconds caps how long an issued access token stays valid. Short on
// purpose: a leaked token is only useful for that window, and the portals refresh
// silently through the refresh cookie.
//
// This control used to live in deploy/local/keycloak/init.sh, guarded by
// TestDeployLocalAccessTokenLifespanIsShort. That path was removed as a duplicate of the
// toolkit, which set no lifespan at all — so the guard would have gone silently with the
// scripts. Set here on every realm the toolkit creates.
const accessTokenLifespanSeconds = 300

// buildBackendImage builds the api-gateway image (context: scenario-b/backend).
func (c HubConfig) buildBackendImage(ctx context.Context) error {
	return c.buildImage(ctx, hubBackendImage, "backend/services/api-gateway/Dockerfile", "backend")
}

// provisionKeycloakRealm creates (idempotently) the realm + client with a fixed
// local secret via kcadm inside the running Keycloak container.
func (c HubConfig) provisionKeycloakRealm(ctx context.Context) error {
	kc := "/opt/keycloak/bin/kcadm.sh"
	var b strings.Builder
	// Caveat, stated rather than glossed: kcadm takes the password as an argument, so
	// it transits the Keycloak container's process list for the duration of this exec.
	// There is no env equivalent for `kcadm config credentials` (unlike REDISCLI_AUTH,
	// which is why Redis is handled differently). This is not new — the value used to be
	// the constant admin — but the exposure window is real and belongs in a follow-up
	// once realm provisioning moves to an imported realm file, as Scenario A does it.
	fmt.Fprintf(&b,
		"%[1]s config credentials --server http://localhost:8080 --realm master --user %[2]s --password %[3]s && "+
			"(%[1]s create realms -s realm=%[4]s -s enabled=true || true) && "+
			// Local lab HTTP: relax sslRequired so the browser-direct NOC portal
			// password grant is not rejected with "HTTPS required" (never in prod).
			"(%[1]s update realms/%[4]s -s sslRequired=NONE -s accessTokenLifespan=%[8]d || true) && "+
			"(%[1]s create clients -r %[4]s -s clientId=%[5]s -s secret=%[6]s -s enabled=true "+
			"-s publicClient=false -s serviceAccountsEnabled=true -s directAccessGrantsEnabled=true %[7]s || true) && ",
		kc, "admin", mustInfraSecret(secretsDirOf(c.HubEnvFile), "KC_ADMIN_PASSWORD"),
		hubKeycloakRealm, hubKeycloakClient, hubKeycloakSecret, audienceMapperArg(keycloakBackendAudience),
		accessTokenLifespanSeconds)
	// Public noc-portal client so the hub's co-located NOC portal can password-grant
	// against this realm (hub NOC operator users are a separate follow-up — found-hub
	// does not yet provision operator accounts).
	if err := appendNOCPortalClient(&b, kc, hubKeycloakRealm, nocPortalOrigins(c.RPCPort, c.FrontendHost, c.useProxy(), c.NOCPortalOrigins...)); err != nil {
		return err
	}
	script := strings.TrimSuffix(b.String(), " && ")
	_, err := c.Runner.Run(ctx, "docker", "exec", c.keycloakContainer(), "bash", "-c", script)
	return err
}

// renderHubComposeEnv writes ALL hub compose-template interpolation vars into
// HubEnvFile (besu + infra + keycloak + backend/frontend/relay/noc), so every
// `docker compose --env-file` step resolves its ${...}. Ports derive from the
// besu RPC port by fixed offsets (single hub per host). Local dev creds only.
func (c HubConfig) renderHubComposeEnv() error {
	e := c.ContainerPrefix
	vars := map[string]string{
		// besu (hub template)
		"BESU_IMAGE":           c.BesuImage,
		"HUB_CONTAINER_PREFIX": c.ContainerPrefix,
		"HUB_NET_PREFIX":       c.NetPrefix,
		"HUB_VOLUME_PREFIX":    c.VolumePrefix,
		"HUB_RPC_PORT":         itoa(c.RPCPort),
		"HUB_WS_PORT":          itoa(c.WSPort),
		"HUB_P2P_PORT":         itoa(c.P2PPort),
		// shared entity vars (infra/keycloak/backend/frontend/noc)
		"CONTAINER_PREFIX": c.ContainerPrefix,
		"ENTITY":           "hub",
		// The hub is a single institution, so its own code is fixed. Participants registered on
		// the hub registry carry THEIR institution's code, not this one.
		"INSTITUTION_CODE":     "hub",
		"ENTITY_NET_PREFIX":    c.NetPrefix,
		"ENTITY_VOLUME_PREFIX": c.VolumePrefix,
		"ENTITY_RPC_PORT":      itoa(c.RPCPort),
		// R2-H-8 service-mesh mTLS: the hub's service CA + leaf certs, mounted at
		// /svc-tls in hub-compliance + hub-backend. mTLS activates only when
		// GRPC_MTLS_ENABLE is exported.
		"SVC_TLS_VOLUME": c.svcTLSVolume(),
		// infra: postgres + redis (single DB doubles as the keycloak DB locally)
		"POSTGRES_USER": "cbweb3",
		// Per-entity, generated on first provisioning and read back after; the
		// operator can override via the environment. Never a constant again.
		"POSTGRES_PASSWORD": mustInfraSecret(secretsDirOf(c.HubEnvFile), "POSTGRES_PASSWORD"),
		"REDIS_PASSWORD":    mustInfraSecret(secretsDirOf(c.HubEnvFile), "REDIS_PASSWORD"),
		"POSTGRES_DB":       "keycloak",
		"POSTGRES_PORT":     itoa(c.RPCPort + 5000),
		"REDIS_PORT":        itoa(c.RPCPort + 6000),
		// keycloak (joins the entity infra network; DB is the infra postgres)
		"KC_ADMIN_USER":     "admin",
		"KC_ADMIN_PASSWORD": mustInfraSecret(secretsDirOf(c.HubEnvFile), "KC_ADMIN_PASSWORD"),
		"KC_DB_URL":         "jdbc:postgresql://" + e + "-hub-postgres:5432/keycloak",
		"KEYCLOAK_PORT":     itoa(c.RPCPort + 7000),
		// backend / frontend / relay / noc (images must be pre-built locally)
		"GATEWAY_PORT":  itoa(c.RPCPort + 8000),
		"GATEWAY_URL":   fmt.Sprintf("http://localhost:%d", c.RPCPort+8000),
		"BACKEND_IMAGE": hubBackendImage,
		// compliance (hub self-registration of spokes on the IdentityRegistry)
		"COMPLIANCE_IMAGE":           hubComplianceImage,
		"COMPLIANCE_GRPC_ADDR":       e + "-hub-compliance:9093",
		"HUB_CHAIN_ID":               itoa(int(c.ChainID)),
		"HUB_ADMIN_PRIVATE_KEY":      devDeployerKey, // holds GOVERNANCE_ROLE on the hub registry
		"INTERNAL_RELAY_AUTH_SECRET": HubRelayAuthSecret,
		"FRONTEND_IMAGE":             cbFrontendImage("governance", c.RPCPort+8000, c.frontendVariant()),
		"FRONTEND_PORT":              itoa(c.RPCPort + 9000),
		// Browser CORS: the single proxy origin (path routing), else the hub governance
		// portal origin(s) on the hub gateway (host-port; routable host when set).
		"CORS_ALLOW_ORIGINS":   c.corsOrigins(),
		"RELAY_IMAGE":          hubRelayImage,
		"RELAY_CONTAINER_NAME": e + "-relay",
		"RELAY_NET_PREFIX":     c.NetPrefix,
		"RELAY_VOLUME_PREFIX":  c.VolumePrefix,
		"RELAY_PORT":           "7000",
		"NOC_AGENT_BESU_RPC":   fmt.Sprintf("http://%s-hub-validator:8545", e),
		"NOC_AGENT_ENTITY":     "hub",
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
	for k, v := range vars {
		if err := addrs.AppendAddr(c.HubEnvFile, k, v); err != nil {
			return err
		}
	}
	return nil
}

// FoundHubSteps builds the ordered found-hub step set.
func FoundHubSteps(c HubConfig) []Step {
	c.WithDefaults()
	compose := func(tmpl string) func(context.Context) error {
		return func(ctx context.Context) error {
			_, err := c.Runner.Run(ctx, "docker", "compose", "-p", c.ContainerPrefix, "-f", c.template(tmpl), "--env-file", c.HubEnvFile, "up", "-d")
			return err
		}
	}

	return []Step{
		{
			Name:  "build-contracts",
			Check: func(context.Context) (bool, error) { return dirNonEmpty(filepath.Join(c.ContractsDir, "out")), nil },
			Run: func(ctx context.Context) error {
				_, err := c.Runner.Run(ctx, "forge", "build", "--root", c.ContractsDir)
				return err
			},
		},
		genGenesisHubStep(c),
		{
			Name: "render-hub-compose-env",
			Deps: []string{"gen-genesis-hub"},
			// Always re-render (never state-skipped): config/ports/images must be
			// fresh in the .env before every compose, and AppendAddr is an upsert.
			Check: func(context.Context) (bool, error) { return false, nil },
			Run:   func(context.Context) error { return c.renderHubComposeEnv() },
		},
		{
			Name: "start-besu-hub",
			Deps: []string{"render-hub-compose-env"},
			Run: func(ctx context.Context) error {
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-p", c.ContainerPrefix, "-f", c.template("hub"), "--env-file", c.HubEnvFile, "up", "-d"); err != nil {
					return err
				}
				return c.WaitRPC(ctx)
			},
		},
		{
			Name: "deploy-hub-contracts",
			Deps: []string{"build-contracts", "start-besu-hub"},
			// Skip only when the broadcast parses AND its identityRegistry actually
			// has code on the live chain — a stale broadcast over a recreated volume
			// must re-deploy, not skip.
			Check: func(ctx context.Context) (bool, error) {
				m, err := hubContractMap(c.broadcastPath())
				if err != nil || m["identityRegistry"] == "" {
					return false, nil
				}
				has, err := contractHasCode(ctx, c.HubRPC, m["identityRegistry"])
				if err != nil {
					return false, nil // chain unreachable → re-deploy (safe)
				}
				return has, nil
			},
			Run: func(ctx context.Context) error {
				if err := c.WaitRPC(ctx); err != nil { // readiness gate
					return err
				}
				// Single forge script (the 7 contracts' order is internal to
				// Solidity). Runs inside the contracts project, with the local dev
				// deployer/admin/CB accounts, and --legacy for the zero-gas chain.
				cmd := fmt.Sprintf("cd %q && "+
					"DEPLOYER_PRIVATE_KEY=%s ADMIN_PRIVATE_KEY=%s ADMIN_ADDRESS=%s "+
					"CENTRAL_BANK_ADDRESS=%s CENTRAL_BANK_B_ADDRESS=%s "+
					"forge script script/CBWeb3Hub.s.sol:DeployCBWeb3Hub --rpc-url %s --broadcast --legacy",
					c.ContractsDir, devDeployerKey, devDeployerKey, devDeployerAddr,
					devDeployerAddr, devCBBAddr, c.HubRPC)
				_, err := c.Runner.Run(ctx, "sh", "-c", cmd)
				return err
			},
		},
		// Infra (postgres + redis) creates the entity_infra_network + DB that
		// keycloak/backend join — must come BEFORE them.
		{Name: "start-hub-infra", Deps: []string{"render-hub-compose-env"}, Run: compose("entity-infra")},
		{
			Name: "provision-keycloak-hub",
			Deps: []string{"start-hub-infra"},
			Check: func(context.Context) (bool, error) {
				// idempotent: skip if the secret is already written to all targets
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
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-p", c.ContainerPrefix, "-f", c.template("entity-keycloak"), "--env-file", c.HubEnvFile, "up", "-d"); err != nil {
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
				for _, env := range c.KeycloakEnv { // write-back
					if err := addrs.AppendAddr(env, "KEYCLOAK_CLIENT_SECRET", secret); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			// Mirrors the spoke: provision-keycloak-hub is skipped once KEYCLOAK_CLIENT_SECRET
			// exists, so an origin newly declared in spec.noc.portalOrigins would never reach a
			// hub that is already provisioned. Registering origins is declarative, so it
			// converges on its own every run.
			Name: "reconcile-noc-origins",
			Deps: []string{"provision-keycloak-hub"},
			Check: func(ctx context.Context) (bool, error) {
				return nocOriginsAlreadyRegistered(ctx, c.Runner, c.keycloakContainer(),
					keycloakAdminCLI, hubKeycloakRealm,
					mustInfraSecret(secretsDirOf(c.HubEnvFile), "KC_ADMIN_PASSWORD"),
					nocPortalOrigins(c.RPCPort, c.FrontendHost, c.useProxy(), c.NOCPortalOrigins...))
			},
			Run: func(ctx context.Context) error {
				if _, err := c.Runner.Run(ctx, "docker", "compose", "-p", c.ContainerPrefix,
					"-f", c.template("entity-keycloak"), "--env-file", c.HubEnvFile, "up", "-d"); err != nil {
					return err
				}
				if err := c.WaitKeycloak(ctx); err != nil {
					return err
				}
				return reconcileNOCOrigins(ctx, c.Runner, c.keycloakContainer(),
					keycloakAdminCLI, hubKeycloakRealm,
					mustInfraSecret(secretsDirOf(c.HubEnvFile), "KC_ADMIN_PASSWORD"),
					nocPortalOrigins(c.RPCPort, c.FrontendHost, c.useProxy(), c.NOCPortalOrigins...))
			},
		},
		{
			Name: "render-hub-env",
			Deps: []string{"deploy-hub-contracts"},
			// Always re-render: backend env (contract addresses, secrets) must be
			// fresh before every backend start; AppendAddr upserts.
			Check: func(context.Context) (bool, error) { return false, nil },
			Run: func(context.Context) error {
				m, err := hubContractMap(c.broadcastPath())
				if err != nil {
					return err
				}
				for key, addr := range map[string]string{
					"HUB_IDENTITY_REGISTRY_ADDRESS": m["identityRegistry"],
					// No HUB_TOKEN_A/B_ADDRESS: found-hub deploys no tCeBM (see
					// CBWeb3Hub.s.sol). Mirrored tokens are created per CB currency
					// registration ("W-tCeBM_<ISO>") and resolved from the PairRegistry,
					// so there is no fixed pair of currencies to wire here.
					"FX_AGREEMENT_CONTRACT_ADDRESS":  m["fxAgreement"],
					"PAIR_REGISTRY_CONTRACT_ADDRESS": m["pairRegistry"],
					"CURRENCY_REGISTRY_ADDRESS":      m["currencyRegistry"],
					"MANUAL_ORACLE_ADDRESS":          m["manualOracle"],
				} {
					if addr == "" {
						continue // tCeBM not deployed at found-hub
					}
					if err := addrs.AppendAddr(c.HubEnvFile, key, addr); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			Name: "build-hub-backend-image",
			Check: func(ctx context.Context) (bool, error) {
				// imageExists (not an inline inspect) so --rebuild reaches this gate too.
				return imageExists(ctx, c.Runner, hubBackendImage), nil
			},
			Run: func(ctx context.Context) error { return c.buildBackendImage(ctx) },
		},
		{
			// R2-H-8 service-mesh mTLS: the hub's service CA + leaf certs (hub
			// compliance + hub api-gateway). Harmless until GRPC_MTLS_ENABLE is set.
			Name: "gen-svc-tls-hub",
			Check: func(ctx context.Context) (bool, error) {
				return volumeHasFile(ctx, c.Runner, c.svcTLSVolume(), "svc-ca.crt"), nil
			},
			Run: func(ctx context.Context) error { return genServiceTLS(ctx, c.Runner, c.svcTLSVolume()) },
		},
		{
			// The Cacti relay's own service identity, written into ITS volume so the private key never
			// lands on the host tree. Generated here because the relay is one per deployment, like the
			// hub — each CB then pins the certificate half in its own PKI volume (pin-relay-cert).
			//
			// SOFT: the relay is deployed outside the toolkit, so its volume may not exist yet. Missing
			// it leaves the relay on the shared secret, which is the migration state, not a failure.
			Name: "gen-relay-identity-cacti",
			Soft: true,
			Check: func(ctx context.Context) (bool, error) {
				return volumeHasFile(ctx, c.Runner, relayDataVolume(), relayPeerKeyID+".crt"), nil
			},
			Run: func(ctx context.Context) error {
				return ensureRelayIdentity(ctx, c.Runner, relayDataVolume(), relayPeerKeyID)
			},
		},
		{
			// Compliance holds the GOVERNANCE signer and performs the on-chain
			// registerParticipant when a spoke self-registers (POST
			// /internal/v1/spokes/register). Needs the deployed IdentityRegistry
			// address (render-hub-env) and the shared network (start-besu-hub).
			Name: "start-hub-compliance",
			Deps: []string{"render-hub-env", "start-besu-hub", "gen-svc-tls-hub"},
			Run: func(ctx context.Context) error {
				if err := c.buildImage(ctx, hubComplianceImage, "backend/services/compliance/Dockerfile", "backend"); err != nil {
					return err
				}
				// Build the auth image here too (shared): the hub does not run auth,
				// but every spoke/bank backend does, and images are built once on the
				// hub host before spokes/banks start.
				if err := c.buildImage(ctx, hubAuthImage, "backend/services/auth/Dockerfile", "backend"); err != nil {
					return err
				}
				return compose("entity-compliance")(ctx)
			},
		},
		{Name: "start-hub-backend", Deps: []string{"start-hub-infra", "render-hub-env", "provision-keycloak-hub", "build-hub-backend-image", "start-hub-compliance"}, Run: compose("hub-backend")},
		// UI / relay / NOC are SOFT (non-fatal): they build heavier Node/React
		// images on demand; a build/start failure never blocks the hub's
		// operational core (besu + contracts + infra + keycloak + backend).
		{
			Name: "start-hub-frontend", Deps: []string{"start-hub-backend"}, Soft: true,
			Run: func(ctx context.Context) error {
				// The hub governance portal bakes the hub api-gateway URL at build time. The
				// browser reaches the gateway at frontendHost:<gwPort> (spec.frontendHost — a
				// routable IP/DNS for remote access, else localhost); behind the proxy it is
				// instead same-origin at http://<frontendHost>/<scn>/api/v1/ and the SPA is
				// built base-path-aware (VITE_BASE_PATH) under /<scn>/governance/.
				gwPort := c.RPCPort + 8000
				api := fmt.Sprintf("http://%s:%d", frontendHostOrLocal(c.FrontendHost), gwPort)
				args := map[string]string{"VITE_API_URL": api, "VITE_SCENARIO": "scenario-b", "VITE_INSTITUTION_NAME": "hub"}
				if c.useProxy() {
					args["VITE_API_URL"] = proxyAPIURL(c.FrontendHost)
					args["VITE_BASE_PATH"] = proxyPortalBase("governance")
				}
				if err := buildFrontendImage(ctx, c.Runner, c.scenarioBDir(), cbFrontendImage("governance", gwPort, c.frontendVariant()), "governance", args); err != nil {
					return err
				}
				return compose("entity-frontend")(ctx)
			},
		},
		// NOTE: the Cacti relay is NOT started by the toolkit (scenario-a strategy):
		// it is deployed externally (see provisioning/scripts/start-cacti.sh) and its
		// address is provided via each manifest's spec.relay.endpoint; spokes register
		// dynamically at found-spoke (register-relay-spoke → POST /api/v1/spokes).
		{
			// add-noc-agent runs the hub's own noc-agent (monitors the hub validator
			// besu), pushing to the observe NOC backend. The NOC control plane
			// (db+backend+portal) is a dedicated observe deployment, not the hub.
			// Soft: observability never blocks provisioning.
			Name: "add-noc-agent", Deps: []string{"start-besu-hub"}, Soft: true,
			Run: func(ctx context.Context) error {
				if err := c.buildImage(ctx, hubNocAgentImage,
					"backend/services/noc-agent/Dockerfile", "backend/services/noc-agent"); err != nil {
					return err
				}
				agentCfg, err := renderAgentYAML(c.nocBundle(), c.NOCBackendURL,
					deterministicAgentKey("hub", nocFoundingAgentLabel), 15)
				if err != nil {
					return err
				}
				if err := writeVolumeFile(ctx, c.Runner, c.nocAgentVolume(), "agent.yaml", agentCfg, "0644"); err != nil {
					return err
				}
				return compose("noc-agent")(ctx)
			},
		},
		{
			Name: "emit-hub-bundle",
			Deps: []string{"deploy-hub-contracts"},
			// Skip only when the on-disk bundle already carries the public
			// endpoints (AdvertisedHost). A stale localhost bundle from an older
			// toolkit must re-emit so remote spokes can dial the hub.
			Check: func(context.Context) (bool, error) {
				b, err := bundle.LoadHub(filepath.Join(c.OutDir, "bundles", "hub.bundle.yaml"))
				if err != nil {
					return false, nil
				}
				return b.HubRPC == c.publicHubRPC() &&
					b.HubWS == c.publicHubWS() &&
					b.HubGateway == c.publicHubGateway(), nil
			},
			Run: func(context.Context) error {
				m, err := hubContractMap(c.broadcastPath())
				if err != nil {
					return err
				}
				b := bundle.HubBundle{
					ChainID: c.ChainID,
					// Public endpoints for remote spokes (node.advertisedHost).
					HubRPC:     c.publicHubRPC(),
					HubWS:      c.publicHubWS(),
					HubGateway: c.publicHubGateway(),
					Contracts:  m,
				}
				_, err = bundle.EmitHub(b, c.OutDir)
				return err
			},
		},
		{
			// emit-noc-bundle publishes the hub's monitoring topology (public,
			// no-secrets) so an observe-mode NOC deployment can register the hub
			// and drive its agent. Pure config; Soft (observability never blocks).
			Name: "emit-noc-bundle",
			Deps: []string{"start-hub-infra"},
			Soft: true,
			Check: func(context.Context) (bool, error) {
				b, err := bundle.LoadNOC(filepath.Join(c.OutDir, "bundles", "hub.noc.bundle.yaml"))
				if err != nil {
					return false, nil
				}
				return b.SpokeUUID == deterministicUUID("hub"), nil
			},
			Run: func(context.Context) error {
				_, err := bundle.EmitNOC(c.nocBundle(), c.OutDir)
				return err
			},
		},
	}
}

// hubContractMap maps the CBWeb3Hub broadcast deployments to bundle keys.
// TokenizedCentralBankMoney deployments are ignored: the hub deploys no tCeBM
// (CBWeb3Hub.s.sol) — the mirrored token of each currency is created per CB
// registration and resolved from the PairRegistry at runtime, so an older
// broadcast that still carries them has nothing to contribute to the bundle.
func hubContractMap(broadcastPath string) (map[string]string, error) {
	list, err := addrs.ParseBroadcastList(broadcastPath)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, d := range list {
		switch d.Name {
		case "IdentityRegistry":
			out["identityRegistry"] = d.Address
		case "FXAgreement":
			out["fxAgreement"] = d.Address
		case "PairRegistry":
			out["pairRegistry"] = d.Address
		case "CurrencyRegistry":
			out["currencyRegistry"] = d.Address
		case "ManualOracle":
			out["manualOracle"] = d.Address
		case "AutomatedMarketMaker":
			// TD-001: the hub no longer deploys a default AMM, so this case is
			// normally not hit. Kept for backward compatibility with older broadcasts;
			// corridor AMMs are deployed per-pair at propose time and resolved at runtime.
			out["amm"] = d.Address
		case "LiquidityCommitRegistry":
			// Hub-wide LCR — enables SovereignLiquidityService, which gates the
			// sovereign-add + cross-currency bridge-in/out endpoints.
			out["liquidityCommitRegistry"] = d.Address
		}
	}
	return out, nil
}

func dirNonEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

// waitRPC polls eth_blockNumber until the node answers or the timeout elapses.
func waitRPC(ctx context.Context, rpcURL string, timeout time.Duration) error {
	if rpcURL == "" {
		return nil
	}
	deadline := time.Now().Add(timeout)
	body := []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			// Shared by hub, spoke and join gates — name only the URL so a spoke's
			// own Besu (e.g. found-spoke's SpokeRPC) is never mislabeled "hub RPC".
			return fmt.Errorf("RPC %s not ready within %s", rpcURL, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// waitHTTPOK polls a GET endpoint until it returns a 2xx, or the timeout elapses
// (used to gate on Keycloak readiness). Empty url is a no-op.
func waitHTTPOK(ctx context.Context, url string, timeout time.Duration) error {
	if url == "" {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("endpoint %s not ready within %s", url, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}
