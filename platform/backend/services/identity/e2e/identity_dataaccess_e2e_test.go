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

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/contract"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/grpc/jsoncodec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

func TestIdentityAndDataAccessE2E(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	dataAccessPort := mustFreePort(t)
	identityPort := mustFreePort(t)
	dataAccessAddr := "127.0.0.1:" + dataAccessPort
	identityAddr := "127.0.0.1:" + identityPort

	repoRoot := mustRepoRoot(t)
	dataAccessDir := filepath.Join(repoRoot, "backend/services/data-access")
	identityDir := filepath.Join(repoRoot, "backend/services/identity")

	startService(t, ctx, dataAccessDir, map[string]string{
		"DATA_ACCESS_GRPC_PORT": dataAccessPort,
	})
	waitForTCP(t, dataAccessAddr, 20*time.Second)

	// INTERNAL_JWT_SECRET matches IDENTITY_JWT_SECRET so login token can be used
	// by provider.ValidateToken during RegisterParticipant in this E2E flow.
	startService(t, ctx, identityDir, map[string]string{
		"IDENTITY_PROVIDER":               "local",
		"IDENTITY_GRPC_PORT":              identityPort,
		"IDENTITY_HOST_URL":               "http://localhost:8081",
		"IDENTITY_JWT_SECRET":             "local-identity-secret",
		"INTERNAL_JWT_PROVIDER":           "local",
		"INTERNAL_JWT_SECRET":             "local-identity-secret",
		"DATA_ACCESS_GRPC_ADDR":           dataAccessAddr,
		"DATA_ACCESS_REQUEST_TIMEOUT_SEC": "5",
	})
	waitForTCP(t, identityAddr, 20*time.Second)

	codec := jsoncodec.Codec{}
	encoding.RegisterCodec(codec)
	conn, err := grpc.DialContext(
		ctx,
		identityAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec)),
	)
	if err != nil {
		t.Fatalf("failed to dial identity grpc: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := contract.NewIdentityServiceClient(conn)

	loginResp, err := client.Login(ctx, &contract.LoginRequest{
		User:     "bank-a",
		Password: "secret-a",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if strings.TrimSpace(loginResp.AccessToken) == "" {
		t.Fatal("expected non-empty login access token")
	}

	registerResp, err := client.RegisterParticipant(ctx, &contract.RegisterParticipantRequest{
		AccessToken: loginResp.AccessToken,
		Country:     "BR",
		BankCode:    "001",
		Role:        "issuer",
	})
	if err != nil {
		t.Fatalf("register participant failed: %v", err)
	}
	if registerResp.UserID != "bank-a" {
		t.Fatalf("unexpected user id: %s", registerResp.UserID)
	}
	if registerResp.KMSKeyID == "" {
		t.Fatal("expected non-empty kms key id")
	}
	if registerResp.WalletAddress == "" {
		t.Fatal("expected non-empty wallet address")
	}

	getResp, err := client.GetByUser(ctx, &contract.GetByUserRequest{UserID: "bank-a"})
	if err != nil {
		t.Fatalf("get by user failed: %v", err)
	}
	if !getResp.Found || getResp.Binding == nil {
		t.Fatal("expected wallet binding for bank-a")
	}

	digest := make([]byte, 32)
	for i := range digest {
		digest[i] = byte(i + 1)
	}
	signResp, err := client.SignTransaction(ctx, &contract.SignTransactionRequest{
		UserID: "bank-a",
		Digest: hex.EncodeToString(digest),
	})
	if err != nil {
		t.Fatalf("sign transaction failed: %v", err)
	}
	if signResp.Signature == "" || !strings.HasPrefix(signResp.Signature, "0x") {
		t.Fatalf("unexpected signature value: %q", signResp.Signature)
	}
	if signResp.KMSKeyID == "" {
		t.Fatal("expected kms key id in sign response")
	}
}

func TestIdentityAndDataAccessE2ENegativeScenarios(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	dataAccessPort := mustFreePort(t)
	identityPort := mustFreePort(t)
	dataAccessAddr := "127.0.0.1:" + dataAccessPort
	identityAddr := "127.0.0.1:" + identityPort

	repoRoot := mustRepoRoot(t)
	dataAccessDir := filepath.Join(repoRoot, "backend/services/data-access")
	identityDir := filepath.Join(repoRoot, "backend/services/identity")

	startService(t, ctx, dataAccessDir, map[string]string{
		"DATA_ACCESS_GRPC_PORT": dataAccessPort,
	})
	waitForTCP(t, dataAccessAddr, 20*time.Second)

	startService(t, ctx, identityDir, map[string]string{
		"IDENTITY_PROVIDER":               "local",
		"IDENTITY_GRPC_PORT":              identityPort,
		"IDENTITY_HOST_URL":               "http://localhost:8081",
		"IDENTITY_JWT_SECRET":             "local-identity-secret",
		"INTERNAL_JWT_PROVIDER":           "local",
		"INTERNAL_JWT_SECRET":             "local-identity-secret",
		"DATA_ACCESS_GRPC_ADDR":           dataAccessAddr,
		"DATA_ACCESS_REQUEST_TIMEOUT_SEC": "5",
	})
	waitForTCP(t, identityAddr, 20*time.Second)

	codec := jsoncodec.Codec{}
	encoding.RegisterCodec(codec)
	conn, err := grpc.DialContext(
		ctx,
		identityAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(codec)),
	)
	if err != nil {
		t.Fatalf("failed to dial identity grpc: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := contract.NewIdentityServiceClient(conn)

	_, err = client.RegisterParticipant(ctx, &contract.RegisterParticipantRequest{
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

	_, err = client.SignTransaction(ctx, &contract.SignTransactionRequest{
		UserID: "unknown-user",
		Digest: strings.Repeat("ab", 32),
	})
	if err == nil {
		t.Fatal("expected not found error for unknown participant")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", status.Code(err))
	}

	_, err = client.SignTransaction(ctx, &contract.SignTransactionRequest{
		UserID: "bank-a",
		Digest: "abcd",
	})
	if err == nil {
		t.Fatal("expected internal error for invalid digest without onboarding")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound before onboarding, got %v", status.Code(err))
	}

	loginResp, err := client.Login(ctx, &contract.LoginRequest{
		User:     "bank-a",
		Password: "secret-a",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	_, err = client.RegisterParticipant(ctx, &contract.RegisterParticipantRequest{
		AccessToken: loginResp.AccessToken,
		Country:     "BR",
		BankCode:    "001",
		Role:        "issuer",
	})
	if err != nil {
		t.Fatalf("register participant failed: %v", err)
	}

	_, err = client.SignTransaction(ctx, &contract.SignTransactionRequest{
		UserID: "bank-a",
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
	// Test runs from backend/services/identity/e2e.
	root := filepath.Clean(filepath.Join(wd, "../../../.."))
	return root
}
