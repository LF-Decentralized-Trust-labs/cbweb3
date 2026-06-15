// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
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
)

// secpFixture is a secp256k1 wallet: its private key, uncompressed pub key hex,
// and derived 0x address. Used to construct valid PoP signatures.
type secpFixture struct {
	pubHex  string
	address string
	signPoP func(nonceHex string) string // hex 65-byte recoverable sig over sha256(nonce)
}

func newSecpFixture(t *testing.T) secpFixture {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pubHex := hex.EncodeToString(gethcrypto.FromECDSAPub(&key.PublicKey))
	addr := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	return secpFixture{
		pubHex:  pubHex,
		address: addr,
		signPoP: func(nonceHex string) string {
			nonceBytes, _ := hex.DecodeString(nonceHex)
			digest := sha256.Sum256(nonceBytes)
			sig, err := gethcrypto.Sign(digest[:], key)
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			return hex.EncodeToString(sig)
		},
	}
}

// ---------------------------------------------------------------------------
// SubmitCredentialRequest
// ---------------------------------------------------------------------------

func TestSubmitCredentialRequest(t *testing.T) {
	ctx := context.Background()
	secp := newSecpFixture(t)

	// Valid P-256 CSR for the credential request.
	csrPEM, _, err := pki.GenerateCSR("user", "Bank", "ROLE_COMMERCIAL_BANK", "BR")
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}

	okKC := func() *fakeKeycloak {
		return &fakeKeycloak{
			getAdminTokenFn: func(_ context.Context) (string, error) { return "adm", nil },
			createUserFn:    func(_ context.Context, _ string, _ keycloak.CreateUserRequest) (string, error) { return "uid-7", nil },
			assignRoleFn:    func(_ context.Context, _, _, _ string) error { return nil },
		}
	}
	validReq := func() *authv1.SubmitCredentialRequestReq {
		return &authv1.SubmitCredentialRequestReq{
			CsrPem:              csrPEM,
			BlockchainPubKeyHex: secp.pubHex,
			Role:                domain.RoleCommercialBank,
			Username:            "u",
			Email:               "e@x.com",
			InstitutionName:     "Bank",
		}
	}

	t.Run("missing_csr_fields", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").SubmitCredentialRequest(ctx, &authv1.SubmitCredentialRequestReq{})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("missing_user_fields", func(t *testing.T) {
		r := validReq()
		r.Username = ""
		_, err := svc(nil, nil, nil, nil, nil, "").SubmitCredentialRequest(ctx, r)
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("invalid_csr_pem", func(t *testing.T) {
		r := validReq()
		r.CsrPem = "not-a-pem"
		_, err := svc(nil, nil, nil, nil, nil, "").SubmitCredentialRequest(ctx, r)
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("invalid_pub_key", func(t *testing.T) {
		r := validReq()
		r.BlockchainPubKeyHex = "zzzz"
		_, err := svc(okKC(), nil, nil, nil, nil, "").SubmitCredentialRequest(ctx, r)
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("admin_token_error", func(t *testing.T) {
		kc := okKC()
		kc.getAdminTokenFn = func(_ context.Context) (string, error) { return "", errors.New("x") }
		_, err := svc(kc, nil, nil, nil, nil, "").SubmitCredentialRequest(ctx, validReq())
		wantCode(t, err, codes.Internal)
	})
	t.Run("user_already_exists", func(t *testing.T) {
		kc := okKC()
		kc.createUserFn = func(_ context.Context, _ string, _ keycloak.CreateUserRequest) (string, error) {
			return "", errors.New("user already exists")
		}
		_, err := svc(kc, nil, nil, nil, nil, "").SubmitCredentialRequest(ctx, validReq())
		wantCode(t, err, codes.AlreadyExists)
	})
	t.Run("upsert_conflict", func(t *testing.T) {
		comp := &fakeCompliance{upsertFn: func(_ context.Context, _ complianceclient.Participant) error {
			return errors.New("duplicate")
		}}
		_, err := svc(okKC(), comp, nil, nil, nil, "").SubmitCredentialRequest(ctx, validReq())
		wantCode(t, err, codes.AlreadyExists)
	})
	t.Run("success", func(t *testing.T) {
		comp := &fakeCompliance{upsertFn: func(_ context.Context, p complianceclient.Participant) error {
			if p.Status != string(domain.ParticipantStatusCredentialRequested) {
				t.Fatalf("status=%q", p.Status)
			}
			if p.WalletAddress != secp.address {
				t.Fatalf("wallet=%q want %q", p.WalletAddress, secp.address)
			}
			return nil
		}}
		resp, err := svc(okKC(), comp, nil, nil, nil, "").SubmitCredentialRequest(ctx, validReq())
		if err != nil || resp.UserId != "uid-7" || resp.WalletAddress != secp.address {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
}

// ---------------------------------------------------------------------------
// GetOnboardingStatus
// ---------------------------------------------------------------------------

func TestGetOnboardingStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("missing_request_id", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("bank_code_empty", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: "bank_code:"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("bank_code_list_error", func(t *testing.T) {
		comp := &fakeCompliance{listFn: func(_ context.Context, _ complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
			return nil, errors.New("db")
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: "bank_code:001"})
		wantCode(t, err, codes.Internal)
	})
	t.Run("bank_code_not_found", func(t *testing.T) {
		comp := &fakeCompliance{listFn: func(_ context.Context, _ complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
			return nil, nil
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: "bank_code:001"})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("bank_code_success", func(t *testing.T) {
		comp := &fakeCompliance{listFn: func(_ context.Context, _ complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
			return []complianceclient.Participant{{UserID: "u1", Status: "PENDING"}}, nil
		}}
		resp, err := svc(nil, comp, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: "bank_code:001"})
		if err != nil || resp.UserId != "u1" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
	t.Run("by_user_error", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, errors.New("db")
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: "uid"})
		wantCode(t, err, codes.Internal)
	})
	t.Run("by_user_not_found", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, nil
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: "uid"})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("exposes_nonce_when_approved_unexpired", func(t *testing.T) {
		future := time.Now().UTC().Add(time.Hour)
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{
				UserID:            "uid",
				Status:            string(domain.ParticipantStatusKYCApproved),
				PopNonce:          "abc123",
				PopNonceExpiresAt: &future,
			}, true, nil
		}}
		resp, _ := svc(nil, comp, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: "uid"})
		if resp.PopNonce != "abc123" {
			t.Fatalf("expected nonce exposed, got %q", resp.PopNonce)
		}
	})
	t.Run("hides_nonce_when_expired", func(t *testing.T) {
		past := time.Now().UTC().Add(-time.Hour)
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{
				UserID:            "uid",
				Status:            string(domain.ParticipantStatusKYCApproved),
				PopNonce:          "abc123",
				PopNonceExpiresAt: &past,
			}, true, nil
		}}
		resp, _ := svc(nil, comp, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: "uid"})
		if resp.PopNonce != "" {
			t.Fatalf("expected nonce hidden, got %q", resp.PopNonce)
		}
	})
	t.Run("no_nonce_when_not_approved", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{UserID: "uid", Status: "PENDING", PopNonce: "x"}, true, nil
		}}
		resp, _ := svc(nil, comp, nil, nil, nil, "").GetOnboardingStatus(ctx, &authv1.GetOnboardingStatusRequest{RequestId: "uid"})
		if resp.PopNonce != "" {
			t.Fatalf("nonce should be hidden, got %q", resp.PopNonce)
		}
	})
}

// ---------------------------------------------------------------------------
// CompleteOnboarding
// ---------------------------------------------------------------------------

func TestCompleteOnboarding(t *testing.T) {
	ctx := context.Background()
	secp := newSecpFixture(t)
	nonceHex := hex.EncodeToString([]byte("pop-nonce"))
	popSig := secp.signPoP(nonceHex)

	csrPEM, _, _ := pki.GenerateCSR("user", "Bank", "ROLE_COMMERCIAL_BANK", "BR")
	future := time.Now().UTC().Add(time.Hour)

	approvedParticipant := func() complianceclient.Participant {
		return complianceclient.Participant{
			UserID:            "uid-9",
			Role:              domain.RoleCommercialBank,
			InstitutionName:   "Bank",
			WalletAddress:     secp.address,
			Status:            string(domain.ParticipantStatusKYCApproved),
			PopNonce:          nonceHex,
			PopNonceExpiresAt: &future,
			CsrPem:            csrPEM,
		}
	}
	okKC := func() *fakeKeycloak {
		return &fakeKeycloak{
			getAdminTokenFn: func(_ context.Context) (string, error) { return "adm", nil },
			resetPasswordFn: func(_ context.Context, _, _, _ string) error { return nil },
		}
	}
	validReq := func() *authv1.CompleteOnboardingRequest {
		return &authv1.CompleteOnboardingRequest{
			UserId:              "uid-9",
			PopSignatureHex:     popSig,
			BlockchainPubKeyHex: secp.pubHex,
		}
	}

	t.Run("missing_fields", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").CompleteOnboarding(ctx, &authv1.CompleteOnboardingRequest{UserId: "u"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("lookup_error", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, errors.New("db")
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").CompleteOnboarding(ctx, validReq())
		wantCode(t, err, codes.Internal)
	})
	t.Run("not_found", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, nil
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").CompleteOnboarding(ctx, validReq())
		wantCode(t, err, codes.NotFound)
	})
	t.Run("wrong_status", func(t *testing.T) {
		p := approvedParticipant()
		p.Status = "PENDING"
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return p, true, nil
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").CompleteOnboarding(ctx, validReq())
		wantCode(t, err, codes.FailedPrecondition)
	})
	t.Run("no_nonce", func(t *testing.T) {
		p := approvedParticipant()
		p.PopNonce = ""
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return p, true, nil
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").CompleteOnboarding(ctx, validReq())
		wantCode(t, err, codes.FailedPrecondition)
	})
	t.Run("nonce_expired", func(t *testing.T) {
		p := approvedParticipant()
		past := time.Now().UTC().Add(-time.Hour)
		p.PopNonceExpiresAt = &past
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return p, true, nil
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").CompleteOnboarding(ctx, validReq())
		wantCode(t, err, codes.DeadlineExceeded)
	})
	t.Run("pop_verification_failed", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return approvedParticipant(), true, nil
		}}
		r := validReq()
		// Sign a different nonce so recovered addr won't match the stored nonce digest.
		r.PopSignatureHex = secp.signPoP(hex.EncodeToString([]byte("other-nonce")))
		_, err := svc(nil, comp, nil, nil, nil, "").CompleteOnboarding(ctx, r)
		wantCode(t, err, codes.Unauthenticated)
	})
	t.Run("pub_key_mismatch", func(t *testing.T) {
		other := newSecpFixture(t)
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return approvedParticipant(), true, nil
		}}
		r := validReq()
		r.BlockchainPubKeyHex = other.pubHex
		_, err := svc(nil, comp, nil, nil, nil, "").CompleteOnboarding(ctx, r)
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("no_csr", func(t *testing.T) {
		p := approvedParticipant()
		p.CsrPem = ""
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return p, true, nil
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").CompleteOnboarding(ctx, validReq())
		wantCode(t, err, codes.FailedPrecondition)
	})
	t.Run("sign_csr_error", func(t *testing.T) {
		comp := &fakeCompliance{
			getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
				return approvedParticipant(), true, nil
			},
			signCSRFn: func(_ context.Context, _, _, _, _, _ string) (complianceclient.SignedCSR, error) {
				return complianceclient.SignedCSR{}, errors.New("ca down")
			},
		}
		_, err := svc(nil, comp, nil, nil, nil, "").CompleteOnboarding(ctx, validReq())
		wantCode(t, err, codes.Internal)
	})
	t.Run("onchain_error", func(t *testing.T) {
		comp := &fakeCompliance{
			getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
				return approvedParticipant(), true, nil
			},
			signCSRFn: func(_ context.Context, _, _, _, _, _ string) (complianceclient.SignedCSR, error) {
				return complianceclient.SignedCSR{CertPEM: "CERT"}, nil
			},
		}
		bc := &fakeRegistry{registerFn: func(_ context.Context, _, _, _ string, _ [32]byte) (string, error) {
			return "", errors.New("revert")
		}}
		_, err := svc(nil, comp, nil, bc, nil, "").CompleteOnboarding(ctx, validReq())
		wantCode(t, err, codes.Internal)
	})
	t.Run("reset_password_error", func(t *testing.T) {
		comp := &fakeCompliance{
			getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
				return approvedParticipant(), true, nil
			},
			signCSRFn: func(_ context.Context, _, _, _, _, _ string) (complianceclient.SignedCSR, error) {
				return complianceclient.SignedCSR{CertPEM: "CERT"}, nil
			},
		}
		bc := &fakeRegistry{registerFn: func(_ context.Context, _, _, _ string, _ [32]byte) (string, error) {
			return "0xTX", nil
		}}
		kc := okKC()
		kc.resetPasswordFn = func(_ context.Context, _, _, _ string) error { return errors.New("boom") }
		_, err := svc(kc, comp, nil, bc, nil, "").CompleteOnboarding(ctx, validReq())
		wantCode(t, err, codes.Internal)
	})
	t.Run("success", func(t *testing.T) {
		comp := &fakeCompliance{
			getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
				return approvedParticipant(), true, nil
			},
			signCSRFn: func(_ context.Context, _, _, _, _, _ string) (complianceclient.SignedCSR, error) {
				return complianceclient.SignedCSR{CertPEM: "CERTPEM"}, nil
			},
			upsertFn: func(_ context.Context, p complianceclient.Participant) error {
				if p.Status != string(domain.ParticipantStatusActive) || p.CertificateData != "CERTPEM" {
					t.Fatalf("unexpected upsert: %+v", p)
				}
				return nil
			},
		}
		bc := &fakeRegistry{registerFn: func(_ context.Context, _, _, _ string, _ [32]byte) (string, error) {
			return "0xTX", nil
		}}
		resp, err := svc(okKC(), comp, nil, bc, nil, "").CompleteOnboarding(ctx, validReq())
		if err != nil || resp.Status != string(domain.ParticipantStatusActive) || resp.CertPem != "CERTPEM" || resp.TxHash != "0xTX" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
}

// ---------------------------------------------------------------------------
// onboarding_kms: CreateOnboardingKey / SignOnboardingPoP / GetOnboardingKey
// ---------------------------------------------------------------------------

// kmsSecpProvider is a kms.Provider backed by a real secp256k1 key so that the
// public-key recovery path (ecrecover) in addressToPubKeyHex works end-to-end.
type kmsSecpProvider struct {
	fakeKMS
}

func newKMSSecp(t *testing.T) (*kmsSecpProvider, string) {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	addr := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	p := &kmsSecpProvider{}
	p.createKeyFn = func(_ context.Context, _ string) (kms.KeyInfo, error) {
		return kms.KeyInfo{Address: addr}, nil
	}
	p.getAddressFn = func(_ context.Context, _ string) (string, error) { return addr, nil }
	p.signFn = func(_ context.Context, _, digestHex string) (kms.SignResult, error) {
		digest, _ := hex.DecodeString(digestHex)
		sig, err := gethcrypto.Sign(digest, key)
		if err != nil {
			return kms.SignResult{}, err
		}
		// Emulate cast-style V (27/28) prefix with 0x to exercise normalization.
		sig[64] += 27
		return kms.SignResult{Signature: "0x" + hex.EncodeToString(sig), Address: addr}, nil
	}
	return p, addr
}

func TestCreateOnboardingKey(t *testing.T) {
	ctx := context.Background()
	t.Run("missing_bank_code", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").CreateOnboardingKey(ctx, &authv1.CreateOnboardingKeyRequest{})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("kms_error", func(t *testing.T) {
		k := &fakeKMS{createKeyFn: func(_ context.Context, _ string) (kms.KeyInfo, error) {
			return kms.KeyInfo{}, errors.New("kms")
		}}
		s := &identityService{kms: k}
		_, err := s.CreateOnboardingKey(ctx, &authv1.CreateOnboardingKeyRequest{BankCode: "001"})
		wantCode(t, err, codes.Internal)
	})
	t.Run("success", func(t *testing.T) {
		k, addr := newKMSSecp(t)
		s := &identityService{kms: k}
		resp, err := s.CreateOnboardingKey(ctx, &authv1.CreateOnboardingKeyRequest{BankCode: "001"})
		if err != nil || resp.Address != addr || resp.PubKeyHex == "" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
}

func TestSignOnboardingPoP(t *testing.T) {
	ctx := context.Background()
	t.Run("missing_fields", func(t *testing.T) {
		s := &identityService{kms: &fakeKMS{}}
		_, err := s.SignOnboardingPoP(ctx, &authv1.SignOnboardingPoPRequest{KeyId: "k"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("invalid_nonce_hex", func(t *testing.T) {
		s := &identityService{kms: &fakeKMS{}}
		_, err := s.SignOnboardingPoP(ctx, &authv1.SignOnboardingPoPRequest{KeyId: "k", NonceHex: "zz"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("kms_sign_error", func(t *testing.T) {
		k := &fakeKMS{signFn: func(_ context.Context, _, _ string) (kms.SignResult, error) {
			return kms.SignResult{}, errors.New("hsm")
		}}
		s := &identityService{kms: k}
		_, err := s.SignOnboardingPoP(ctx, &authv1.SignOnboardingPoPRequest{KeyId: "k", NonceHex: "ab"})
		wantCode(t, err, codes.Internal)
	})
	t.Run("success_normalizes_v", func(t *testing.T) {
		k, _ := newKMSSecp(t)
		s := &identityService{kms: k}
		resp, err := s.SignOnboardingPoP(ctx, &authv1.SignOnboardingPoPRequest{KeyId: "k", NonceHex: hex.EncodeToString([]byte("n"))})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		// 65-byte sig => 130 hex chars; V normalized to 00 or 01.
		if len(resp.SignatureHex) != 130 {
			t.Fatalf("sig len=%d", len(resp.SignatureHex))
		}
		v := resp.SignatureHex[128:]
		if v != "00" && v != "01" {
			t.Fatalf("V not normalized: %q", v)
		}
		if resp.PubKeyHex == "" {
			t.Fatal("missing pub key")
		}
	})
}

func TestGetOnboardingKey(t *testing.T) {
	ctx := context.Background()
	t.Run("missing_key_id", func(t *testing.T) {
		s := &identityService{kms: &fakeKMS{}}
		_, err := s.GetOnboardingKey(ctx, &authv1.GetOnboardingKeyRequest{})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("not_found", func(t *testing.T) {
		k := &fakeKMS{getAddressFn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("missing")
		}}
		s := &identityService{kms: k}
		_, err := s.GetOnboardingKey(ctx, &authv1.GetOnboardingKeyRequest{KeyId: "k"})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("success", func(t *testing.T) {
		k, addr := newKMSSecp(t)
		s := &identityService{kms: k}
		resp, err := s.GetOnboardingKey(ctx, &authv1.GetOnboardingKeyRequest{KeyId: "k"})
		if err != nil || resp.Address != addr || resp.PubKeyHex == "" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
}
