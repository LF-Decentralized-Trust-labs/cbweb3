// SPDX-License-Identifier: Apache-2.0

// Guards the one class of value that must not reach a log line: an amount on the
// Zeto path.
//
// The card that prompted this asked to redact "amounts and hash_lock" from backend
// logs. Two of those three targets turned out to be already public by design, so
// redacting them would be theatre rather than privacy:
//
//   - hashLock is emitted in the public on-chain event LogHTLCLocked, and the secret
//     itself in LogHTLCClaimed (contracts/src/interfaces/IHashTimeLockedContract.sol).
//     The Cacti relay depends on reading it from the chain to pair the two legs.
//   - fCeBM and tCeBM are plain ERC20 contracts, so their transfer amounts are on
//     the public ledger already.
//
// What is genuinely private is the Zeto side. The public HTLC event deliberately
// carries zetoLockRef and NOT the amount — the value lives in the private Zeto
// state. So an amount logged next to a Zeto operation is the real leak: it re-exposes
// in plaintext, on stdout, precisely the number the privacy layer exists to hide, and
// pairs it with the counterparty identity on the same line.
//
// This is a source-scanning guard, the same shape as the Dockerfile hardening guards
// in the toolkits. A behavioural test cannot catch it: the logger call compiles and
// runs happily either way, and the damage is only visible in the log stream.
// Scenario B note: this package has no Zeto-path amount logs today — all nine of
// its escrow.go amount lines are on the public ERC20 path (fCeBM/tCeBM to a Besu
// address), and scenario B has no Paladin adapter. The guard is installed here
// anyway, as a tripwire: the card that prompted it asks for the same protection in
// both scenarios, and the moment a private-path log appears here it should fail
// rather than be noticed in a portal.
package server

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// logCallWithAmount matches a structured-log call that passes an "amount" key.
var logCallWithAmount = regexp.MustCompile(`(?s)logger\.(Info|Debug|Warn|Error)\((.{0,400}?)\)\n`)

// zetoMarkers identify a log line on the private path. Kept deliberately literal:
// the point is to catch the handful of call sites that touch Zeto, not to guess.
var zetoMarkers = []string{"Zeto", "zeto", "transferLocked", "tCeBM via"}

func TestNoPrivateAmountInZetoLogs(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var offenders []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		text := string(src)
		for _, m := range logCallWithAmount.FindAllStringSubmatch(text, -1) {
			call := m[0]
			if !strings.Contains(call, `"amount"`) {
				continue
			}
			onZetoPath := false
			for _, marker := range zetoMarkers {
				if strings.Contains(call, marker) {
					onZetoPath = true
					break
				}
			}
			if !onZetoPath {
				continue
			}
			// Line number, for a message the reader can act on.
			idx := strings.Index(text, call)
			line := 1 + strings.Count(text[:idx], "\n")
			offenders = append(offenders, f+":"+itoaLine(line)+" "+firstLine(call))
		}
	}

	if len(offenders) > 0 {
		t.Errorf("a Zeto-path log line carries a plaintext amount — the value the privacy "+
			"layer exists to hide. Log the operation and its identifier, not the number:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

func itoaLine(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// The guard must not fire on the public ERC20 path: those amounts are already on the
// public ledger, and blocking them would be noise dressed as privacy. This test
// documents that boundary so a later "tighten it everywhere" change has to argue with
// something.
func TestGuardIgnoresPublicLedgerAmounts(t *testing.T) {
	t.Parallel()
	sample := "\ts.logger.Info(\"minting fCeBM\", \"to\", record.RequesterBesuAddress, \"amount\", record.Amount)\n"
	for _, marker := range zetoMarkers {
		if strings.Contains(sample, marker) {
			t.Fatalf("the fCeBM sample matched the Zeto marker %q — the classification is wrong", marker)
		}
	}
}
