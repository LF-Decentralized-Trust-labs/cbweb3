package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// FR-004/SC-003: waitSync completes only once the node reports not-syncing with
// a non-zero block; a node that never syncs hits the timeout with a clear error.
func TestWaitSyncSucceedsWhenSynced(t *testing.T) {
	calls := 0
	fake := func(context.Context, string) (bool, uint64, error) {
		calls++
		if calls < 3 {
			return true, 0, nil // still syncing
		}
		return false, 42, nil // synced
	}
	if err := waitSync(context.Background(), "http://node", 2*time.Second, time.Millisecond, fake); err != nil {
		t.Fatalf("waitSync should succeed once synced: %v", err)
	}
	if calls < 3 {
		t.Fatalf("expected polling until synced, calls=%d", calls)
	}
}

func TestWaitSyncTimesOut(t *testing.T) {
	fake := func(context.Context, string) (bool, uint64, error) { return true, 0, nil } // never syncs
	err := waitSync(context.Background(), "http://node", 20*time.Millisecond, time.Millisecond, fake)
	if err == nil {
		t.Fatal("waitSync must error when the node never syncs")
	}
}

// A synced report with block 0 is not "synced" — the node has no chain yet.
func TestWaitSyncRejectsZeroBlock(t *testing.T) {
	fake := func(context.Context, string) (bool, uint64, error) { return false, 0, nil }
	if err := waitSync(context.Background(), "http://node", 20*time.Millisecond, time.Millisecond, fake); err == nil {
		t.Fatal("waitSync must not treat block 0 as synced")
	}
}

// ethSyncing decodes eth_syncing (bool or progress object) + eth_blockNumber.
func TestEthSyncingDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result any
		switch req.Method {
		case "eth_syncing":
			result = false // not syncing
		case "eth_blockNumber":
			result = "0x2a" // 42
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	defer srv.Close()
	syncing, block, err := ethSyncing(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ethSyncing: %v", err)
	}
	if syncing || block != 42 {
		t.Fatalf("want (false, 42), got (%v, %d)", syncing, block)
	}
}
