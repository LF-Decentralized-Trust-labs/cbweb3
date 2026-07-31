// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// echoDesc registers a single unary method /authz.test.Echo/Ping that echoes the
// authenticated actor back to the caller. It lets the end-to-end mTLS tests below
// exercise the real transport + interceptor path without a generated proto stub.
func echoDesc() *grpc.ServiceDesc {
	return &grpc.ServiceDesc{
		ServiceName: "authz.test.Echo",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Ping",
			Handler: func(_ any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				req := new(emptypb.Empty)
				if err := dec(req); err != nil {
					return nil, err
				}
				handler := func(ctx context.Context, _ any) (any, error) {
					return &wrapperspb.StringValue{Value: Actor(ctx)}, nil
				}
				if interceptor == nil {
					return handler(ctx, req)
				}
				return interceptor(ctx, req, &grpc.UnaryServerInfo{FullMethod: "/authz.test.Echo/Ping"}, handler)
			},
		}},
	}
}

// writeKeyPair generates an ECDSA leaf certificate signed by caCert/caKey (or a
// self-signed CA when isCA), writes cert + key PEM to dir, and returns their paths.
func writeKeyPair(t *testing.T, dir, name, cn string, isCA bool, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, dns ...string) (string, string, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		DNSNames:              dns,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	signerCert, signerKey := tmpl, key
	if isCA {
		tmpl.IsCA = true
		tmpl.KeyUsage |= x509.KeyUsageCertSign
	} else {
		signerCert, signerKey = caCert, caKey
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signerCert, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatalf("create cert %s: %v", name, err)
	}
	leaf, _ := x509.ParseCertificate(der)
	certPath := filepath.Join(dir, name+".crt")
	keyPath := filepath.Join(dir, name+".key")
	keyDER, _ := x509.MarshalECPrivateKey(key)
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath, leaf, key
}

// startEnforcingServer stands up a real gRPC server whose options come from
// ServerOptionsFromEnv (so it exercises mTLS selection + enforce), over bufconn.
func startEnforcingServer(t *testing.T, certFile, keyFile, caFile string) *bufconn.Listener {
	t.Helper()
	t.Setenv(EnvCertFile, certFile)
	t.Setenv(EnvKeyFile, keyFile)
	t.Setenv(EnvCAFile, caFile)
	t.Setenv(EnvEnforce, "true")

	opts, err := ServerOptionsFromEnv(nil, nil)
	if err != nil {
		t.Fatalf("ServerOptionsFromEnv: %v", err)
	}
	srv := grpc.NewServer(opts...)
	srv.RegisterService(echoDesc(), nil)

	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis
}

// TestEndToEnd_MTLSEnforced_AuthenticatesCertAndRejectsPlaintext is the acceptance
// proof for R2-H-8: under mTLS + enforce, a properly-certified caller is admitted
// and its identity is the verified certificate CN, while a plaintext caller is
// rejected at the transport — no service accepts plaintext intra-cluster.
func TestEndToEnd_MTLSEnforced_AuthenticatesCertAndRejectsPlaintext(t *testing.T) {
	dir := t.TempDir()
	caCertFile, _, caCert, caKey := writeKeyPair(t, dir, "ca", "service-ca", true, nil, nil)
	srvCert, srvKey, _, _ := writeKeyPair(t, dir, "server", "compliance", false, caCert, caKey, "compliance")
	cliCert, cliKey, _, _ := writeKeyPair(t, dir, "client", "api-gateway", false, caCert, caKey)

	lis := startEnforcingServer(t, srvCert, srvKey, caCertFile)
	dialer := grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) })

	// 1. Authenticated mTLS caller: admitted, identity derived from the cert CN.
	clientCreds, err := ClientTLSConfig(cliCert, cliKey, caCertFile, "compliance")
	if err != nil {
		t.Fatalf("ClientTLSConfig: %v", err)
	}
	mtlsConn, err := grpc.NewClient("passthrough:///bufnet", dialer, grpc.WithTransportCredentials(clientCreds))
	if err != nil {
		t.Fatalf("dial mtls: %v", err)
	}
	defer mtlsConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out wrapperspb.StringValue
	if err := mtlsConn.Invoke(ctx, "/authz.test.Echo/Ping", &emptypb.Empty{}, &out); err != nil {
		t.Fatalf("authenticated mTLS call must succeed, got: %v", err)
	}
	if out.Value != "api-gateway" {
		t.Fatalf("expected actor from client cert CN 'api-gateway', got %q", out.Value)
	}

	// 2. Plaintext caller: the TLS handshake fails, so the call never reaches the
	//    handler — the server accepts no plaintext connection.
	plainConn, err := grpc.NewClient("passthrough:///bufnet", dialer, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial plaintext: %v", err)
	}
	defer plainConn.Close()

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	err = plainConn.Invoke(ctx2, "/authz.test.Echo/Ping", &emptypb.Empty{}, &wrapperspb.StringValue{})
	if err == nil {
		t.Fatal("plaintext caller MUST be rejected under mTLS enforcement")
	}
	if code := status.Code(err); code != codes.Unavailable {
		t.Logf("plaintext rejected with code %v (%v)", code, err) // any non-nil rejection is a pass
	}
}

// --- ClientDialOptionFromEnv --------------------------------------------------

func TestClientDialOptionFromEnv_DefaultsToInsecure(t *testing.T) {
	opt, err := ClientDialOptionFromEnv("compliance")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opt == nil {
		t.Fatal("expected a dial option")
	}
}

func TestClientDialOptionFromEnv_PartialConfigIsRejected(t *testing.T) {
	t.Setenv(EnvCertFile, "/pki/client.crt")
	if _, err := ClientDialOptionFromEnv("compliance"); err == nil {
		t.Fatal("expected error for partial client mTLS configuration")
	}
}

func TestClientDialOptionFromEnv_FullConfigRequiresServerName(t *testing.T) {
	dir := t.TempDir()
	caCertFile, _, caCert, caKey := writeKeyPair(t, dir, "ca", "service-ca", true, nil, nil)
	cliCert, cliKey, _, _ := writeKeyPair(t, dir, "client", "api-gateway", false, caCert, caKey)
	t.Setenv(EnvCertFile, cliCert)
	t.Setenv(EnvKeyFile, cliKey)
	t.Setenv(EnvCAFile, caCertFile)

	if _, err := ClientDialOptionFromEnv(""); err == nil {
		t.Fatal("expected error when serverName is empty under mTLS")
	}
	if _, err := ClientDialOptionFromEnv("compliance"); err != nil {
		t.Fatalf("expected mTLS dial option, got error: %v", err)
	}
}
