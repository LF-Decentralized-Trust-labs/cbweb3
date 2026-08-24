// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
)

// auditCtxRecorder captures the context emitAudit issues its call with. The embedded
// interface is left nil on purpose: emitAudit touches CreateAuditLog and nothing else, so
// any other call is a bug this test should expose as a panic rather than absorb.
type auditCtxRecorder struct {
	complianceclient.Client
	got chan context.Context
}

func (r *auditCtxRecorder) CreateAuditLog(ctx context.Context, _ complianceclient.AuditEntry) error {
	r.got <- ctx
	return nil
}

// emitAudit deliberately detaches from the request context — the audit entry has to
// outlive the RPC that triggered it. Detached must not mean unbounded: a compliance
// service that accepts the connection and then stops answering would otherwise leak one
// goroutine per audited operation, for the life of the process (finding R2-LOW).
func TestEmitAudit_CallIsBounded(t *testing.T) {
	rec := &auditCtxRecorder{got: make(chan context.Context, 1)}
	s := &identityService{compliance: rec}

	s.emitAudit(context.Background(), "TEST_ACTION", "actor-1", "", "", "corr-1", "127.0.0.1", "SUCCESS")

	select {
	case ctx := <-rec.got:
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("emitAudit issued the audit call with no deadline: a hung compliance service leaks a goroutine per audited operation")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > auditEmitTimeout {
			t.Errorf("deadline is %v away, want (0, %v]", remaining, auditEmitTimeout)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("emitAudit never reached CreateAuditLog")
	}
}

// The bound has to stay short enough to be worth having.
func TestAuditEmitTimeout_IsMeaningful(t *testing.T) {
	if auditEmitTimeout <= 0 {
		t.Fatal("auditEmitTimeout is unset — the detached call is unbounded")
	}
	if auditEmitTimeout > 30*time.Second {
		t.Errorf("auditEmitTimeout = %v: too long to bound a single audit write", auditEmitTimeout)
	}
}
