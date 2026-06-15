// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSwapQuote_TableNameAndExpiry(t *testing.T) {
	q := &SwapQuote{}
	if q.TableName() != "swap_quotes" {
		t.Fatalf("unexpected table name: %s", q.TableName())
	}

	q.ValidUntil = time.Now().Add(-time.Second)
	if !q.IsExpired() {
		t.Fatal("expected expired quote")
	}
	if got := q.TimeRemainingSeconds(); got != 0 {
		t.Fatalf("expected 0 remaining for expired quote, got %v", got)
	}

	q.ValidUntil = time.Now().Add(10 * time.Second)
	if q.IsExpired() {
		t.Fatal("expected non-expired quote")
	}
	if got := q.TimeRemainingSeconds(); got <= 0 {
		t.Fatalf("expected positive remaining seconds, got %v", got)
	}
}

func TestPoolCommit_Methods(t *testing.T) {
	c := &PoolCommit{}
	if c.TableName() != "pool_commits" {
		t.Fatalf("unexpected table name: %s", c.TableName())
	}

	c.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	if !c.IsExpired() {
		t.Fatal("expected expired commit")
	}
	c.ExpiresAt = time.Now().UTC().Add(time.Minute)
	if c.IsExpired() {
		t.Fatal("expected non-expired commit")
	}

	c.Side = CommitSideA
	if c.OppositeSide() != CommitSideB {
		t.Fatalf("expected opposite of A to be B, got %s", c.OppositeSide())
	}
	c.Side = CommitSideB
	if c.OppositeSide() != CommitSideA {
		t.Fatalf("expected opposite of B to be A, got %s", c.OppositeSide())
	}
}

func TestPoolCommitDistribution_MarshalJSON(t *testing.T) {
	d := PoolCommitDistribution{"lp-1": "60.0000", "lp-2": "40.0000"}
	raw, err := d.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var back map[string]string
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if back["lp-1"] != "60.0000" || back["lp-2"] != "40.0000" {
		t.Fatalf("unexpected distribution round-trip: %v", back)
	}
}

func TestSwapRollbackLog_Methods(t *testing.T) {
	r := &SwapRollbackLog{}
	if r.TableName() != "swap_rollback_logs" {
		t.Fatalf("unexpected table name: %s", r.TableName())
	}
	r.RetryCount = 2
	if r.MaxRetriesExceeded() {
		t.Fatal("did not expect max retries at 2")
	}
	r.RetryCount = 3
	if !r.MaxRetriesExceeded() {
		t.Fatal("expected max retries at 3")
	}
}

func TestSwapRateLimitCounter_Methods(t *testing.T) {
	c := &SwapRateLimitCounter{}
	if c.TableName() != "swap_rate_limit_counters" {
		t.Fatalf("unexpected table name: %s", c.TableName())
	}

	cases := []struct {
		window RateLimitWindow
		count  int
		want   bool
	}{
		{RateLimitWindowMinute, 9, false},
		{RateLimitWindowMinute, 10, true},
		{RateLimitWindowHour, 99, false},
		{RateLimitWindowHour, 100, true},
		{RateLimitWindow("unknown"), 9999, false},
	}
	for _, tc := range cases {
		c.WindowType = tc.window
		c.SwapCount = tc.count
		if got := c.IsLimitExceeded(); got != tc.want {
			t.Fatalf("window=%s count=%d: want %v got %v", tc.window, tc.count, tc.want, got)
		}
	}
}

func TestIsAdminRole(t *testing.T) {
	if !IsAdminRole(RoleNOC) {
		t.Fatal("expected NOC to be admin role")
	}
	if !IsAdminRole(RoleSupervisor) {
		t.Fatal("expected Supervisor to be admin role")
	}
	if IsAdminRole("not-a-role") {
		t.Fatal("did not expect arbitrary role to be admin")
	}
}

func TestSwapExecError_Error(t *testing.T) {
	e := &SwapExecError{Code: ErrCodeSlippageLimitExceeded}
	if e.Error() != string(ErrCodeSlippageLimitExceeded) {
		t.Fatalf("unexpected error string: %s", e.Error())
	}
}

func TestMiscTableNames(t *testing.T) {
	if (CrossCurrencySwapOperation{}).TableName() != "cross_currency_swap_operations" &&
		(CrossCurrencySwapOperation{}).TableName() == "" {
		t.Fatalf("cross currency table name empty")
	}
	if (TransferLimit{}).TableName() == "" {
		t.Fatal("transfer limit table name empty")
	}
	if (TransferVolumeLog{}).TableName() == "" {
		t.Fatal("transfer volume log table name empty")
	}
	if (LPFeeEvent{}).TableName() != "lp_fee_events" {
		t.Fatal("lp fee event table name mismatch")
	}
	if (PairProposal{}).TableName() != "pair_proposals" {
		t.Fatal("pair proposal table name mismatch")
	}
}
