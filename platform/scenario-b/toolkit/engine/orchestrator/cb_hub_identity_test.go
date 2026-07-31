package orchestrator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// The hub is the only chain every CB shares. A single key there made each CB's acts appear
// on-chain as the founding CB — same LogSwap.user, same CENTRAL_BANK_ROLE holder on every
// sovereign W-token. These tests pin the derivation and the wiring that fix it.

func TestDeriveCBHubKey_DeterministicAndPerSpoke(t *testing.T) {
	k1, a1 := deriveCBHubKey("spoke-brl")
	k2, a2 := deriveCBHubKey("spoke-brl")
	if k1 != k2 || a1 != a2 {
		t.Fatalf("derivation must be stable across runs, or a re-apply orphans the hub-side registration of the previous one")
	}

	_, aCop := deriveCBHubKey("spoke-cop")
	if a1 == aCop {
		t.Fatalf("two spokes derived the same hub identity (%s) — CB-B would act as CB-A on the hub", a1)
	}

	if strings.EqualFold(a1, devDeployerAddr) {
		t.Fatalf("CB hub identity collides with the founder/deployer address %s", devDeployerAddr)
	}
	if strings.EqualFold(k1, devDeployerKey) {
		t.Fatalf("CB hub key collides with the deployer key")
	}
	if !strings.HasPrefix(k1, "0x") || len(k1) != 66 {
		t.Fatalf("unexpected private key shape: %q", k1)
	}
}

// The bank derivation shares the mechanism but must not share the domain: a spoke id and a
// bank code must never derive the same key.
func TestDeriveCBHubKey_DistinctDomainFromBankKey(t *testing.T) {
	_, cbAddr := deriveCBHubKey("acme")
	_, bankAddr := deriveBankKey("acme")
	if cbAddr == bankAddr {
		t.Fatalf("CB and bank derivations collide for the same id (%s)", cbAddr)
	}
}

func TestComposeEnvCarriesPerCBHubIdentity(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.CBAddress = "" // no operator override → derived identity
	env := envMap(cfg.ComposeEnv())

	wantKey, wantAddr := deriveCBHubKey(cfg.SpokeID)
	if env["HUB_SIGNER_PRIVATE_KEY"] != wantKey {
		t.Fatalf("HUB_SIGNER_PRIVATE_KEY = %q, want the derived per-CB key", env["HUB_SIGNER_PRIVATE_KEY"])
	}
	if env["HUB_SIGNER_PRIVATE_KEY"] == env["CB_PRIVATE_KEY"] {
		t.Fatalf("the hub signing key must differ from the spoke deployer key")
	}
	if env["LOCAL_CB_HUB_SIGNER"] != wantAddr {
		t.Fatalf("LOCAL_CB_HUB_SIGNER = %q, want %q", env["LOCAL_CB_HUB_SIGNER"], wantAddr)
	}
	// The spoke deployer key stays put: it holds the roles granted when this spoke's own
	// contracts were deployed, and rotating it would strand them.
	if env["CB_PRIVATE_KEY"] != devDeployerKey {
		t.Fatalf("CB_PRIVATE_KEY = %q, want the spoke deployer key unchanged", env["CB_PRIVATE_KEY"])
	}
}

// One central bank, two hub identities. go-ethereum tracks nonces per process, so the gateway
// and the relayer sharing one key means each keeps its own counter and concurrent submissions
// claim the same nonce — and a replaced transaction leaves a position waiting forever on a hash
// that never mines, because the relayer persists it as an intent on broadcast.
func TestDeriveCBRelayerKey_DistinctFromTheGatewayIdentity(t *testing.T) {
	gwKey, gwAddr := deriveCBHubKey("spoke-brl")
	rlKey, rlAddr := deriveCBRelayerKey("spoke-brl")

	if gwKey == rlKey || gwAddr == rlAddr {
		t.Fatalf("the relayer must not share the gateway's key/address (%s)", gwAddr)
	}
	if k2, a2 := deriveCBRelayerKey("spoke-brl"); k2 != rlKey || a2 != rlAddr {
		t.Fatalf("relayer derivation must be stable across runs, or its role grant is orphaned")
	}
	if _, other := deriveCBRelayerKey("spoke-ars"); other == rlAddr {
		t.Fatalf("two spokes derived the same relayer identity (%s)", rlAddr)
	}
	// It must also stay clear of every other derivation domain.
	if _, bankAddr := deriveBankKey("spoke-brl"); bankAddr == rlAddr {
		t.Fatalf("relayer derivation collides with the bank domain (%s)", rlAddr)
	}
	if strings.EqualFold(rlAddr, devDeployerAddr) {
		t.Fatalf("relayer identity collides with the founder/deployer address")
	}
}

func TestComposeEnvCarriesTheRelayerIdentity(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	env := envMap(cfg.ComposeEnv())

	wantKey, wantAddr := deriveCBRelayerKey(cfg.SpokeID)
	if env["HUB_RELAYER_PRIVATE_KEY"] != wantKey {
		t.Fatalf("HUB_RELAYER_PRIVATE_KEY = %q, want the derived relayer key", env["HUB_RELAYER_PRIVATE_KEY"])
	}
	// The address goes to the GATEWAY, which grants the role; the key goes to the relayer.
	if env["HUB_RELAYER_ADDRESS"] != wantAddr {
		t.Fatalf("HUB_RELAYER_ADDRESS = %q, want %q", env["HUB_RELAYER_ADDRESS"], wantAddr)
	}
	if env["HUB_RELAYER_PRIVATE_KEY"] == env["HUB_SIGNER_PRIVATE_KEY"] {
		t.Fatalf("the relayer and the gateway must not share a hub key — that is the nonce collision")
	}
	if env["HUB_RELAYER_PRIVATE_KEY"] == env["CB_PRIVATE_KEY"] {
		t.Fatalf("the relayer hub key must differ from the spoke deployer key")
	}
}

func TestComposeEnvHubChainID(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.HubChainID = 1337
	if got := envMap(cfg.ComposeEnv())["HUB_CHAIN_ID"]; got != "1337" {
		t.Fatalf("HUB_CHAIN_ID = %q, want 1337", got)
	}

	// Unknown chain id stays empty so the compose template's own default applies rather
	// than a silently wrong value.
	cfg.HubChainID = 0
	if got := envMap(cfg.ComposeEnv())["HUB_CHAIN_ID"]; got != "" {
		t.Fatalf("HUB_CHAIN_ID = %q, want empty when unknown", got)
	}
}

// register-cb must register the CB's OWN hub identity. Registering the founder's address
// instead is what left CB-B unregistered while looking registered.
func TestRegisterCBUsesDerivedHubIdentity(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"already_registered":false}`))
	}))
	defer srv.Close()

	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.CBAddress = ""
	hp, err := bundle.EmitHub(bundle.HubBundle{
		ChainID: 1337, HubRPC: "http://hub:8545", HubWS: "ws://hub:8546", HubGateway: srv.URL,
		Contracts: map[string]string{
			"identityRegistry": "0xh1", "fxAgreement": "0xh4",
			"pairRegistry": "0xh5", "currencyRegistry": "0xh6", "manualOracle": "0xh7",
		},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.HubBundlePath = hp

	if err := findStep(FoundSpokeSteps(cfg), "register-cb").Run(context.Background()); err != nil {
		t.Fatalf("register-cb: %v", err)
	}

	_, wantAddr := deriveCBHubKey(cfg.SpokeID)
	if !strings.Contains(gotBody, wantAddr) {
		t.Fatalf("register-cb payload does not carry the derived hub identity %s: %s", wantAddr, gotBody)
	}
	if strings.Contains(gotBody, devDeployerAddr) {
		t.Fatalf("register-cb must not register the founder address: %s", gotBody)
	}
}

// An operator-pinned -cb-address still wins, so an externally custodied CB key remains usable.
func TestCBHubAddressHonoursOperatorOverride(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})
	cfg.CBAddress = "0xABCDEF0000000000000000000000000000000001"
	if got := cfg.CBHubAddress(); got != cfg.CBAddress {
		t.Fatalf("CBHubAddress() = %q, want the operator override %q", got, cfg.CBAddress)
	}
}

// The SPOKE tokens' CENTRAL_BANK_ROLE must go to the address that signs spoke transactions
// (the deployer, used by the CB's relayer and gateway) — not to the hub identity, which would
// leave the relayer unable to mint or burn tCeBM on its own spoke.
func TestDeploySpokeGrantsCentralBankRoleToTheSpokeSigner(t *testing.T) {
	fake := &exec.FakeRunner{}
	cfg := testSpokeCfg(t, fake)
	cfg.CBAddress = ""

	if err := findStep(FoundSpokeSteps(cfg), "deploy-spoke-contracts").Run(context.Background()); err != nil {
		t.Fatalf("deploy-spoke-contracts: %v", err)
	}

	cmd := strings.Join(fake.Calls[len(fake.Calls)-1].Args, " ")
	if !strings.Contains(cmd, "CENTRAL_BANK_ADDRESS="+devDeployerAddr) {
		t.Fatalf("spoke deploy must grant CENTRAL_BANK_ROLE to the spoke signer %s: %s", devDeployerAddr, cmd)
	}
	if _, hubAddr := deriveCBHubKey(cfg.SpokeID); strings.Contains(cmd, hubAddr) {
		t.Fatalf("spoke deploy must not use the hub identity as central bank: %s", cmd)
	}
}

// envMap turns the "K=V" ComposeEnv slice into a lookup table.
func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.Index(kv, "="); i > 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	return out
}
