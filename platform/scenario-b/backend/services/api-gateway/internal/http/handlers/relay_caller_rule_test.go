// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// The rule these cases pin down is the difference between authenticated and authorized. The
// signature layer says which peer is calling; before this rule existed the delegated endpoints then
// authorized on the payer_bank_id in the request body, so any authenticated peer could name another
// bank and have the central bank act against that bank's money.
func TestDecideRelayCaller(t *testing.T) {
	cases := []struct {
		name        string
		caller      string
		claimed     string
		wantRefusal bool
		wantStatus  int
		wantCode    string
	}{
		{
			name:    "caller acts for itself",
			caller:  "bank-a",
			claimed: "bank-a",
		},
		{
			// Entity ids are configuration, and comparing them by spelling would make a
			// capitalisation difference in one manifest look like an attack.
			name:    "case and padding are not identity",
			caller:  "Bank-A",
			claimed: "  bank-a ",
		},
		{
			name:        "caller names another bank",
			caller:      "bank-a",
			claimed:     "bank-b",
			wantRefusal: true,
			wantStatus:  fiber.StatusForbidden,
			wantCode:    "RELAY_CALLER_BANK_MISMATCH",
		},
		{
			// The legacy shared secret is identical in every entity, so a request it
			// authenticated has no caller to bind. These routes refuse it rather than fall back
			// to trusting the body.
			name:        "no verified identity",
			caller:      "",
			claimed:     "bank-a",
			wantRefusal: true,
			wantStatus:  fiber.StatusUnauthorized,
			wantCode:    "RELAY_CALLER_IDENTITY_REQUIRED",
		},
		{
			// The handler's own required-field check answers 400 with a better message; refusing
			// here as well would only hide it.
			name:    "no bank named — left to the handler's field validation",
			caller:  "bank-a",
			claimed: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decideRelayCaller(tc.caller, tc.claimed)
			if !tc.wantRefusal {
				if got != nil {
					t.Fatalf("expected the request to proceed, got refusal %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected a refusal for caller=%q claiming %q", tc.caller, tc.claimed)
			}
			if got.Status != tc.wantStatus {
				t.Fatalf("status = %d, want %d", got.Status, tc.wantStatus)
			}
			if got.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q", got.Code, tc.wantCode)
			}
			if got.Message == "" {
				t.Fatal("a refusal must say why: an operator reads this text, not the code")
			}
		})
	}
}

// The mismatch message must name BOTH sides. A bank operator seeing only "forbidden" cannot tell a
// misconfigured RELAY_KEY_ID from a genuinely wrong payer_bank_id, and those have different fixes.
func TestDecideRelayCaller_MismatchNamesBothSides(t *testing.T) {
	d := decideRelayCaller("bank-a", "bank-b")
	if d == nil {
		t.Fatal("expected a refusal")
	}
	for _, want := range []string{"bank-a", "bank-b"} {
		if !strings.Contains(d.Message, want) {
			t.Fatalf("message %q must name %q", d.Message, want)
		}
	}
}
