// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

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

// --- derivation is load-bearing: lock the actual addresses ---
//
// The tests above prove the derivation is deterministic and that the domains are distinct. Neither
// notices if the derivation CHANGES: every existing deployment has on-chain state bound to these
// exact addresses — IdentityRegistry maps them as the token's central bank, PairRegistry admits only
// them as proposer/confirmer, and the W-token's roles are granted to them. A silent change orphans
// all of it and the failure appears as "not the central bank of tokenB" long after the fact.
//
// So these literals are a REGRESSION LOCK, not a specification of the algorithm. They were read off
// a live stack (verify-sovereign-hub-identity.sh, step 15). If a change to the derivation is
// intended, the on-chain handover has to be planned and these values updated deliberately.
func TestDerivedHubIdentitiesAreStableAcrossRefactors(t *testing.T) {
	cases := []struct {
		spokeID     string
		gatewayAddr string
		relayerAddr string
	}{
		{"spoke-brl", "0xaE3467da888C4Af6F171993Fe9D183033FB71E74", "0x9f4fafEEF19E4Dd2472E13d0E1869f9f228B14e2"},
		{"spoke-ars", "0x383DBDbB1F9Eb49Bc54FF3b8FE38BE1E6580b370", "0x5A6c18C02bE57b819Be807c700eAbC6ee035103b"},
	}
	for _, tc := range cases {
		t.Run(tc.spokeID, func(t *testing.T) {
			if _, addr := deriveCBHubKey(tc.spokeID); addr != tc.gatewayAddr {
				t.Fatalf("gateway identity for %s changed to %s (was %s) — on-chain roles are bound to the old address",
					tc.spokeID, addr, tc.gatewayAddr)
			}
			if _, addr := deriveCBRelayerKey(tc.spokeID); addr != tc.relayerAddr {
				t.Fatalf("relayer identity for %s changed to %s (was %s) — its CENTRAL_BANK_ROLE grant is bound to the old address",
					tc.spokeID, addr, tc.relayerAddr)
			}
		})
	}
}

// The private key format is part of the contract with the compose env: the backend receives this
// string verbatim. keyprovider's exporter returns hex WITHOUT the 0x prefix, so routing the
// derivation through it must not drop the prefix.
func TestDerivedKeysKeepTheEnvHexFormat(t *testing.T) {
	for name, key := range map[string]string{
		"cb hub":     first(deriveCBHubKey("spoke-brl")),
		"cb relayer": first(deriveCBRelayerKey("spoke-brl")),
		"bank":       first(deriveBankKey("bank-itau")),
	} {
		if !strings.HasPrefix(key, "0x") || len(key) != 66 {
			t.Fatalf("%s key %q must be 0x-prefixed and 66 chars", name, key)
		}
	}
}

func first(a, _ string) string { return a }

// Equivalence with the raw derivation, for every domain. This is what makes routing through the
// KeyProvider a refactor rather than a change: the provider hashes seed||id and the inline path
// hashed salt+id, which are the same bytes when the seed IS the salt.
func TestDerivationMatchesTheRawKeccakDomain(t *testing.T) {
	cases := []struct {
		name string
		salt string
		id   string
		got  func() (string, string)
	}{
		{"cb hub", cbHubKeyDerivationSalt, "spoke-brl", func() (string, string) { return deriveCBHubKey("spoke-brl") }},
		{"cb relayer", cbRelayerKeyDerivationSalt, "spoke-brl", func() (string, string) { return deriveCBRelayerKey("spoke-brl") }},
		{"bank", bankKeyDerivationSalt, "bank-itau", func() (string, string) { return deriveBankKey("bank-itau") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			material := crypto.Keccak256([]byte(tc.salt + tc.id))
			want, err := crypto.ToECDSA(material)
			if err != nil {
				t.Skipf("fixture salt+id produced an out-of-range scalar; not expected for these inputs")
			}
			wantAddr := crypto.PubkeyToAddress(want.PublicKey).Hex()
			wantHex := "0x" + hex.EncodeToString(crypto.FromECDSA(want))

			gotHex, gotAddr := tc.got()
			if gotAddr != wantAddr || gotHex != wantHex {
				t.Fatalf("derivation diverged from keccak256(salt+id): got %s / %s, want %s / %s",
					gotAddr, gotHex, wantAddr, wantHex)
			}
		})
	}
}

// --- token administration as a separate identity ---
//
// The gateway key holds BOTH CENTRAL_BANK_ROLE (mint/burn, used on every liquidity provisioning)
// and DEFAULT_ADMIN_ROLE (decides who may issue). Those are different kinds of authority: issuance
// is operational, administration should be rare. Fused, a compromise of the gateway container grants
// permanent issuance rights that survive rotating the gateway key.
//
// The administration identity is deliberately NOT handed to any container: only its address goes
// on-chain, and the toolkit re-derives the key when an administrative act is needed.
func TestDeriveCBTokenAdminKey_DistinctFromEveryOtherDomain(t *testing.T) {
	const spokeID = "spoke-brl"
	_, admin := deriveCBTokenAdminKey(spokeID)
	_, gateway := deriveCBHubKey(spokeID)
	_, relayer := deriveCBRelayerKey(spokeID)
	_, bank := deriveBankKey(spokeID) // the same id in the bank domain

	if admin == "" {
		t.Fatal("admin identity must be derived")
	}
	for name, other := range map[string]string{"gateway": gateway, "relayer": relayer, "bank domain": bank} {
		if admin == other {
			t.Fatalf("admin identity collides with the %s identity (%s) — the split would be nominal", name, other)
		}
	}
}

func TestDeriveCBTokenAdminKey_DeterministicAndPerSpoke(t *testing.T) {
	a1, addr1 := deriveCBTokenAdminKey("spoke-brl")
	a2, addr2 := deriveCBTokenAdminKey("spoke-brl")
	if a1 != a2 || addr1 != addr2 {
		t.Fatal("derivation must be deterministic: a re-apply would otherwise strand the previous administrator")
	}
	_, other := deriveCBTokenAdminKey("spoke-ars")
	if addr1 == other {
		t.Fatalf("two spokes derived the same administrator (%s) — administration would not be sovereign", other)
	}
}

// The administration key must never reach a container: that is the whole point of separating it.
// Compose env is the only channel the toolkit has, so its absence there is the invariant.
func TestComposeEnvNeverCarriesTheTokenAdminKey(t *testing.T) {
	c := SpokeConfig{SpokeID: "spoke-brl", Currency: "BRL", RPCPort: 33645, VolumePrefix: "cb"}
	adminKey, _ := deriveCBTokenAdminKey(c.SpokeID)

	// ComposeEnv is a []string of "KEY=VALUE", so the check is on the whole entry: a substring
	// match also catches the key being embedded in a larger value.
	for _, entry := range c.ComposeEnv() {
		if strings.Contains(entry, adminKey) || strings.Contains(entry, strings.TrimPrefix(adminKey, "0x")) {
			name := entry
			if i := strings.Index(entry, "="); i > 0 {
				name = entry[:i]
			}
			t.Fatalf("compose env %s carries the token administration PRIVATE KEY — it must stay with the toolkit", name)
		}
	}
}

// --- relay key id ---
//
// A central bank's ENTITY is its ROLE ("central-bank"), so every CB in the topology carries the same
// BANK_CODE. That cannot serve as the service-to-service authentication id: the receiver pins one
// public key per id, so two CBs sharing it means only one of them can ever authenticate — and
// "signed by central-bank" would not say WHICH central bank, which is the shared-secret problem
// again with asymmetric keys on top.
//
// The id therefore comes from the manifest name, which is unique by construction (it is what the
// container prefix is built from). BANK_CODE is deliberately left alone: it flows into owner_bank_id
// on bridge positions and into the reconciliation's self-exclusion.
func TestComposeEnvCarriesAUniqueRelayKeyID(t *testing.T) {
	c := SpokeConfig{
		SpokeID: "spoke-brl", Currency: "BRL", RPCPort: 33645,
		VolumePrefix: "cb", Entity: "central-bank", RelayKeyID: "central-bank-brazil",
	}
	env := map[string]string{}
	for _, e := range c.ComposeEnv() {
		if i := strings.Index(e, "="); i > 0 {
			env[e[:i]] = e[i+1:]
		}
	}
	if env["RELAY_KEY_ID"] != "central-bank-brazil" {
		t.Fatalf("RELAY_KEY_ID = %q, want central-bank-brazil", env["RELAY_KEY_ID"])
	}
	if env["RELAY_KEY_ID"] == env["BANK_CODE"] {
		t.Fatalf("the relay id must differ from BANK_CODE (%q) — BANK_CODE is the role and collides across CBs", env["BANK_CODE"])
	}
	if env["RELAY_KEY_ID"] == env["ENTITY"] {
		t.Fatalf("the relay id must differ from ENTITY (%q), which is the role", env["ENTITY"])
	}
}

// Two central banks must never derive the same relay id, or the receiver can pin only one key.
func TestRelayKeyIDDiffersBetweenCentralBanks(t *testing.T) {
	br := SpokeConfig{SpokeID: "spoke-brl", RPCPort: 33645, RelayKeyID: "central-bank-brazil"}
	ar := SpokeConfig{SpokeID: "spoke-ars", RPCPort: 33745, RelayKeyID: "central-bank-argentina"}
	if br.relayKeyID() == ar.relayKeyID() {
		t.Fatalf("both central banks resolved the relay id %q", br.relayKeyID())
	}
}

// Without an explicit value the spoke id is the fallback: still unique per CB, and available
// without threading the manifest name through every construction path.
func TestRelayKeyIDFallsBackToTheSpokeID(t *testing.T) {
	c := SpokeConfig{SpokeID: "spoke-brl", RPCPort: 33645}
	if got := c.relayKeyID(); got != "spoke-brl" {
		t.Fatalf("fallback relay id = %q, want spoke-brl", got)
	}
}

// Enforcement is a RECEIVER-side setting and a commercial bank hosts no internal relay routes, so it
// must never inherit the flag. If it did, the bank would refuse to start: its PKI dir holds only its
// own key and its -participant certificate (which the pin loader skips by design), leaving an empty
// registry — and empty plus enforcement is exactly the combination the gateway refuses. An operator
// enabling enforcement for the central banks must not take every bank down with it.
func TestJoinComposeEnvNeverInheritsEnforcement(t *testing.T) {
	t.Setenv("RELAY_REQUIRE_SIGNATURE", "true")
	c := JoinConfig{BankID: "bank-itau", SpokeID: "spoke-brl", RPCPort: 33646, VolumePrefix: "bank"}
	for _, e := range c.ComposeEnv() {
		if strings.HasPrefix(e, "RELAY_REQUIRE_SIGNATURE=") {
			if e != "RELAY_REQUIRE_SIGNATURE=" {
				t.Fatalf("a bank inherited enforcement (%q) — its registry is empty, so it would refuse to start", e)
			}
			return
		}
	}
	t.Fatal("RELAY_REQUIRE_SIGNATURE is not set at all on a bank — it would inherit the operator's export")
}
