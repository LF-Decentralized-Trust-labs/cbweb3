package orchestrator

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"sync"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/keyprovider"
	"github.com/ethereum/go-ethereum/crypto"
)

// itoa is a short alias for strconv.Itoa, used to render port numbers into the
// compose env files.
func itoa(n int) string { return strconv.Itoa(n) }

// Well-known Hyperledger Besu development accounts, pre-funded in the zero-gas
// genesis (see qbftConfig alloc). Used ONLY for `environment: local` contract
// deployment/signing; production keys come from the KeyProvider (deferred). These
// are public test keys, not secrets.
const (
	// 0xfe3b557e8fb62b89f4916b721be55ceb828dbd73 — deployer / admin / CB-A.
	devDeployerKey  = "0x8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63"
	devDeployerAddr = "0xfe3b557e8fb62b89f4916b721be55ceb828dbd73"
	// 0xf17f52151ebef6c7334fad080c5704d77216b732 — CB-B (second sovereign).
	devCBBAddr = "0xf17f52151ebef6c7334fad080c5704d77216b732"

	// bankKeyDerivationSalt namespaces the per-bank onboarding key derivation so a
	// bank code can never collide with another derivation domain.
	bankKeyDerivationSalt = "cbweb3-scenario-b-bank-key:"

	// cbHubKeyDerivationSalt namespaces the per-CB HUB key derivation. A distinct
	// domain from the bank salt so a spoke id and a bank code can never derive the
	// same key.
	cbHubKeyDerivationSalt = "cbweb3-scenario-b-cb-hub-key:"

	// cbRelayerKeyDerivationSalt namespaces the CB's RELAYER hub key — a second
	// identity for the same central bank, in its own derivation domain.
	cbRelayerKeyDerivationSalt = "cbweb3-scenario-b-cb-relayer-key:"

	// cbTokenAdminKeyDerivationSalt namespaces the CB's W-token ADMINISTRATION key — a third
	// identity, in its own domain, and the only one never handed to a container.
	cbTokenAdminKeyDerivationSalt = "cbweb3-scenario-b-cb-token-admin-key:"
)

// deriveCBHubKey deterministically derives a per-CB secp256k1 dev key for the HUB
// chain from the spoke id, so each central bank acts on the shared hub under its own
// identity.
//
// Why the hub specifically: every spoke is its own network, so reusing one key across
// spokes is harmless there — the chains never meet. The hub is the one chain all CBs
// share, and using a single key there made every CB's act appear on-chain as the founding
// CB: same msg.sender on LogSwap, same CENTRAL_BANK_ROLE holder on every sovereign
// W-token. Sovereignty was asserted in the topology but not observable on the ledger.
//
// The spoke-side key (CB_PRIVATE_KEY) is intentionally left alone: it is the deployer of
// that spoke's own contracts, and rotating it would strand every role granted at deploy
// time.
//
// Deterministic and stateless: a re-apply must derive the same address, or the hub-side
// registration and role grants of the previous run would be orphaned. Zero-gas hub, so no
// funding is needed. Dev keys, not secrets — production custody is the KeyProvider's job.
func deriveCBHubKey(spokeID string) (privHex, addr string) {
	return deriveKeyFromSalt(cbHubKeyDerivationSalt, spokeID)
}

// deriveCBRelayerKey derives the hub key for a central bank's bridge relayer, distinct from
// the key its api-gateway uses.
//
// Why two keys for one CB: go-ethereum's nonce counter is per process. The gateway and the
// relayer are separate containers, so with one shared key each keeps its own counter and two
// concurrent submissions claim the same nonce — one transaction is then replaced or rejected.
// That is worse than a retry: the relayer persists a burn/mint hash as an intent the moment it
// is broadcast, so a replaced transaction leaves a position waiting on a hash that will never
// be mined, and reconciliation answers "not yet mined" forever.
//
// The two identities are not equivalent. The gateway's is the one IdentityRegistry maps as the
// token's central bank (PairRegistry admits only that address as proposer/confirmer) and it
// holds the token's DEFAULT_ADMIN_ROLE; the relayer's only needs CENTRAL_BANK_ROLE to mint and
// burn, which the gateway grants it at boot.
func deriveCBRelayerKey(spokeID string) (privHex, addr string) {
	return deriveKeyFromSalt(cbRelayerKeyDerivationSalt, spokeID)
}

// deriveCBTokenAdminKey derives the identity that ADMINISTERS this CB's W-token — the holder of
// DEFAULT_ADMIN_ROLE, which decides who may issue the currency.
//
// Why it is separate from the gateway's. The gateway mints on every liquidity provisioning
// (MintAndApproveForAMM), so its key is operational and lives in a long-running container.
// Administration is the authority to grant issuance, and it should be exercised rarely — at
// provisioning, not per payment. Fused into one key, as it was, a compromise of the gateway
// container yields permanent issuance rights: the attacker grants CENTRAL_BANK_ROLE to an address
// of their own, and rotating the gateway key afterwards does not take it away.
//
// This key is deliberately NOT written into any container environment. Only its address goes
// on-chain; the toolkit re-derives the key when an administrative act is needed. That is the
// difference between a structural split and a secrecy boundary — with derived keys it is the former,
// and when production custody lands it becomes the latter without the topology changing.
func deriveCBTokenAdminKey(spokeID string) (privHex, addr string) {
	return deriveKeyFromSalt(cbTokenAdminKeyDerivationSalt, spokeID)
}

// deriveBankKey deterministically derives a per-bank secp256k1 dev key from the
// bank code, mirroring scenario-a where each commercial bank owns its key. The key
// seeds the bank's auth KMS (KMS_SEED_PRIVATE_KEY) so onboarding produces a DISTINCT
// on-chain wallet per bank (the participants table enforces a unique wallet_address).
// No funding is needed: the local genesis is zero-gas. These are deterministic dev
// keys, not secrets. Returns the 0x-prefixed private key hex and the EVM address.
func deriveBankKey(bankCode string) (privHex, addr string) {
	return deriveKeyFromSalt(bankKeyDerivationSalt, bankCode)
}

// deriveKeyFromSalt derives a deterministic secp256k1 key from a namespacing salt and an
// id. The salt keeps derivation domains apart, so the same id in two roles never yields
// the same key. Returns the 0x-prefixed private key hex and the EVM address.
//
// It derives THROUGH the KeyProvider rather than hashing here, so the interface that production
// custody will implement is the one actually exercised on every apply. The addresses are unchanged
// by construction: the provider hashes seed||id and this used to hash salt+id, which are the same
// bytes when the seed IS the salt — hence one provider per derivation domain. Locked by
// TestDerivedHubIdentitiesAreStableAcrossRefactors, because every existing deployment has on-chain
// roles bound to these exact addresses.
//
// The 0x prefix is re-applied here: the provider's exporter returns bare hex, and the compose env
// contract is 0x-prefixed.
func deriveKeyFromSalt(salt, id string) (privHex, addr string) {
	priv, address, err := deriveViaKeyProvider(salt, id)
	if err != nil {
		// Unreachable with the local provider: the URI is a constant built here, and the local
		// implementation derives in memory without failing. Panicking rather than returning an
		// empty key is deliberate — an empty HUB_SIGNER_PRIVATE_KEY would be written into a
		// compose file and surface much later as an unsignable transaction.
		panic(fmt.Sprintf("keyprovider derivation failed for domain %q id %q: %v", salt, id, err))
	}
	return priv, address
}

// localKeyProviderURI addresses the seeded in-memory emulator for one derivation domain. The seed
// is the domain salt, which is what preserves the existing addresses.
func localKeyProviderURI(salt string) string {
	return "kms://local-emulator?seed=" + url.QueryEscape(salt)
}

// deriveViaKeyProvider returns the 0x-prefixed private key hex and EVM address for id within the
// derivation domain named by salt.
//
// Providers are cached per domain: they hold their derived keys in memory, so reusing one keeps a
// re-derivation of the same id free, and a provisioning run touches only a handful of ids.
func deriveViaKeyProvider(salt, id string) (privHex, addr string, err error) {
	kp, err := keyProviderFor(salt)
	if err != nil {
		return "", "", err
	}
	pub, err := kp.GenerateKey(context.Background(), id)
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}
	pubKey, err := crypto.UnmarshalPubkey(pub)
	if err != nil {
		return "", "", fmt.Errorf("unmarshal public key: %w", err)
	}
	// The local provider exports the private key so the backend can keep signing in-process; the
	// production provider deliberately does not implement LocalKeyExporter, and this path is where
	// that difference will surface (the toolkit will then emit addresses only).
	exporter, ok := kp.(keyprovider.LocalKeyExporter)
	if !ok {
		return "", "", fmt.Errorf("provider for domain %q does not export private keys", salt)
	}
	hexKey, err := exporter.ExportPrivateKeyHex(id)
	if err != nil {
		return "", "", fmt.Errorf("export private key: %w", err)
	}
	return "0x" + hexKey, crypto.PubkeyToAddress(*pubKey).Hex(), nil
}

var (
	keyProviderMu      sync.Mutex
	keyProvidersBySalt = map[string]keyprovider.KeyProvider{}
)

func keyProviderFor(salt string) (keyprovider.KeyProvider, error) {
	keyProviderMu.Lock()
	defer keyProviderMu.Unlock()
	if kp, ok := keyProvidersBySalt[salt]; ok {
		return kp, nil
	}
	kp, err := keyprovider.New(localKeyProviderURI(salt))
	if err != nil {
		return nil, fmt.Errorf("build key provider for domain %q: %w", salt, err)
	}
	keyProvidersBySalt[salt] = kp
	return kp, nil
}
