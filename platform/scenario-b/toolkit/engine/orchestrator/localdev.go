package orchestrator

import "strconv"

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
)
