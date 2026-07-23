// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

// Package integrationlite is the hermetic, in-process integration-lite tier for
// the Scenario B payment-orchestrator. It wires the orchestrator's REAL
// constructors (grpc/server.New, services.NewBridgeLockMintService,
// services.NewBridgeBurnUnlockService) against in-memory SQLite and hand-written
// fakes implementing the internal/ports interfaces — no Docker, no live chain /
// Postgres. In-process gRPC runs over bufconn. Target runtime: <2 min.
package integrationlite

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	server "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/grpc/server"
	pb "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/payment_orchestrator/v1"
	"github.com/glebarez/sqlite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMemDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return db
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

const bufSize = 1 << 20

func dialBufconn(t *testing.T, srv *grpc.Server) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	go func() { _ = srv.Serve(lis) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//nolint:staticcheck // grpc.DialContext + bufconn is the standard in-process wiring.
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	})
	return conn
}

type orchestratorEnv struct {
	client pb.PaymentOrchestratorServiceClient
	token  *spyToken
	fiat   *spyFiat
	relay  *spyRelay
}

// newOrchestratorEnv wires the REAL payment-orchestrator gRPC server (server.New)
// with an in-memory escrow repo and fake chain / interoperability ports, exposed
// over bufconn.
func newOrchestratorEnv(t *testing.T) *orchestratorEnv {
	t.Helper()
	token := &spyToken{balance: "1000000"}
	fiat := &spyFiat{balance: "1000000"}
	relay := &spyRelay{}

	srv, err := server.New(server.Config{
		Token:      token,
		Fiat:       fiat,
		Relay:      relay,
		EscrowRepo: newInMemEscrowRepo(),
		Logger:     testLogger(),
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	conn := dialBufconn(t, srv)
	return &orchestratorEnv{
		client: pb.NewPaymentOrchestratorServiceClient(conn),
		token:  token,
		fiat:   fiat,
		relay:  relay,
	}
}
