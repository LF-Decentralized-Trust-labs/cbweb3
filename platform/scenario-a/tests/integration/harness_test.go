// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

// Package integration contains the hermetic, in-process integration-test tier
// for Scenario A (Enhanced Correspondent Banking, dual-layer HTLC).
//
// These suites wire the REAL payment-orchestrator gRPC server (server.New) over
// an in-process bufconn transport, backed by hand-written fakes for the chain
// (Zeto / on-chain HTLC) and relay ports. No Docker, no live chain, no Keycloak,
// no Postgres. The full lane runs in well under two minutes.
//
// The compliance dependency is exercised over the real shared compliance proto
// via an in-process gRPC server (fakeCompliance) that faithfully replicates the
// service's CheckAndDeductTransferLimit wei-accumulation decision logic. The
// orchestrator never calls compliance directly — the API gateway does — so the
// AML/CFT suite drives the same gate the gateway enforces: compliance is
// consulted at payment initiation, and a deny blocks any settlement side-effect.
package integration

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	orchserver "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
	compliancev1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	orchpb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const (
	bufSize             = 1024 * 1024
	testPaladinIdentity = "funded_operator@spoke-a-bank-a"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io_Discard{}, &slog.HandlerOptions{Level: slog.LevelError}))
}

// io_Discard is a tiny io.Writer sink; avoids importing io just for Discard.
type io_Discard struct{}

func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }

// dialBuf returns a grpc.ClientConn backed by the given bufconn listener.
func dialBuf(t *testing.T, lis *bufconn.Listener) *grpc.ClientConn {
	t.Helper()
	//nolint:staticcheck // grpc.DialContext with a bufconn dialer is the documented in-process pattern.
	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// ── orchestrator harness ──────────────────────────────────────────────────────

// orchEnv bundles a running in-process orchestrator and its fakes so a suite can
// drive the gRPC API and assert on chain/relay side-effects.
type orchEnv struct {
	client orchpb.PaymentOrchestratorServiceClient
	zeto   *fakeZeto
	htlc   *fakeHTLC
	relay  *captureRelay
	fxRepo *memFXRepo
}

// orchConfig lets each suite tune the cross-spoke behaviour it needs.
type orchConfig struct {
	crossSpokeMode bool
	spokePrefix    string
	withHTLC       bool
	fxRepo         *memFXRepo
}

func startOrchestrator(t *testing.T, cfg orchConfig) *orchEnv {
	t.Helper()

	zeto := &fakeZeto{}
	relay := &captureRelay{}
	var htlc *fakeHTLC
	var htlcPort ports.HTLCContractPort
	if cfg.withHTLC {
		htlc = &fakeHTLC{}
		htlcPort = htlc
	}
	var fxRepoPort ports.FXAgreementRepository
	if cfg.fxRepo != nil {
		fxRepoPort = cfg.fxRepo
	}

	grpcServer, startWorkers, err := orchserver.New(orchserver.Config{
		Zeto:            zeto,
		HTLC:            htlcPort,
		Relay:           relay,
		FXRepo:          fxRepoPort,
		CrossSpokeMode:  cfg.crossSpokeMode,
		SpokePrefix:     cfg.spokePrefix,
		PaladinIdentity: testPaladinIdentity,
		Logger:          discardLogger(),
	})
	if err != nil {
		t.Fatalf("orchestrator server.New: %v", err)
	}

	lis := bufconn.Listen(bufSize)
	go func() { _ = grpcServer.Serve(lis) }()

	// Run the relay workers so the lock/settle handlers get registered on the
	// capture relay (mirrors production startRelayWorkers behaviour).
	workerCtx, workerCancel := context.WithCancel(context.Background())
	go startWorkers(workerCtx)

	t.Cleanup(func() {
		workerCancel()
		grpcServer.Stop()
		_ = lis.Close()
	})

	conn := dialBuf(t, lis)

	// Wait until the relay lock handler is registered before returning, so suites
	// can fire relay events immediately.
	deadline := time.After(2 * time.Second)
	for relay.lockHandler() == nil {
		select {
		case <-deadline:
			t.Fatal("relay handlers were not registered in time")
		case <-time.After(2 * time.Millisecond):
		}
	}

	return &orchEnv{
		client: orchpb.NewPaymentOrchestratorServiceClient(conn),
		zeto:   zeto,
		htlc:   htlc,
		relay:  relay,
		fxRepo: cfg.fxRepo,
	}
}

// ── compliance harness ────────────────────────────────────────────────────────

// startCompliance brings up the in-process fake compliance gRPC server over
// bufconn and returns a real proto client plus a handle to seed transfer limits.
func startCompliance(t *testing.T) (compliancev1.ComplianceServiceClient, *fakeCompliance) {
	t.Helper()

	svc := newFakeCompliance()
	grpcServer := grpc.NewServer()
	compliancev1.RegisterComplianceServiceServer(grpcServer, svc)

	lis := bufconn.Listen(bufSize)
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	conn := dialBuf(t, lis)
	return compliancev1.NewComplianceServiceClient(conn), svc
}
