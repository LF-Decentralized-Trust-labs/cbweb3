// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnverifiedCallerRefusalsCarryACode guards the class, not the one occurrence.
//
// The bank portal decides what a 401 MEANS from its code (services/api/trust-errors.ts): a
// trust rejection is shown as a notice, and anything unclassified is treated as an expired
// session — the interceptor refreshes, retries, gets the same 401 and calls forceLogout().
//
// So a refusal written without a code does not merely read poorly, it ejects the operator.
// ListPositionsForCaller shipped that way and the consequence was a deadlock: the dashboard
// is where login lands and it calls that endpoint, so a bank whose identity the central bank
// could not yet verify was thrown back to the login screen on arrival — and the portal is the
// only intended route to the onboarding that would have made it verifiable.
//
// The scan is deliberately narrow. Plenty of 401s in this service are genuine session
// failures (no cookie, bad token) where logging out is exactly right. The ones this covers
// are the refusals raised BECAUSE there is no verified relay caller: the session is fine, the
// identity is not, and the portal must be able to tell the difference.
func TestUnverifiedCallerRefusalsCarryACode(t *testing.T) {
	t.Parallel()

	// The api-gateway's HTTP layer, from this package's directory.
	roots := []string{".", "../middleware"}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(root, name)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			checkUnverifiedCallerRefusals(t, path, string(src))
		}
	}
}

// checkUnverifiedCallerRefusals reports any StatusUnauthorized response that follows a
// VerifiedRelayCaller check without naming a code.
//
// The refusal literal is read to its closing "})" in the full source rather than inside a
// fixed line window: a window long enough for today's call sites silently stops covering one
// that grows a comment, which is a guard that quietly reports success.
func checkUnverifiedCallerRefusals(t *testing.T, path, src string) {
	t.Helper()
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "VerifiedRelayCaller(") {
			continue
		}
		lookahead := i + 25
		if lookahead > len(lines) {
			lookahead = len(lines)
		}
		rel := strings.Index(strings.Join(lines[i:lookahead], "\n"), "StatusUnauthorized")
		if rel < 0 {
			continue
		}
		// Re-anchor in the full source so the literal can be read past the lookahead.
		abs := strings.Index(src, strings.Join(lines[i:lookahead], "\n")) + rel
		refusal := src[abs:]
		if closing := strings.Index(refusal, "})"); closing >= 0 {
			refusal = refusal[:closing]
		}
		if !strings.Contains(refusal, `"code"`) {
			t.Errorf("%s:%d: a 401 raised because there is no verified caller carries no \"code\".\n"+
				"\tThe bank portal reads the code to tell a trust rejection from an expired session; without one it\n"+
				"\trefreshes, retries and logs the operator out. Use RELAY_CALLER_IDENTITY_REQUIRED, as the middleware does.",
				path, i+1)
		}
	}
}
