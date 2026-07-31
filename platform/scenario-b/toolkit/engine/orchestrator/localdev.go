package orchestrator

import (
	"encoding/hex"
	"strconv"

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
func deriveKeyFromSalt(salt, id string) (privHex, addr string) {
	seed := crypto.Keccak256([]byte(salt + id))
	for {
		priv, err := crypto.ToECDSA(seed)
		if err == nil {
			return "0x" + hex.EncodeToString(crypto.FromECDSA(priv)),
				crypto.PubkeyToAddress(priv.PublicKey).Hex()
		}
		// A keccak digest is a valid secp256k1 scalar with overwhelming probability;
		// on the vanishingly rare out-of-range value, re-hash to stay deterministic.
		seed = crypto.Keccak256(seed)
	}
}
