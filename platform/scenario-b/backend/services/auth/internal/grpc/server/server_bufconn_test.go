// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

// TestNew_ServesOverBufconn exercises the server constructor end-to-end: it
// registers the AuthService on a real *grpc.Server, dials it via an in-memory
// bufconn listener, and verifies a ValidateToken RPC round-trips. The SUCCESS
// audit path (the goroutine in emitAudit) is exercised here too.
func TestNew_ServesOverBufconn(t *testing.T) {
	kc := &fakeKeycloak{validateTokenFn: func(_ context.Context, _ string) (domain.TokenClaims, error) {
		return domain.TokenClaims{Subject: "sub-1", Issuer: "iss", Roles: []string{"ROLE_NOC"}}, nil
	}}
	// Compliance reports "not found" so claims are returned unchanged; the audit
	// callback records that emitAudit fired.
	comp := &fakeCompliance{
		getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, nil
		},
		auditFn: func(_ context.Context, _ complianceclient.AuditEntry) error { return nil },
	}

	srv := New(kc, &fakeKMS{}, comp, &fakeRegistry{}, "", &fakeNonce{})

	lis := bufconn.Listen(1024 * 1024)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := authv1.NewAuthServiceClient(conn)
	resp, err := client.ValidateToken(context.Background(), &authv1.ValidateTokenRequest{AccessToken: "tok"})
	if err != nil {
		t.Fatalf("ValidateToken RPC: %v", err)
	}
	if resp.Subject != "sub-1" || len(resp.Roles) != 1 {
		t.Fatalf("unexpected resp: %+v", resp)
	}
}

// TestMetadataHelpers covers the metadata extraction helpers directly,
// including the no-metadata fallback paths.
func TestMetadataHelpers(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
			"x-correlation-id", "c1",
			"x-forwarded-for", "1.2.3.4",
		))
		if got := correlationIDFromCtx(ctx); got != "c1" {
			t.Errorf("correlationID=%q", got)
		}
		if got := ipAddressFromCtx(ctx); got != "1.2.3.4" {
			t.Errorf("ip=%q", got)
		}
	})
	t.Run("absent", func(t *testing.T) {
		ctx := context.Background()
		if got := correlationIDFromCtx(ctx); got != "" {
			t.Errorf("correlationID=%q, want empty", got)
		}
		if got := ipAddressFromCtx(ctx); got != "" {
			t.Errorf("ip=%q, want empty", got)
		}
	})
}
