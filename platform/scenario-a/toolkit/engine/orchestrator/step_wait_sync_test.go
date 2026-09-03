// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// rpcServer dispatches JSON-RPC requests to per-method handlers for tests.
func rpcServer(handlers map[string]func(params []any) (any, error)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
			ID     int    `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		h, ok := handlers[req.Method]
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"error":{"code":-32601,"message":"method not found: %s"}}`, req.ID, req.Method)
			return
		}
		result, err := h(req.Params)
		if err != nil {
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"error":{"code":-32000,"message":%q}}`, req.ID, err.Error())
			return
		}
		out, _ := json.Marshal(result)
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":%s}`, req.ID, out)
	}))
}

func TestWaitSyncStep_ReachesTarget(t *testing.T) {
	var calls int64
	srv := rpcServer(map[string]func([]any) (any, error){
		"eth_blockNumber": func([]any) (any, error) {
			n := atomic.AddInt64(&calls, 1)
			if n < 3 {
				return "0x0", nil
			}
			return "0x5", nil // height 5 >= target
		},
		"net_peerCount": func([]any) (any, error) { return "0x2", nil }, // 2 peers → connected
	})
	defer srv.Close()

	step := newWaitSyncStep(srv.URL, 1, 5*time.Second, 10*time.Millisecond, nil)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestWaitSyncStep_Timeout(t *testing.T) {
	srv := rpcServer(map[string]func([]any) (any, error){
		"eth_blockNumber": func([]any) (any, error) { return "0x0", nil },
		"net_peerCount":   func([]any) (any, error) { return "0x2", nil },
	})
	defer srv.Close()

	step := newWaitSyncStep(srv.URL, 5, 100*time.Millisecond, 10*time.Millisecond, nil)
	if err := step.Run(context.Background()); err == nil {
		t.Fatal("Run should time out when target height is never reached")
	}
}

// A node with zero peers (isolated singleton) must NOT pass wait-sync even if it
// has produced its own block — guards against a misconfigured BOOTNODE_ENODE.
func TestWaitSyncStep_NoPeers_DoesNotPass(t *testing.T) {
	srv := rpcServer(map[string]func([]any) (any, error){
		"eth_blockNumber": func([]any) (any, error) { return "0x9", nil }, // has blocks
		"net_peerCount":   func([]any) (any, error) { return "0x0", nil }, // but isolated
	})
	defer srv.Close()

	step := newWaitSyncStep(srv.URL, 1, 100*time.Millisecond, 10*time.Millisecond, nil)
	if err := step.Run(context.Background()); err == nil {
		t.Fatal("Run must NOT pass when the node has zero peers (singleton)")
	}
}
