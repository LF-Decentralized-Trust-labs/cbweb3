// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// The reporter was switched off in every deploy because it read CB_INTERNAL_API_URL,
// a name set nowhere in the repository — not in a template, not in the toolkit, not
// in deploy-lnet. Nothing caught it, because a wiring error is invisible to a test
// that does not assert WHICH names are consulted. This does.
func TestSettlementReportURL(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	for _, tc := range []struct {
		name string
		vars map[string]string
		want string
	}{
		{
			// The case that was broken: only the toolkit-rendered name is present.
			name: "falls back to the name the toolkit actually renders",
			vars: map[string]string{"CENTRAL_BANK_API_URL": "http://cb:8080"},
			want: "http://cb:8080",
		},
		{
			name: "explicit override wins",
			vars: map[string]string{"CB_INTERNAL_API_URL": "http://override:9090", "CENTRAL_BANK_API_URL": "http://cb:8080"},
			want: "http://override:9090",
		},
		{
			name: "override alone is honoured",
			vars: map[string]string{"CB_INTERNAL_API_URL": "http://override:9090"},
			want: "http://override:9090",
		},
		{
			// A central bank: it receives reports, it does not send them, so an empty
			// result is correct and leaves the reporter unconstructed.
			name: "central bank has neither, reporter stays off",
			vars: map[string]string{},
			want: "",
		},
		{
			name: "blank override does not mask the default",
			vars: map[string]string{"CB_INTERNAL_API_URL": "   ", "CENTRAL_BANK_API_URL": "http://cb:8080"},
			want: "http://cb:8080",
		},
		{
			name: "whitespace is trimmed",
			vars: map[string]string{"CENTRAL_BANK_API_URL": "  http://cb:8080  "},
			want: "http://cb:8080",
		},
	} {
		if got := settlementReportURL(env(tc.vars)); got != tc.want {
			t.Errorf("%s: settlementReportURL = %q, want %q", tc.name, got, tc.want)
		}
	}
}
