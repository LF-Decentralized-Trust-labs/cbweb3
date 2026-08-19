// SPDX-License-Identifier: Apache-2.0

// Bound the audit-log page size on the governance route (finding R2-M-14).
//
// Two handlers read the same audit log. The supervisor one has always capped its page size
// at maxAuditLimit; the governance one passed the client's `limit` straight through, and the
// compliance repository has a *default* of 50 but no maximum — so `?limit=10000000` asks
// the database for ten million rows and serialises whatever comes back. `page` was
// unvalidated too, and a negative page yields a negative OFFSET.
//
// The cap belongs at the handler because that is where untrusted input arrives, and the two
// handlers reading one log should not disagree about what a legal page is.
package handlers

import (
	"strconv"
	"testing"
)

func TestAuditPageSize_CapsAnOversizedLimit(t *testing.T) {
	got := auditPageSize(strconv.Itoa(10_000_000))
	if got > maxAuditLimit {
		t.Errorf("limit=10000000 produced %d, above the %d cap", got, maxAuditLimit)
	}
}

func TestAuditPageSize_KeepsAReasonableLimit(t *testing.T) {
	if got := auditPageSize("25"); got != 25 {
		t.Errorf("limit=25 produced %d, want 25 — a legal page size must survive", got)
	}
}

func TestAuditPageSize_FallsBackOnGarbageAndZero(t *testing.T) {
	for _, in := range []string{"", "abc", "0", "-5"} {
		got := auditPageSize(in)
		if got < 1 || got > maxAuditLimit {
			t.Errorf("limit=%q produced %d, outside 1..%d", in, got, maxAuditLimit)
		}
	}
}

// A negative page becomes a negative OFFSET downstream; floor it at the first page.
func TestAuditPageNumber_FloorsAtOne(t *testing.T) {
	for _, in := range []string{"", "abc", "0", "-3"} {
		if got := auditPageNumber(in); got != 1 {
			t.Errorf("page=%q produced %d, want 1", in, got)
		}
	}
	if got := auditPageNumber("7"); got != 7 {
		t.Errorf("page=7 produced %d, want 7", got)
	}
}
