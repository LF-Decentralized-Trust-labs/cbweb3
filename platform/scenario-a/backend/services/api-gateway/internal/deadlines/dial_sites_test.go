// SPDX-License-Identifier: Apache-2.0

package deadlines

import (
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
// It exists because the defect it guards was itself an omission of exactly this
// shape. The adapters already bounded the DIAL — `context.WithTimeout` around
// grpc.DialContext — which reads, at a glance, like the calls are bounded too. They
// were not. A new adapter added tomorrow would look equally finished and be equally
// unbounded, and nothing else in the suite would notice.
func TestEveryDialSiteAppliesTheDeadlineBackstop(t *testing.T) {
	// Relative to internal/deadlines/.
	sites := []struct {
		file  string
		value string
		note  string
	}{
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

	for _, s := range sites {
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
		want := "grpc.WithUnaryInterceptor(grpcx.WithDefaultDeadline(" + s.value + "))"
		if !strings.Contains(body, want) {
			t.Errorf("%s (%s): dial does not apply the backstop.\n  want to find: %s",
				s.file, s.note, want)
		}
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
