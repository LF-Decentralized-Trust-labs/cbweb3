// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/addrs"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// --- a 2xx the step cannot use is not a success ---
//
// register-currency exists to obtain the sovereign W-token address and record it as
// W_TOKEN_ADDRESS. It used to swallow a reply it could not read:
//
//	if json.Unmarshal(body, &out) == nil && out.TokenAddress != "" { ...record... }
//	return nil
//
// A hub answering 2xx without token_address therefore left the step reporting success with nothing
// recorded, and the failure surfaced one step later as
//
//	separate-token-admin: W_TOKEN_ADDRESS not found in ... — register-currency must run first
//
// which is false: register-currency did run, and passed.
//
// Worse, it was not recoverable by re-running. register-currency has no Check, and the engine skips
// a step whose durable state says done (orchestrator.go: `o.state.Get(st.Name) == StatusDone`). So
// every subsequent apply skipped the step that had failed to do its job and failed again at the same
// place, until someone hand-edited .provisioning-state.yaml. The card called this "re-runnable"; it
// was not.

func hubReplying(t *testing.T, status int, body string) (cfg SpokeConfig, calls *int) {
	t.Helper()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "register-currency") {
			n++
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	cfg = testSpokeCfg(t, &exec.FakeRunner{})
	hp, err := bundle.EmitHub(bundle.HubBundle{
		ChainID: 1337, HubRPC: "http://hub:8545", HubWS: "ws://hub:8546", HubGateway: srv.URL,
		Contracts: map[string]string{
			"identityRegistry": "0xh1", "fxAgreement": "0xh4",
			"pairRegistry": "0xh5", "currencyRegistry": "0xh6", "manualOracle": "0xh7",
		},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.HubBundlePath = hp
	return cfg, &n
}

func TestRegisterCurrencyRefusesA2xxWithoutTokenAddress(t *testing.T) {
	cfg, _ := hubReplying(t, http.StatusOK, `{"already_registered":true}`)

	err := findStep(FoundSpokeSteps(cfg), "register-currency").Run(context.Background())
	if err == nil {
		t.Fatal("a 2xx carrying no token_address must fail the step: it is the step's only output, and reporting success strands every later step that needs it")
	}
	// The message has to name what the hub actually said, or an operator cannot tell this from a
	// network failure or a rejected registration.
	if !strings.Contains(err.Error(), "token_address") {
		t.Fatalf("error does not name the missing field: %v", err)
	}
	if !strings.Contains(err.Error(), "already_registered") {
		t.Fatalf("error does not quote the hub's reply, so the operator cannot see what came back: %v", err)
	}
	if got := addrs.ReadAddr(cfg.SpokeEnvFile, "W_TOKEN_ADDRESS"); got != "" {
		t.Fatalf("W_TOKEN_ADDRESS = %q, want it unwritten on a failed step", got)
	}
}

func TestRegisterCurrencyRefusesAnUnreadableBody(t *testing.T) {
	cfg, _ := hubReplying(t, http.StatusOK, `not json at all`)

	err := findStep(FoundSpokeSteps(cfg), "register-currency").Run(context.Background())
	if err == nil {
		t.Fatal("a 2xx whose body cannot be parsed must fail: the step has no address to record")
	}
	if !strings.Contains(err.Error(), "not json at all") {
		t.Fatalf("error does not quote the unreadable body: %v", err)
	}
}

func TestRegisterCurrencyRecordsTheTokenAddress(t *testing.T) {
	cfg, _ := hubReplying(t, http.StatusOK, `{"token_address":"0xWTOKEN"}`)

	if err := findStep(FoundSpokeSteps(cfg), "register-currency").Run(context.Background()); err != nil {
		t.Fatalf("register-currency: %v", err)
	}
	if got := addrs.ReadAddr(cfg.SpokeEnvFile, "W_TOKEN_ADDRESS"); got != "0xWTOKEN" {
		t.Fatalf("W_TOKEN_ADDRESS = %q, want 0xWTOKEN", got)
	}
}

// The recoverability half: idempotency by observation, not by durable state. With a Check, a re-run
// retries the registration when the address is absent instead of skipping it because a previous run
// was recorded done.
func TestRegisterCurrencyIsIdempotentByReadingTheEnvFile(t *testing.T) {
	cfg, calls := hubReplying(t, http.StatusOK, `{"token_address":"0xWTOKEN"}`)
	step := findStep(FoundSpokeSteps(cfg), "register-currency")

	if step.Check == nil {
		t.Fatal("register-currency needs a Check: without one the engine skips it on the strength of durable state alone, so a run that recorded done without recording the address can never retry")
	}

	satisfied, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if satisfied {
		t.Fatal("Check must report unsatisfied while W_TOKEN_ADDRESS is absent")
	}

	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("register-currency: %v", err)
	}

	satisfied, err = step.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !satisfied {
		t.Fatal("Check must report satisfied once the address is recorded, so a re-run does not register the currency twice")
	}
	if *calls != 1 {
		t.Fatalf("hub was called %d times, want 1", *calls)
	}
}

// The guard downstream must not accuse a step that ran and passed.
func TestSeparateTokenAdminDoesNotBlameRegisterCurrency(t *testing.T) {
	cfg := testSpokeCfg(t, &exec.FakeRunner{})

	err := findStep(FoundSpokeSteps(cfg), "separate-token-admin").Run(context.Background())
	if err == nil {
		t.Fatal("separate-token-admin must refuse to run without W_TOKEN_ADDRESS")
	}
	if strings.Contains(err.Error(), "must run first") {
		t.Fatalf("the message tells the operator to run a step that may already have run and passed: %v", err)
	}
	if !strings.Contains(err.Error(), "W_TOKEN_ADDRESS") {
		t.Fatalf("the message must name what is missing: %v", err)
	}
}

// truncateForLog carries two properties the error messages above depend on, and neither is
// exercised by asserting on those messages: a body short enough to quote whole passes through, so
// the assertions elsewhere stay meaningful, and the two guards hold on a body that is neither.
//
// The CR/LF half is not cosmetic. The quoted body comes from the hub, so a reply carrying newlines
// would let a misbehaving or hostile one write extra lines into an error an operator reads as a
// single statement — a forged log entry inside a message this step produces. Stripping them is what
// prevents that, and nothing else here would notice if it went away.
func TestTruncateForLog(t *testing.T) {
	const max = 200

	t.Run("an empty body says so instead of quoting nothing", func(t *testing.T) {
		for _, in := range []string{"", "   ", "\n\n", " \r\n "} {
			if got := truncateForLog([]byte(in)); got != "(empty body)" {
				t.Errorf("truncateForLog(%q) = %q, want %q", in, got, "(empty body)")
			}
		}
	})

	t.Run("a short body is quoted unchanged", func(t *testing.T) {
		const body = `{"error":"currency already registered"}`
		if got := truncateForLog([]byte(body)); got != body {
			t.Errorf("truncateForLog(%q) = %q, want it unchanged", body, got)
		}
	})

	t.Run("newlines cannot survive into the error", func(t *testing.T) {
		// A hub reply shaped to look like two more log lines once quoted.
		body := "{\"error\":\"nope\"}\r\nINFO  registration succeeded\nINFO  token_address=0xdeadbeef"
		got := truncateForLog([]byte(body))
		if strings.ContainsAny(got, "\r\n") {
			t.Fatalf("the quoted body kept a line break, so it can forge a log entry: %q", got)
		}
		// The content is still there — the point is that it cannot break the line, not that it
		// is hidden.
		if !strings.Contains(got, "registration succeeded") {
			t.Errorf("the body was mangled beyond recognition: %q", got)
		}
	})

	t.Run("a long body is bounded", func(t *testing.T) {
		body := strings.Repeat("A", max*3)
		got := truncateForLog([]byte(body))
		if !strings.HasPrefix(got, strings.Repeat("A", max)) {
			t.Errorf("the first %d characters were not kept: %q", max, got)
		}
		if !strings.HasSuffix(got, "… (truncated)") {
			t.Errorf("a truncated body does not say so: %q", got)
		}
		// Bounded, not merely marked: an unbounded body reaches logs and terminals whether or
		// not it carries a suffix.
		if len(got) > max+len("… (truncated)") {
			t.Errorf("output is %d bytes, which is not bounded by the %d cap", len(got), max)
		}
	})
}
