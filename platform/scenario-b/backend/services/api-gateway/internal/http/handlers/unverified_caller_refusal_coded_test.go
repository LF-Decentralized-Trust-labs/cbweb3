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

	// The api-gateway's HTTP layer, from this package's directory. These two are the
	// whole of it, not a sample: every VerifiedRelayCaller call site in the service lives
	// in one of them (7 of 7 at the time of writing). A package that starts calling it —
	// a router/v2 handler, say — has to be added here, and nothing will say so, which is
	// why the reason is recorded rather than left to be re-derived.
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
// The span searched runs from the check to the end of its enclosing function, not to a
// fixed number of lines ahead. A line count is the failure this guard was written about:
// the first draft used one, and an explanatory comment growing between the check and the
// refusal pushed the refusal out of the window — so the guard stopped seeing the very
// call site it exists for and reported success in silence. Bounding by the function has
// no number to outgrow.
func checkUnverifiedCallerRefusals(t *testing.T, path, src string) {
	t.Helper()
	for _, span := range unverifiedCallerSpans(src) {
		rel := strings.Index(span.body, "StatusUnauthorized")
		if rel < 0 {
			continue
		}
		refusal := span.body[rel:]
		// The response literal ends at its closing "})"; read only that far, so a coded
		// refusal later in the same function cannot vouch for an uncoded one.
		if closing := strings.Index(refusal, "})"); closing >= 0 {
			refusal = refusal[:closing]
		}
		if !strings.Contains(refusal, `"code"`) {
			t.Errorf("%s:%d: a 401 raised because there is no verified caller carries no \"code\".\n"+
				"\tThe bank portal reads the code to tell a trust rejection from an expired session; without one it\n"+
				"\trefreshes, retries and logs the operator out. Use RELAY_CALLER_IDENTITY_REQUIRED, as the middleware does.",
				path, span.line)
		}
	}
}

// callerSpan is one VerifiedRelayCaller check and the rest of the function holding it.
type callerSpan struct {
	line int    // 1-indexed line of the check, for the failure message
	body string // source from the check to the end of its enclosing function
}

// unverifiedCallerSpans slices the file at every VerifiedRelayCaller call site.
//
// The end of a span is the next top-level declaration — a line beginning "func " at
// column 0 — which is where the enclosing function must have closed. That is a cheap
// stand-in for parsing, and it errs the safe way: if it ever over-reads it examines more
// source than needed, never less.
func unverifiedCallerSpans(src string) []callerSpan {
	lines := strings.Split(src, "\n")

	// Byte offset of the start of each line, so a span can be cut from the full source.
	offsets := make([]int, len(lines))
	at := 0
	for i, l := range lines {
		offsets[i] = at
		at += len(l) + 1 // +1 for the newline consumed by Split
	}

	var spans []callerSpan
	for i, line := range lines {
		if !strings.Contains(line, "VerifiedRelayCaller(") {
			continue
		}
		end := len(src)
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "func ") {
				end = offsets[j]
				break
			}
		}
		spans = append(spans, callerSpan{line: i + 1, body: src[offsets[i]:end]})
	}
	return spans
}
