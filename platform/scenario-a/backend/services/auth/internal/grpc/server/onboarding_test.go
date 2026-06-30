// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
	pki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// secp256k1KMS is a KMS fake backed by a real secp256k1 key, so signatures are
// genuinely ecrecover-able. This lets us exercise the full PoP / pub-key-recovery
// paths in onboarding without a live HSM.
type secp256k1KMS struct {
	priv *ecdsa.PrivateKey
}

func newSecpKMS(t *testing.T) *secp256k1KMS {
	t.Helper()
	k, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	return &secp256k1KMS{priv: k}
}

func (k *secp256k1KMS) address() string { return gethcrypto.PubkeyToAddress(k.priv.PublicKey).Hex() }
func (k *secp256k1KMS) pubKeyHex() string {
	return hex.EncodeToString(gethcrypto.FromECDSAPub(&k.priv.PublicKey))
}

func (k *secp256k1KMS) Name() string { return "secp" }
func (k *secp256k1KMS) CreateKey(_ context.Context, _ string) (kms.KeyInfo, error) {
	return kms.KeyInfo{Address: k.address()}, nil
}
func (k *secp256k1KMS) Sign(_ context.Context, _, digestHex string) (kms.SignResult, error) {
	digest, err := hex.DecodeString(digestHex)
	if err != nil {
		return kms.SignResult{}, err
	}
	sig, err := gethcrypto.Sign(digest, k.priv)
	if err != nil {
		return kms.SignResult{}, err
	}
	return kms.SignResult{Signature: "0x" + hex.EncodeToString(sig), Address: k.address()}, nil
}
func (k *secp256k1KMS) GetAddress(_ context.Context, _ string) (string, error) {
	return k.address(), nil
}
func (k *secp256k1KMS) DeleteKey(_ context.Context, _ string) error { return nil }

// ── CreateOnboardingKey / GetOnboardingKey / addressToPubKeyHex ───────────────

func TestCreateOnboardingKey(t *testing.T) {
	t.Parallel()
	km := newSecpKMS(t)
	svc := &identityService{kms: km}
	resp, err := svc.CreateOnboardingKey(context.Background(), &authv1.CreateOnboardingKeyRequest{BankCode: "bank-a"})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if resp.Address != km.address() || resp.PubKeyHex != km.pubKeyHex() {
		t.Errorf("unexpected resp %+v (want addr %s pub %s)", resp, km.address(), km.pubKeyHex())
	}

	// Missing bank code.
	if _, err := svc.CreateOnboardingKey(context.Background(), &authv1.CreateOnboardingKeyRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	// KMS error.
	svc.kms = &fakeKMS{createFn: func(ctx context.Context, uid string) (kms.KeyInfo, error) { return kms.KeyInfo{}, errors.New("down") }}
	if _, err := svc.CreateOnboardingKey(context.Background(), &authv1.CreateOnboardingKeyRequest{BankCode: "b"}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

func TestGetOnboardingKey(t *testing.T) {
	t.Parallel()
	km := newSecpKMS(t)
	svc := &identityService{kms: km}
	resp, err := svc.GetOnboardingKey(context.Background(), &authv1.GetOnboardingKeyRequest{KeyId: "bank-a"})
	if err != nil {
		t.Fatalf("get key: %v", err)
	}
	if resp.Address != km.address() {
		t.Errorf("addr = %q", resp.Address)
	}

	// Missing key id.
	if _, err := svc.GetOnboardingKey(context.Background(), &authv1.GetOnboardingKeyRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
}

func TestSignOnboardingPoP(t *testing.T) {
	t.Parallel()
	km := newSecpKMS(t)
	svc := &identityService{kms: km}

	nonce := "deadbeefcafe"
	resp, err := svc.SignOnboardingPoP(context.Background(), &authv1.SignOnboardingPoPRequest{KeyId: "bank-a", NonceHex: nonce})
	if err != nil {
		t.Fatalf("sign pop: %v", err)
	}
	if resp.SignatureHex == "" || resp.PubKeyHex != km.pubKeyHex() {
		t.Errorf("unexpected resp %+v", resp)
	}
	// Signature must verify as PoP against the KMS address.
	if err := pki.VerifyPoP(nonce, resp.SignatureHex, km.address()); err != nil {
		t.Errorf("PoP signature should verify: %v", err)
	}

	// Validation.
	if _, err := svc.SignOnboardingPoP(context.Background(), &authv1.SignOnboardingPoPRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
	// Invalid nonce hex.
	if _, err := svc.SignOnboardingPoP(context.Background(), &authv1.SignOnboardingPoPRequest{KeyId: "k", NonceHex: "zz"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument (nonce), got %v", err)
	}
}

// ── SubmitCredentialRequest ───────────────────────────────────────────────────

func TestSubmitCredentialRequest(t *testing.T) {
	t.Parallel()
	km := newSecpKMS(t)
	csrPEM, _, err := pki.GenerateCSR("bank-user", "Bank A", domain.RoleCommercialBank, "BR")
	if err != nil {
		t.Fatalf("csr: %v", err)
	}

	base := func() *identityService {
		return &identityService{
			keycloak: &fakeKeycloak{
				adminTokenFn: func(ctx context.Context) (string, error) { return "adm", nil },
				createUserFn: func(ctx context.Context, a string, r keycloak.CreateUserRequest) (string, error) { return "uid", nil },
			},
			compliance: &fakeCompliance{upsertFn: func(ctx context.Context, p complianceclient.Participant) error { return nil }},
		}
	}

	resp, err := base().SubmitCredentialRequest(context.Background(), &authv1.SubmitCredentialRequestReq{
		CsrPem: csrPEM, BlockchainPubKeyHex: km.pubKeyHex(), Role: domain.RoleCommercialBank,
		Username: "bank-user", Email: "b@x.com", InstitutionName: "Bank A",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if resp.Status != string(domain.ParticipantStatusCredentialRequested) || resp.WalletAddress != km.address() {
		t.Errorf("unexpected resp %+v", resp)
	}

	// Validation: missing crypto fields.
	if _, err := base().SubmitCredentialRequest(context.Background(), &authv1.SubmitCredentialRequestReq{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
	// Validation: missing identity fields.
	if _, err := base().SubmitCredentialRequest(context.Background(), &authv1.SubmitCredentialRequestReq{
		CsrPem: csrPEM, BlockchainPubKeyHex: km.pubKeyHex(), Role: domain.RoleCommercialBank,
	}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument (identity), got %v", err)
	}
	// Invalid CSR PEM.
	if _, err := base().SubmitCredentialRequest(context.Background(), &authv1.SubmitCredentialRequestReq{
		CsrPem: "not-pem", BlockchainPubKeyHex: km.pubKeyHex(), Role: "R", Username: "u", Email: "e", InstitutionName: "i",
	}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument (csr pem), got %v", err)
	}
	// Invalid pub key.
	if _, err := base().SubmitCredentialRequest(context.Background(), &authv1.SubmitCredentialRequestReq{
		CsrPem: csrPEM, BlockchainPubKeyHex: "00", Role: "R", Username: "u", Email: "e", InstitutionName: "i",
	}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument (pubkey), got %v", err)
	}

	// User already exists.
	s1 := base()
	s1.keycloak.(*fakeKeycloak).createUserFn = func(ctx context.Context, a string, r keycloak.CreateUserRequest) (string, error) {
		return "", errors.New("user already exists")
	}
	if _, err := s1.SubmitCredentialRequest(context.Background(), &authv1.SubmitCredentialRequestReq{
		CsrPem: csrPEM, BlockchainPubKeyHex: km.pubKeyHex(), Role: "R", Username: "u", Email: "e", InstitutionName: "i",
	}); status.Code(err) != codes.AlreadyExists {
		t.Errorf("expected AlreadyExists, got %v", err)
	}
}

// ── GetOnboardingStatus ───────────────────────────────────────────────────────

func TestGetOnboardingStatus(t *testing.T) {
	t.Parallel()
	future := time.Now().UTC().Add(time.Hour)

	// By user id, KYC approved → exposes nonce.
	svc := &identityService{compliance: &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{UserID: uid, Status: string(domain.ParticipantStatusKYCApproved), PopNonce: "abc", PopNonceExpiresAt: &future}, true, nil
	}}}
	resp, err := svc.GetOnboardingStatus(context.Background(), &authv1.GetOnboardingStatusRequest{RequestId: "u"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if resp.PopNonce != "abc" {
		t.Errorf("expected pop nonce exposed, got %q", resp.PopNonce)
	}

	// Expired nonce is not exposed.
	past := time.Now().UTC().Add(-time.Hour)
	svc.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{Status: string(domain.ParticipantStatusKYCApproved), PopNonce: "abc", PopNonceExpiresAt: &past}, true, nil
	}}
	resp, _ = svc.GetOnboardingStatus(context.Background(), &authv1.GetOnboardingStatusRequest{RequestId: "u"})
	if resp.PopNonce != "" {
		t.Errorf("expired nonce must not be exposed, got %q", resp.PopNonce)
	}

	// By bank_code.
	svc.compliance = &fakeCompliance{listFn: func(ctx context.Context, f complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
		return []complianceclient.Participant{{UserID: "u-bank", Status: "PENDING"}}, nil
	}}
	resp, err = svc.GetOnboardingStatus(context.Background(), &authv1.GetOnboardingStatusRequest{RequestId: "bank_code:001"})
	if err != nil || resp.UserId != "u-bank" {
		t.Fatalf("by bank_code: resp=%+v err=%v", resp, err)
	}

	// Validation + not-found paths.
	if _, err := svc.GetOnboardingStatus(context.Background(), &authv1.GetOnboardingStatusRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
	if _, err := svc.GetOnboardingStatus(context.Background(), &authv1.GetOnboardingStatusRequest{RequestId: "bank_code:"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument (empty bank_code), got %v", err)
	}
	svc.compliance = &fakeCompliance{listFn: func(ctx context.Context, f complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
		return nil, nil
	}}
	if _, err := svc.GetOnboardingStatus(context.Background(), &authv1.GetOnboardingStatusRequest{RequestId: "bank_code:001"}); status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
	svc.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{}, false, nil
	}}
	if _, err := svc.GetOnboardingStatus(context.Background(), &authv1.GetOnboardingStatusRequest{RequestId: "u"}); status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound (by id), got %v", err)
	}
}

// ── CompleteOnboarding (full PoP round-trip) ──────────────────────────────────

func TestCompleteOnboarding_Success(t *testing.T) {
	t.Parallel()
	km := newSecpKMS(t)
	wallet := km.address()
	nonce := "deadbeef"

	// Sign the PoP nonce with the real key (sha256(nonce) digest).
	digest := sha256.Sum256(mustHex(t, nonce))
	rawSig, err := gethcrypto.Sign(digest[:], km.priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	popSig := hex.EncodeToString(rawSig)

	future := time.Now().UTC().Add(time.Hour)
	participant := complianceclient.Participant{
		UserID: "u", Role: domain.RoleCommercialBank, WalletAddress: wallet,
		InstitutionName: "Bank A", Status: string(domain.ParticipantStatusKYCApproved),
		PopNonce: nonce, PopNonceExpiresAt: &future, CsrPem: "csr",
	}

	svc := &identityService{
		kms: km,
		compliance: &fakeCompliance{
			getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
				return participant, true, nil
			},
			upsertFn: func(ctx context.Context, p complianceclient.Participant) error { return nil },
		},
		blockchainClient: &fakeRegistry{registerFn: func(ctx context.Context, a, i, r string, fp [32]byte) (string, error) { return "0xtx", nil }},
		keycloak: &fakeKeycloak{
			adminTokenFn: func(ctx context.Context) (string, error) { return "adm", nil },
			resetPwFn:    func(ctx context.Context, a, u, p string) error { return nil },
		},
	}
	// SignParticipantCSR returns a cert.
	svc.compliance = &completeOnboardingCompliance{participant: participant}

	resp, err := svc.CompleteOnboarding(context.Background(), &authv1.CompleteOnboardingRequest{
		UserId: "u", PopSignatureHex: popSig, BlockchainPubKeyHex: km.pubKeyHex(),
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	// CompleteOnboarding issues the cert and activates the participant; on-chain
	// IdentityRegistry registration is performed by the provisioning engine (CB
	// governance key), so the response carries no tx hash.
	if resp.Status != string(domain.ParticipantStatusActive) || resp.CertPem != "signed-cert" || resp.TxHash != "" {
		t.Errorf("unexpected resp %+v", resp)
	}
}

func TestCompleteOnboarding_Validation(t *testing.T) {
	t.Parallel()
	svc := &identityService{compliance: &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{}, false, nil
	}}}

	// Missing fields.
	if _, err := svc.CompleteOnboarding(context.Background(), &authv1.CompleteOnboardingRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
	// Not found.
	if _, err := svc.CompleteOnboarding(context.Background(), &authv1.CompleteOnboardingRequest{UserId: "u", PopSignatureHex: "a", BlockchainPubKeyHex: "b"}); status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}
	// Wrong status.
	svc.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{Status: "PENDING"}, true, nil
	}}
	if _, err := svc.CompleteOnboarding(context.Background(), &authv1.CompleteOnboardingRequest{UserId: "u", PopSignatureHex: "a", BlockchainPubKeyHex: "b"}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", err)
	}
	// KYC approved but no nonce.
	svc.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{Status: string(domain.ParticipantStatusKYCApproved)}, true, nil
	}}
	if _, err := svc.CompleteOnboarding(context.Background(), &authv1.CompleteOnboardingRequest{UserId: "u", PopSignatureHex: "a", BlockchainPubKeyHex: "b"}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition (no nonce), got %v", err)
	}
	// Expired nonce.
	past := time.Now().UTC().Add(-time.Hour)
	svc.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{Status: string(domain.ParticipantStatusKYCApproved), PopNonce: "abc", PopNonceExpiresAt: &past}, true, nil
	}}
	if _, err := svc.CompleteOnboarding(context.Background(), &authv1.CompleteOnboardingRequest{UserId: "u", PopSignatureHex: "a", BlockchainPubKeyHex: "b"}); status.Code(err) != codes.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded (expired), got %v", err)
	}
}

// completeOnboardingCompliance is a compliance fake that returns a fixed
// participant and a signed cert, used by the CompleteOnboarding happy path.
type completeOnboardingCompliance struct {
	participant complianceclient.Participant
}

func (c *completeOnboardingCompliance) UpsertParticipant(_ context.Context, _ complianceclient.Participant) error {
	return nil
}
func (c *completeOnboardingCompliance) GetParticipantByUser(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
	return c.participant, true, nil
}
func (c *completeOnboardingCompliance) ListParticipants(_ context.Context, _ complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
	return nil, nil
}
func (c *completeOnboardingCompliance) CreateAuditLog(_ context.Context, _ complianceclient.AuditEntry) error {
	return nil
}
func (c *completeOnboardingCompliance) IssueParticipantCertificate(_ context.Context, _, _, _, _ string) (complianceclient.IssuedCertificate, error) {
	return complianceclient.IssuedCertificate{}, errors.New("unexpected")
}
func (c *completeOnboardingCompliance) SignParticipantCSR(_ context.Context, _, _, _, _, _ string) (complianceclient.SignedCSR, error) {
	return complianceclient.SignedCSR{CertPEM: "signed-cert"}, nil
}
func (c *completeOnboardingCompliance) ManageParticipantStatus(_ context.Context, _, _, _ string) error {
	return nil
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex: %v", err)
	}
	return b
}
