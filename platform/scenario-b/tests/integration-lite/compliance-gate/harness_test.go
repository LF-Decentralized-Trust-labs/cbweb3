// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

// Package integrationlite is the hermetic, in-process integration-lite tier for
// the Scenario B compliance gate. It wires the compliance service's REAL gRPC
// server (grpc/server.New, in-memory repository) over bufconn together with the
// REAL services.ZKComplianceGate (in-memory SQLite zk-pointer table). No Docker,
// no live chain / Postgres / Keycloak. Target runtime: <2 min.
package integrationlite

import (
	"context"
	"net"
	"testing"
	"time"

	server "github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/grpc/server"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/repository"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
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

// complianceEnv exposes the REAL compliance gRPC service (in-memory repo, no CA,
// noop on-chain registry) over an in-process bufconn client.
type complianceEnv struct {
	client compliancv1.ComplianceServiceClient
	repo   repository.Repository
}

func newComplianceEnv(t *testing.T) *complianceEnv {
	t.Helper()
	repo := repository.NewMemoryRepository()
	// CA nil and registry nil are explicitly supported by server.New (dev/test mode).
	srv, err := server.New(repo, nil, nil)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	conn := dialBufconn(t, srv)
	return &complianceEnv{
		client: compliancv1.NewComplianceServiceClient(conn),
		repo:   repo,
	}
}
