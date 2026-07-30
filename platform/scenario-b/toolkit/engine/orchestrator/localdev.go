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
)

// deriveBankKey deterministically derives a per-bank secp256k1 dev key from the
// bank code, mirroring scenario-a where each commercial bank owns its key. The key
// seeds the bank's auth KMS (KMS_SEED_PRIVATE_KEY) so onboarding produces a DISTINCT
// on-chain wallet per bank (the participants table enforces a unique wallet_address).
// No funding is needed: the local genesis is zero-gas. These are deterministic dev
// keys, not secrets. Returns the 0x-prefixed private key hex and the EVM address.
func deriveBankKey(bankCode string) (privHex, addr string) {
	seed := crypto.Keccak256([]byte(bankKeyDerivationSalt + bankCode))
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
