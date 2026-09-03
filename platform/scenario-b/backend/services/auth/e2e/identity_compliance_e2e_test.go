// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package e2e_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestIdentityAndComplianceE2E(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compliancePort := mustFreePort(t)
	identityPort := mustFreePort(t)
	complianceAddr := "127.0.0.1:" + compliancePort
	identityAddr := "127.0.0.1:" + identityPort

	repoRoot := mustRepoRoot(t)
	complianceDir := filepath.Join(repoRoot, "backend/services/compliance")
	authDir := filepath.Join(repoRoot, "backend/services/auth")

	startService(t, ctx, complianceDir, map[string]string{
		"COMPLIANCE_GRPC_PORT": compliancePort,
	})
	waitForTCP(t, complianceAddr, 20*time.Second)

	// INTERNAL_JWT_SECRET matches AUTH_JWT_SECRET so login token can be used
	// by provider.ValidateToken during RegisterParticipant in this E2E flow.
	startService(t, ctx, authDir, map[string]string{
		"AUTH_GRPC_PORT":                 identityPort,
		"COMPLIANCE_GRPC_ADDR":           complianceAddr,
		"COMPLIANCE_REQUEST_TIMEOUT_SEC": "5",
	})
	waitForTCP(t, identityAddr, 20*time.Second)

	conn, err := grpc.DialContext(
		ctx,
		identityAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial identity grpc: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := authv1.NewAuthServiceClient(conn)

	loginResp, err := client.Login(ctx, &authv1.LoginRequest{
		User:     "bank-a",
		Password: "secret-a",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if strings.TrimSpace(loginResp.AccessToken) == "" {
		t.Fatal("expected non-empty login access token")
	}

	registerResp, err := client.RegisterParticipant(ctx, &authv1.RegisterParticipantRequest{
		AccessToken: loginResp.AccessToken,
		Country:     "BR",
		BankCode:    "001",
		Role:        "issuer",
	})
	if err != nil {
		t.Fatalf("register participant failed: %v", err)
	}
	if registerResp.UserId != "bank-a" {
		t.Fatalf("unexpected user id: %s", registerResp.UserId)
	}
	if registerResp.WalletAddress == "" {
		t.Fatal("expected non-empty wallet address")
	}

	digest := make([]byte, 32)
	for i := range digest {
		digest[i] = byte(i + 1)
	}
	signResp, err := client.SignTransaction(ctx, &authv1.SignTransactionRequest{
		UserId: "bank-a",
		Digest: hex.EncodeToString(digest),
	})
	if err != nil {
		t.Fatalf("sign transaction failed: %v", err)
	}
	if signResp.Signature == "" || !strings.HasPrefix(signResp.Signature, "0x") {
		t.Fatalf("unexpected signature value: %q", signResp.Signature)
	}
}

func TestIdentityAndComplianceE2ENegativeScenarios(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compliancePort := mustFreePort(t)
	identityPort := mustFreePort(t)
	complianceAddr := "127.0.0.1:" + compliancePort
	identityAddr := "127.0.0.1:" + identityPort

	repoRoot := mustRepoRoot(t)
	complianceDir := filepath.Join(repoRoot, "backend/services/compliance")
	authDir := filepath.Join(repoRoot, "backend/services/auth")

	startService(t, ctx, complianceDir, map[string]string{
		"COMPLIANCE_GRPC_PORT": compliancePort,
	})
	waitForTCP(t, complianceAddr, 20*time.Second)

	startService(t, ctx, authDir, map[string]string{
		"AUTH_GRPC_PORT":                 identityPort,
		"COMPLIANCE_GRPC_ADDR":           complianceAddr,
		"COMPLIANCE_REQUEST_TIMEOUT_SEC": "5",
	})
	waitForTCP(t, identityAddr, 20*time.Second)

	conn, err := grpc.DialContext(
		ctx,
		identityAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial identity grpc: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := authv1.NewAuthServiceClient(conn)

	_, err = client.RegisterParticipant(ctx, &authv1.RegisterParticipantRequest{
		AccessToken: "invalid-token",
		Country:     "BR",
		BankCode:    "001",
		Role:        "issuer",
	})
	if err == nil {
		t.Fatal("expected unauthenticated error for invalid token")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", status.Code(err))
	}

	_, err = client.SignTransaction(ctx, &authv1.SignTransactionRequest{
		UserId: "unknown-user",
		Digest: strings.Repeat("ab", 32),
	})
	if err == nil {
		t.Fatal("expected not found error for unknown participant")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", status.Code(err))
	}

	_, err = client.SignTransaction(ctx, &authv1.SignTransactionRequest{
		UserId: "bank-a",
		Digest: "abcd",
	})
	if err == nil {
		t.Fatal("expected internal error for invalid digest without onboarding")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound before onboarding, got %v", status.Code(err))
	}

	loginResp, err := client.Login(ctx, &authv1.LoginRequest{
		User:     "bank-a",
		Password: "secret-a",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	_, err = client.RegisterParticipant(ctx, &authv1.RegisterParticipantRequest{
		AccessToken: loginResp.AccessToken,
		Country:     "BR",
		BankCode:    "001",
		Role:        "issuer",
	})
	if err != nil {
		t.Fatalf("register participant failed: %v", err)
	}

	_, err = client.SignTransaction(ctx, &authv1.SignTransactionRequest{
		UserId: "bank-a",
		Digest: "abcd",
	})
	if err == nil {
		t.Fatal("expected internal error for invalid digest size")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v", status.Code(err))
	}
}

func startService(t *testing.T, ctx context.Context, dir string, env map[string]string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/"+filepath.Base(dir))
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	mergedEnv := os.Environ()
	for k, v := range env {
		mergedEnv = append(mergedEnv, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = mergedEnv

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start service %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if cmd.Process == nil {
			return
		}
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			_ = cmd.Process.Kill()
		}
		waitDone := make(chan struct{})
		go func() {
			_, _ = cmd.Process.Wait()
			close(waitDone)
		}()
		select {
		case <-waitDone:
		case <-time.After(2 * time.Second):
		}
	})
}

func mustFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	return port
}

func waitForTCP(t *testing.T, address string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("service at %s did not become ready in %s", address, timeout)
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	// Test runs from backend/services/auth/e2e.
	root := filepath.Clean(filepath.Join(wd, "../../../.."))
	return root
}
