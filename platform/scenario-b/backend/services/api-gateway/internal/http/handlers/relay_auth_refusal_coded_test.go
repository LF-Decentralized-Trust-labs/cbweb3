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

// TestRelayAuthRefusalsCarryACode is the sibling TestUnverifiedCallerRefusalsCarryACode names in
// its exclusion list, and it closes the other half of the same operator-ejection class.
//
// That guard anchors on the branch taken when there is no verified caller: the request
// authenticated, the identity was not one the central bank can act for. These refusals are the
// other condition — the request never authenticated at all — so no caller-absence branch exists to
// anchor on, and the invariant is stated over the middleware instead: a relay-authentication
// middleware refuses no request anonymously.
//
// The consequence is the same one. PaymentProxyHandler.proxy relays the central bank's status and
// body verbatim, so a refusal raised here reaches the bank portal unchanged; trust-errors.ts reads
// the code, and an unclassified 401 is taken for an expired session — refresh, retry, same 401,
// forceLogout(). A bank whose INTERNAL_RELAY_AUTH_SECRET diverged from its central bank's ejected
// the operator to the login screen saying nothing about why, which is the shape of a configuration
// error that has already happened once (the relay template that shipped without the secret).
//
// The scan is over functions rather than files so that a relay-auth middleware added later is
// covered by being named like one, and the run fails if it finds none — a guard that quietly
// inspects nothing is the failure this whole family of tests exists to avoid.
func TestRelayAuthRefusalsCarryACode(t *testing.T) {
	t.Parallel()

	const root = "../middleware"

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}

	inspected := 0
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
		found, seen, err := uncodedRelayAuthRefusals(path, string(src))
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		inspected += seen
		for _, r := range found {
			t.Errorf("%s:%d (%s): a relay-authentication refusal carries no \"code\".\n"+
				"\tThe bank portal reads the code to tell a configuration fault from an expired session;\n"+
				"\twithout one it refreshes, retries and logs the operator out. Use RELAY_AUTH_REQUIRED,\n"+
				"\tRELAY_AUTH_INVALID or RELAY_AUTH_NOT_CONFIGURED — never the caller-identity code, which\n"+
				"\twould send the operator to an onboarding that is not the problem.",
				path, r.line, r.fn)
		}
	}

	if inspected == 0 {
		t.Fatalf("no RequireRelayAuth* middleware found under %s: the guard inspected nothing.\n"+
			"\tIf the middleware was renamed, rename the match here with it rather than leaving a\n"+
			"\tgreen run that checks no code at all.", root)
	}
}

// relayRefusal is one refusal response literal raised by a relay-authentication middleware.
type relayRefusal struct {
	line int    // 1-indexed line of the response literal
	fn   string // enclosing function, so the failure names something greppable
}

// uncodedRelayAuthRefusals reports every refusal response literal in a relay-authentication
// middleware that names no code, and how many such middlewares it inspected.
//
// A relay-authentication middleware is a function named RequireRelayAuth...: RequireRelayAuth
// (secret-only, guarding the hub's spoke routes) and RequireRelayAuthMigrating
// (signature-preferred, guarding /internal/v1) today. Every refusal inside one is in scope, with no
// branch analysis, because that is the whole of what these functions do — they authenticate the
// relay and refuse, and none of their refusals is a session failure where logging the operator out
// would be right.
func uncodedRelayAuthRefusals(path, src string) ([]relayRefusal, int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, 0, err
	}

	var out []relayRefusal
	inspected := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "RequireRelayAuth") {
			continue
		}
		inspected++
		for _, lit := range refusalResponses(fn.Body) {
			if !hasMapKey(lit, "code") {
				out = append(out, relayRefusal{
					line: fset.Position(lit.Pos()).Line,
					fn:   fn.Name.Name,
				})
			}
		}
	}
	return out, inspected, nil
}

// refusalStatuses are the fiber status names these middlewares refuse with.
//
// Named rather than derived: the constants are identifiers at this point, not numbers, so there is
// nothing to compare against 400. An enumeration also keeps the guard from tripping over a future
// 2xx or 3xx response literal, which would not be a refusal at all.
var refusalStatuses = map[string]bool{
	"StatusUnauthorized":       true,
	"StatusForbidden":          true,
	"StatusServiceUnavailable": true,
}

// refusalResponses returns the response map literals of every c.Status(<refusal>).JSON(...) in the
// block. Every one is returned, not the first: an uncoded refusal must not be excused by a coded
// sibling — the regression the caller-identity guard was rewritten for.
func refusalResponses(block *ast.BlockStmt) []*ast.CompositeLit {
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
		if !isRefusalStatusCall(sel.X) {
			return true
		}
		if lit, ok := json.Args[0].(*ast.CompositeLit); ok {
			lits = append(lits, lit)
		}
		return true
	})
	return lits
}

// isRefusalStatusCall reports whether the expression is c.Status(fiber.Status<refusal>).
func isRefusalStatusCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Status" {
		return false
	}
	switch a := call.Args[0].(type) {
	case *ast.SelectorExpr:
		return refusalStatuses[a.Sel.Name]
	case *ast.Ident:
		return refusalStatuses[a.Name]
	}
	return false
}

// TestUncodedRelayAuthRefusalsDetector is the guard's own self-test, in the spirit of
// tools/check-license-headers.test.sh: a gate that cannot be shown to fail on a bad fixture is a
// gate that reports success without checking.
func TestUncodedRelayAuthRefusalsDetector(t *testing.T) {
	t.Parallel()

	const coded = `package middleware

func RequireRelayAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if secret == "" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "relay auth not configured on server",
				"code":  "RELAY_AUTH_NOT_CONFIGURED",
			})
		}
		return c.Next()
	}
}
`

	const uncoded = `package middleware

func RequireRelayAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Get("X-Relay-Auth") == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "X-Relay-Auth header is required",
			})
		}
		return c.Next()
	}
}
`

	// The regression the caller-identity guard was rewritten for, in this shape: a coded refusal
	// earlier in the same function must not excuse an uncoded one after it.
	const uncodedBehindACoded = `package middleware

func RequireRelayAuthMigrating(cfg RelayAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if bad {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "relay signature verification failed",
				"code":  "RELAY_SIGNATURE_INVALID",
			})
		}
		if c.Get("X-Relay-Auth") == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "relay authentication required",
			})
		}
		return c.Next()
	}
}
`

	// A 503 refuses just as anonymously as a 401 and is in scope.
	const uncoded503 = `package middleware

func RequireRelayAuthMigrating(cfg RelayAuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if cfg.LegacySecret == "" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "relay auth not configured on server",
			})
		}
		return c.Next()
	}
}
`

	// Not a relay-auth middleware: a session 401 elsewhere is somebody else's business, and
	// logging the operator out there is exactly right.
	const notRelayAuth = `package middleware

func RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Cookies("access_token") == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "session expired, please sign in again",
			})
		}
		return c.Next()
	}
}
`

	// A success response is not a refusal, even inside a relay-auth middleware.
	const okResponseIgnored = `package middleware

func RequireRelayAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	}
}
`

	cases := []struct {
		name          string
		src           string
		want          int
		wantInspected int
	}{
		{"coded refusal passes", coded, 0, 1},
		{"uncoded refusal is reported", uncoded, 1, 1},
		{"uncoded refusal behind a coded one is reported", uncodedBehindACoded, 1, 1},
		{"an uncoded 503 is reported", uncoded503, 1, 1},
		{"a middleware that is not relay auth is out of scope", notRelayAuth, 0, 0},
		{"a success response is not a refusal", okResponseIgnored, 0, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, inspected, err := uncodedRelayAuthRefusals("fixture.go", tc.src)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("reported %d uncoded refusals, want %d (%+v)", len(got), tc.want, got)
			}
			if inspected != tc.wantInspected {
				t.Errorf("inspected %d middlewares, want %d", inspected, tc.wantInspected)
			}
		})
	}
}
