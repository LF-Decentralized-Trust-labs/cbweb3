// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// artifact paths relative to this package dir (scenario-a/toolkit/engine/orchestrator).
const (
	fiatArtifactRel = "../../../contracts/out/FiatCentralBankMoney.sol/FiatCentralBankMoney.json"
	htlcArtifactRel = "../../../contracts/out/HashTimeLockedContract.sol/HashTimeLockedContract.json"
)

// requireArtifact skips when the Foundry artifact is absent, which is the state of
// any tree where `forge build` has not run — a clean CI runner included.
//
// It exists because these tests are NOT hermetic and were silently relying on a
// developer's local contracts/out. Two of them failed outright the first time they
// ran in CI. The third was worse: TestEncodeDeployData_WrongArgCount asserts that
// packing the wrong constructor arity fails, and a missing file also fails, so it
// passed while testing nothing at all.
//
// Skipping is the honest state, not a fix: with no artifact these three do not run
// in CI. Making them run means building contracts there, which is a separate
// decision about putting Foundry in this workflow.
func requireArtifact(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Clean(rel)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("skipping: Foundry artifact %s is absent — run `make contracts.build` to exercise this test", rel)
	}
	return path
}

func TestEncodeDeployData_Fiat(t *testing.T) {
	op := common.HexToAddress("0xFE3B557E8Fb62b89F4916B721be55cEb828dBd73")
	data, err := encodeDeployData(requireArtifact(t, fiatArtifactRel), "Fiat BRL", "fCeBM_BRL", op, op)
	if err != nil {
		t.Fatalf("encodeDeployData(fiat): %v", err)
	}
	// bytecode is ~4KB; encoded ctor args (2 strings + 2 addresses) add several
	// 32-byte words. A short result means encoding silently produced nothing.
	if len(data) < 4000 {
		t.Errorf("encoded deploy data too short: %d bytes", len(data))
	}
}

func TestEncodeDeployData_HTLC(t *testing.T) {
	reg := common.HexToAddress("0x9ab7CA8a88F8e351f9b0eEEA5777929210199295")
	zero := common.Address{}
	data, err := encodeDeployData(requireArtifact(t, htlcArtifactRel), reg, zero, zero)
	if err != nil {
		t.Fatalf("encodeDeployData(htlc): %v", err)
	}
	if len(data) < 3000 {
		t.Errorf("encoded deploy data too short: %d bytes", len(data))
	}
}

func TestEncodeDeployData_WrongArgCount(t *testing.T) {
	// fCeBM constructor needs 4 args; passing 1 must fail at ABI packing.
	_, err := encodeDeployData(requireArtifact(t, fiatArtifactRel), "only-one")
	if err == nil {
		t.Error("expected error packing wrong constructor arity, got nil")
	}
}

func TestEncodeDeployData_MissingArtifact(t *testing.T) {
	if _, err := encodeDeployData("/nonexistent/Contract.json", common.Address{}); err == nil {
		t.Error("expected error for missing artifact, got nil")
	}
}
