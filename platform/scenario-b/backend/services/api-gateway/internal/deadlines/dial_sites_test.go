// SPDX-License-Identifier: Apache-2.0

package deadlines

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Every outbound gRPC connection this gateway opens must carry the deadline
// backstop. This is a source-level tripwire rather than a behavioural test because
// what it guards is a WIRING omission: the interceptor itself is covered by unit
// tests in shared/proto/grpcx, and no unit test can observe whether a given
// grpc.DialContext call was given it.
//
// The assertion is on WithChainUnaryInterceptor, not WithUnaryInterceptor, and that
// is load-bearing: WithUnaryInterceptor ASSIGNS (o.unaryInt = f), so a second one
// added at the same dial silently replaces the first, while the chain variant appends.
// A backstop that a later, unrelated interceptor can quietly drop is not a backstop.
//
// It exists because the defect it guards was itself an omission of exactly this
// shape. The adapters already bounded the DIAL — `context.WithTimeout` around
// grpc.DialContext — which reads, at a glance, like the calls are bounded too. They
// were not. A new adapter added tomorrow would look equally finished and be equally
// unbounded, and nothing else in the suite would notice.
type dialSite struct {
	file  string
	value string
	note  string
}

// dialSites maps each known gRPC dial to the constant it must apply. Paths are
// relative to internal/deadlines/. TestNoDialSiteEscapesThisList checks that this
// list is complete, so adding an adapter cannot quietly opt out of the backstop.
func dialSites() []dialSite {
	return []dialSite{
		{"../app/app.go", "deadlines.Auth",
			"the shared auth+identity connection — the one production actually dials"},
		{"../adapters/compliance/compliance_grpc.go", "deadlines.Compliance",
			"compliance, whose on-chain writes take seconds"},
		{"../adapters/payment/payment_grpc.go", "deadlines.Payment",
			"payment, which carries the cross-currency swap path"},
		{"../adapters/identity/identity_grpc_manager.go", "deadlines.Auth",
			"identity adapter's own dial: no production caller today, but exported"},
		{"../adapters/auth/identity_grpc_provider.go", "deadlines.Auth",
			"auth adapter's own dial: test-only caller today, but exported"},
	}
}

func TestEveryDialSiteAppliesTheDeadlineBackstop(t *testing.T) {
	for _, s := range dialSites() {
		src, err := os.ReadFile(filepath.Clean(s.file))
		if err != nil {
			t.Errorf("%s: cannot read (%s): %v", s.file, s.note, err)
			continue
		}
		body := string(src)
		if !strings.Contains(body, "grpc.DialContext(") {
			// The file stopped dialling — fine, but then it should not be listed here.
			t.Errorf("%s: no grpc.DialContext found; drop it from this list or restore the dial", s.file)
			continue
		}
		want := "grpc.WithChainUnaryInterceptor(grpcx.WithDefaultDeadline(" + s.value + "))"
		if !strings.Contains(body, want) {
			t.Errorf("%s (%s): dial does not apply the backstop.\n  want to find: %s",
				s.file, s.note, want)
		}
	}
}

// No dial site may escape the list above.
//
// The list on its own has the very weakness the interceptor was chosen to avoid: it
// bounds today's dial sites and silently misses the next one. WithDefaultDeadline
// covers methods that do not exist yet; nothing covered ADAPTERS that do not exist
// yet, so a new one could open an unbounded connection and the suite would stay
// green — the omission this whole change exists to prevent, one level up.
//
// So the sites are discovered rather than enumerated: every non-test .go file under
// internal/ that opens a gRPC connection must appear in dialSites(), which then
// forces it through the check above.
func TestNoDialSiteEscapesThisList(t *testing.T) {
	known := map[string]bool{}
	for _, s := range dialSites() {
		known[filepath.ToSlash(filepath.Clean(s.file))] = true
	}

	found := 0
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Vendored and generated trees are third-party dials, not this gateway's.
			switch d.Name() {
			case "vendor", "testdata":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		body := string(src)
		if !strings.Contains(body, "grpc.DialContext(") && !strings.Contains(body, "grpc.NewClient(") {
			return nil
		}
		found++
		rel := filepath.ToSlash(filepath.Clean(path))
		if !known[rel] {
			t.Errorf("%s opens a gRPC connection but is not in dialSites().\n"+
				"  Add it with the constant its peer needs, so the backstop check covers it.", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal/: %v", err)
	}

	// A walk that finds nothing would pass vacuously — the failure mode that let the
	// licence gate report success without checking anything. The list is non-empty, so
	// the discovery must at least rediscover it.
	if found < len(known) {
		t.Errorf("discovered %d dial site(s) but dialSites() lists %d: the walk is not reaching them",
			found, len(known))
	}
}

// The values are the product of a measurement recorded in the package doc. This
// pins the ORDERING that measurement established, so a later edit cannot quietly
// invert it — a payment bound tighter than compliance's would cut the swap path
// the loose payment value exists to protect.
func TestMeasuredValuesKeepTheirOrdering(t *testing.T) {
	if !(Auth < Compliance && Compliance < Payment) {
		t.Errorf("ordering lost: Auth=%v Compliance=%v Payment=%v; want Auth < Compliance < Payment",
			Auth, Compliance, Payment)
	}
	// Floors, not targets: each must stay clear of the slowest call MEASURED on that
	// client, so a well-meaning tightening cannot silently cross it.
	for _, c := range []struct {
		name    string
		got     time.Duration
		atLeast time.Duration
		why     string
	}{
		{"Auth", Auth, time.Second, "login measured 75ms end-to-end"},
		{"Compliance", Compliance, 20 * time.Second, "RegisterCurrencyOnChain measured 16.2s"},
		{"Payment", Payment, 180 * time.Second, "the tryout documents a swap taking up to 180s"},
	} {
		if c.got < c.atLeast {
			t.Errorf("%s=%v is below its floor %v (%s)", c.name, c.got, c.atLeast, c.why)
		}
	}
}
