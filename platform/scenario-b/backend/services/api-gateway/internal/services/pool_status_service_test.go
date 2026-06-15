// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"testing"
	"time"
)

type stubReader struct {
	a, b  string
	ratio float64
}

func (s stubReader) GetPoolReserves(_ context.Context, _ string) (string, string, float64, error) {
	return s.a, s.b, s.ratio, nil
}
func (s stubReader) GetFeeBps(_ context.Context) (uint64, error) { return 30, nil }

type stubCounterpart struct {
	cp *CounterpartCommit
}

func (s stubCounterpart) CounterpartCommit(_ context.Context, _ string) (*CounterpartCommit, error) {
	return s.cp, nil
}

func TestGetPoolStatus_CounterpartMakesPoolPending(t *testing.T) {
	cp := &CounterpartCommit{
		Side:            "B",
		SignerAddress:   "0xf17f52151EbEF6C7334FAD080c5704D77216b732",
		Amount:          "100000",
		ExpiresAt:       time.Now().Add(72 * time.Hour),
		OnChainCommitID: "0xabc",
	}
	svc := NewPoolStatusService(stubReader{a: "0", b: "0"}).
		WithCounterpartSource(stubCounterpart{cp: cp})

	resp, err := svc.GetPoolStatus(context.Background(), "W-BRL-ARS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PoolStatus != "PENDING_COUNTERPART" {
		t.Errorf("expected PENDING_COUNTERPART, got %s", resp.PoolStatus)
	}
	if resp.CounterpartCommit == nil || resp.CounterpartCommit.OnChainCommitID != "0xabc" {
		t.Errorf("expected counterpart_commit to be surfaced, got %+v", resp.CounterpartCommit)
	}
}

func TestGetPoolStatus_NoCounterpartStaysEmpty(t *testing.T) {
	svc := NewPoolStatusService(stubReader{a: "0", b: "0"}).
		WithCounterpartSource(stubCounterpart{cp: nil})

	resp, err := svc.GetPoolStatus(context.Background(), "W-BRL-ARS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PoolStatus != "EMPTY" {
		t.Errorf("expected EMPTY, got %s", resp.PoolStatus)
	}
	if resp.CounterpartCommit != nil {
		t.Errorf("expected nil counterpart_commit, got %+v", resp.CounterpartCommit)
	}
}
