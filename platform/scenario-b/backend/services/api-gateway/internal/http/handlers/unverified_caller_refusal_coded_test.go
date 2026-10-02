// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"go/ast"
	"go/parser"
	"go/token"
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
//
// What it does NOT cover, so that nobody reads more assurance into a green run than is here:
//
//   - The relay-authentication refusals in middleware/relay_signature.go and
//     middleware/internal_relay_auth.go. They are a different condition — the request never
//     authenticated at all — so there is no caller-absence branch to anchor on. They are now
//     covered by TestRelayAuthRefusalsCarryACode, which states the invariant over the middleware
//     instead; this guard still does not reach them.
//   - A refusal whose fiber.Map is built somewhere other than the c.JSON(...) call, or whose
//     status is a variable rather than fiber.StatusUnauthorized. decideRelayCaller in
//     relay_caller.go is that shape today, and it does carry a code.
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
			found, err := uncodedCallerRefusals(path, string(src))
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, r := range found {
				t.Errorf("%s:%d (%s): a 401 raised because there is no verified caller carries no \"code\".\n"+
					"\tThe bank portal reads the code to tell a trust rejection from an expired session; without one it\n"+
					"\trefreshes, retries and logs the operator out. Use RELAY_CALLER_IDENTITY_REQUIRED, as the middleware does.",
					path, r.line, r.fn)
			}
		}
	}
}

// callerRefusal is one 401 response literal raised inside a branch taken because the verified
// caller is missing.
type callerRefusal struct {
	line int    // 1-indexed line of the response literal
	fn   string // enclosing function, so the failure names something greppable
}

// uncodedCallerRefusals reports every StatusUnauthorized response literal that is raised
// inside a branch guarded by the absence of the verified caller and that names no code.
//
// It works on the parsed syntax tree, not on proximity in the text. Proximity is the failure
// this guard has already been rewritten for twice: a fixed line window let an explanatory
// comment push the refusal out of sight, and then reading "the first 401 after the check" let
// any earlier 401 in the same function consume the inspection — so an uncoded refusal sitting
// behind a coded one passed in silence, which is the exact regression the guard exists to
// prevent. Anchoring on the branch has no window and no ordering to outgrow: the refusals
// checked are the ones the compiler agrees are reachable only when the caller is unidentified,
// and a genuine session 401 elsewhere in the same function is not one of them.
func uncodedCallerRefusals(path, src string) ([]callerRefusal, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	var out []callerRefusal
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		names := callerBoundIdents(fn.Body)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ifs, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}
			branch := callerAbsenceBranch(ifs, names)
			if branch == nil {
				return true
			}
			for _, lit := range unauthorizedResponses(branch) {
				if !hasMapKey(lit, "code") {
					out = append(out, callerRefusal{
						line: fset.Position(lit.Pos()).Line,
						fn:   fn.Name.Name,
					})
				}
			}
			return true
		})
	}
	return out, nil
}

// callerBoundIdents collects the identifiers this function assigns from VerifiedRelayCaller,
// so that a later `caller == ""` is recognised as a test of the verified identity and not of
// some unrelated string. Wrapping is transparent: strings.TrimSpace(VerifiedRelayCaller(c))
// is how both current call sites write it.
func callerBoundIdents(body *ast.BlockStmt) map[string]bool {
	names := map[string]bool{}
	record := func(lhs, rhs []ast.Expr) {
		if len(lhs) != len(rhs) {
			return // multi-value call; nothing to attribute to one name
		}
		for i, r := range rhs {
			if !callsVerifiedRelayCaller(r) {
				continue
			}
			if id, ok := lhs[i].(*ast.Ident); ok {
				names[id.Name] = true
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			record(s.Lhs, s.Rhs)
		case *ast.ValueSpec:
			lhs := make([]ast.Expr, 0, len(s.Names))
			for _, id := range s.Names {
				lhs = append(lhs, id)
			}
			record(lhs, s.Values)
		}
		return true
	})
	return names
}

// callerAbsenceBranch returns the block a refusal would live in when this if tests whether the
// verified caller is missing — the then-branch for `caller == ""`, the else-branch for the
// inverted `caller != ""` — and nil when the condition is not such a test. `len(caller) == 0`
// is accepted in either position.
func callerAbsenceBranch(ifs *ast.IfStmt, names map[string]bool) *ast.BlockStmt {
	var branch *ast.BlockStmt
	ast.Inspect(ifs.Cond, func(n ast.Node) bool {
		bin, ok := n.(*ast.BinaryExpr)
		if !ok || (bin.Op != token.EQL && bin.Op != token.NEQ) {
			return true
		}
		if !testsCallerEmptiness(bin.X, bin.Y, names) && !testsCallerEmptiness(bin.Y, bin.X, names) {
			return true
		}
		if bin.Op == token.EQL {
			branch = ifs.Body
		} else if els, ok := ifs.Else.(*ast.BlockStmt); ok {
			branch = els
		}
		return false
	})
	return branch
}

// testsCallerEmptiness reports whether `caller` is a caller-derived expression and `empty` is
// the empty value it would be compared against.
func testsCallerEmptiness(caller, empty ast.Expr, names map[string]bool) bool {
	if isEmptyString(empty) && isCallerExpr(caller, names) {
		return true
	}
	// len(caller) == 0
	if lit, ok := empty.(*ast.BasicLit); ok && lit.Kind == token.INT && lit.Value == "0" {
		if call, ok := caller.(*ast.CallExpr); ok && len(call.Args) == 1 {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "len" {
				return isCallerExpr(call.Args[0], names)
			}
		}
	}
	return false
}

func isEmptyString(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && (lit.Value == `""` || lit.Value == "``")
}

// isCallerExpr reports whether the expression carries the verified caller: one of the
// identifiers bound from VerifiedRelayCaller, or a call to it.
func isCallerExpr(e ast.Expr, names map[string]bool) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && names[id.Name] {
			found = true
		}
		return !found
	})
	return found || callsVerifiedRelayCaller(e)
}

func callsVerifiedRelayCaller(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.Ident: // same package: VerifiedRelayCaller(c)
			if f.Name == "VerifiedRelayCaller" {
				found = true
			}
		case *ast.SelectorExpr: // middleware.VerifiedRelayCaller(c)
			if f.Sel.Name == "VerifiedRelayCaller" {
				found = true
			}
		}
		return !found
	})
	return found
}

// unauthorizedResponses returns the response map literals of every
// c.Status(fiber.StatusUnauthorized).JSON(...) in the block. Every 401 in the block is
// returned, not the first: an uncoded refusal must not be excused by a coded sibling.
func unauthorizedResponses(block *ast.BlockStmt) []*ast.CompositeLit {
	var lits []*ast.CompositeLit
	ast.Inspect(block, func(n ast.Node) bool {
		json, ok := n.(*ast.CallExpr)
		if !ok || len(json.Args) != 1 {
			return true
		}
		sel, ok := json.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "JSON" {
			return true
		}
		if !isUnauthorizedStatusCall(sel.X) {
			return true
		}
		if lit, ok := json.Args[0].(*ast.CompositeLit); ok {
			lits = append(lits, lit)
		}
		return true
	})
	return lits
}

// isUnauthorizedStatusCall reports whether the expression is c.Status(fiber.StatusUnauthorized)
// (or the bare StatusUnauthorized, for a caller inside the fiber package's own idiom).
func isUnauthorizedStatusCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Status" {
		return false
	}
	switch a := call.Args[0].(type) {
	case *ast.SelectorExpr:
		return a.Sel.Name == "StatusUnauthorized"
	case *ast.Ident:
		return a.Name == "StatusUnauthorized"
	}
	return false
}

// hasMapKey reports whether the composite literal has the given string key.
func hasMapKey(lit *ast.CompositeLit, key string) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		k, ok := kv.Key.(*ast.BasicLit)
		if !ok || k.Kind != token.STRING {
			continue
		}
		if strings.Trim(k.Value, "`\"") == key {
			return true
		}
	}
	return false
}

// TestUncodedCallerRefusalsDetector is the guard's own self-test, in the spirit of
// tools/check-license-headers.test.sh: a gate that cannot be shown to fail on a bad fixture
// is a gate that reports success without checking. Two of these fixtures are regressions of
// shapes earlier versions of this guard missed or misread.
func TestUncodedCallerRefusalsDetector(t *testing.T) {
	t.Parallel()

	const coded = `package x

func (h *H) List(c *fiber.Ctx) error {
	caller := strings.TrimSpace(middleware.VerifiedRelayCaller(c))
	if caller == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no verified caller",
			"code":  "RELAY_CALLER_IDENTITY_REQUIRED",
		})
	}
	return nil
}
`

	const uncoded = `package x

func (h *H) List(c *fiber.Ctx) error {
	caller := strings.TrimSpace(middleware.VerifiedRelayCaller(c))
	if caller == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no verified caller",
		})
	}
	return nil
}
`

	// Regression: an uncoded refusal preceded by a coded 401 in the same function. The
	// previous guard read only the first StatusUnauthorized after the check and passed.
	const uncodedBehindACoded401 = `package x

func (h *H) List(c *fiber.Ctx) error {
	caller := strings.TrimSpace(middleware.VerifiedRelayCaller(c))
	if !h.tokenValid(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "token expired",
			"code":  "SESSION_EXPIRED",
		})
	}
	if caller == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no verified caller",
		})
	}
	return nil
}
`

	// Regression the other way: a genuine session 401 in a function that merely reads the
	// caller. Logging the operator out here is correct, and the previous guard failed it.
	const sessionRefusalIsNotOurs = `package x

func (h *H) List(c *fiber.Ctx) error {
	log.Printf("caller=%s", middleware.VerifiedRelayCaller(c))
	if c.Cookies("access_token") == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "session expired, please sign in again",
		})
	}
	return nil
}
`

	const invertedTestUncoded = `package x

func (h *H) List(c *fiber.Ctx) error {
	caller := strings.TrimSpace(VerifiedRelayCaller(c))
	if len(caller) != 0 {
		return h.list(c, caller)
	} else {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "no verified caller",
		})
	}
}
`

	cases := []struct {
		name string
		src  string
		want int
	}{
		{"coded refusal passes", coded, 0},
		{"uncoded refusal is reported", uncoded, 1},
		{"uncoded refusal behind a coded 401 is reported", uncodedBehindACoded401, 1},
		{"a session 401 is not this guard's business", sessionRefusalIsNotOurs, 0},
		{"inverted emptiness test, uncoded else branch", invertedTestUncoded, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := uncodedCallerRefusals("fixture.go", tc.src)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("reported %d uncoded refusals, want %d (%+v)", len(got), tc.want, got)
			}
		})
	}
}
