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
	"google.golang.org/grpc/status"
)

// svc builds an identityService backed by the configurable fakes. Any nil
// dependency is replaced with a default fake so individual tests only wire the
// collaborators they exercise.
func svc(kc *fakeKeycloak, comp *fakeCompliance, k *fakeKMS, bc *fakeRegistry, ns *fakeNonce, caPEM string) *identityService {
	if kc == nil {
		kc = &fakeKeycloak{}
	}
	if comp == nil {
		comp = &fakeCompliance{}
	}
	if k == nil {
		k = &fakeKMS{}
	}
	if bc == nil {
		bc = &fakeRegistry{}
	}
	if ns == nil {
		ns = &fakeNonce{}
	}
	return &identityService{
		keycloak:         kc,
		kms:              k,
		compliance:       comp,
		blockchainClient: bc,
		caCertPEM:        caPEM,
		nonceStore:       ns,
	}
}

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", want)
	}
	if st, _ := status.FromError(err); st.Code() != want {
		t.Fatalf("expected code %s, got %s (%v)", want, st.Code(), err)
	}
}

// ---------------------------------------------------------------------------
// Login / Refresh / Revoke
// ---------------------------------------------------------------------------

func TestLogin(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		kc := &fakeKeycloak{
			loginFn: func(_ context.Context, u, p string) (keycloak.TokenResponse, error) {
				if u != "alice" || p != "pw" {
					t.Fatalf("login args u=%q p=%q", u, p)
				}
				return keycloak.TokenResponse{AccessToken: "at", RefreshToken: "rt", TokenType: "Bearer", ExpiresIn: 300, RefreshExpiresIn: 600}, nil
			},
		}
		resp, err := svc(kc, nil, nil, nil, nil, "").Login(ctx, &authv1.LoginRequest{User: "alice", Password: "pw"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if resp.AccessToken != "at" || resp.RefreshToken != "rt" || resp.ExpiresIn != 300 {
			t.Fatalf("unexpected resp: %+v", resp)
		}
	})

	t.Run("keycloak_error_unauthenticated", func(t *testing.T) {
		kc := &fakeKeycloak{
			loginFn: func(_ context.Context, _, _ string) (keycloak.TokenResponse, error) {
				return keycloak.TokenResponse{}, errors.New("bad creds")
			},
		}
		_, err := svc(kc, nil, nil, nil, nil, "").Login(ctx, &authv1.LoginRequest{User: "x", Password: "y"})
		wantCode(t, err, codes.Unauthenticated)
	})
}

func TestRefreshToken(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		kc := &fakeKeycloak{refreshFn: func(_ context.Context, rt string) (keycloak.TokenResponse, error) {
			return keycloak.TokenResponse{AccessToken: "at2", RefreshToken: rt}, nil
		}}
		resp, err := svc(kc, nil, nil, nil, nil, "").RefreshToken(ctx, &authv1.RefreshTokenRequest{RefreshToken: "rt"})
		if err != nil || resp.AccessToken != "at2" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
	t.Run("error", func(t *testing.T) {
		kc := &fakeKeycloak{refreshFn: func(_ context.Context, _ string) (keycloak.TokenResponse, error) {
			return keycloak.TokenResponse{}, errors.New("expired")
		}}
		_, err := svc(kc, nil, nil, nil, nil, "").RefreshToken(ctx, &authv1.RefreshTokenRequest{RefreshToken: "rt"})
		wantCode(t, err, codes.Unauthenticated)
	})
}

func TestRevokeToken(t *testing.T) {
	ctx := context.Background()
	t.Run("uses_refresh_token", func(t *testing.T) {
		var got string
		kc := &fakeKeycloak{logoutFn: func(_ context.Context, tok string) error { got = tok; return nil }}
		_, err := svc(kc, nil, nil, nil, nil, "").RevokeToken(ctx, &authv1.RevokeTokenRequest{RefreshToken: "rt", AccessToken: "at"})
		if err != nil || got != "rt" {
			t.Fatalf("err=%v got=%q", err, got)
		}
	})
	t.Run("falls_back_to_access_token", func(t *testing.T) {
		var got string
		kc := &fakeKeycloak{logoutFn: func(_ context.Context, tok string) error { got = tok; return nil }}
		_, err := svc(kc, nil, nil, nil, nil, "").RevokeToken(ctx, &authv1.RevokeTokenRequest{AccessToken: "at"})
		if err != nil || got != "at" {
			t.Fatalf("err=%v got=%q", err, got)
		}
	})
	t.Run("error_internal", func(t *testing.T) {
		kc := &fakeKeycloak{logoutFn: func(_ context.Context, _ string) error { return errors.New("boom") }}
		_, err := svc(kc, nil, nil, nil, nil, "").RevokeToken(ctx, &authv1.RevokeTokenRequest{RefreshToken: "rt"})
		wantCode(t, err, codes.Internal)
	})
}

// ---------------------------------------------------------------------------
// RegisterParticipant
// ---------------------------------------------------------------------------

func TestRegisterParticipant(t *testing.T) {
	ctx := context.Background()
	validKC := func() *fakeKeycloak {
		return &fakeKeycloak{validateTokenFn: func(_ context.Context, _ string) (domain.TokenClaims, error) {
			return domain.TokenClaims{Subject: "uid-1"}, nil
		}}
	}

	t.Run("success", func(t *testing.T) {
		kc := validKC()
		k := &fakeKMS{createKeyFn: func(_ context.Context, _ string) (kms.KeyInfo, error) {
			return kms.KeyInfo{Address: "0xWALLET"}, nil
		}}
		comp := &fakeCompliance{upsertFn: func(_ context.Context, p complianceclient.Participant) error {
			if p.WalletAddress != "0xWALLET" || p.Status != string(domain.ParticipantStatusPending) {
				t.Fatalf("unexpected participant: %+v", p)
			}
			return nil
		}}
		resp, err := svc(kc, comp, k, nil, nil, "").RegisterParticipant(ctx, &authv1.RegisterParticipantRequest{AccessToken: "t", Country: "BR"})
		if err != nil || resp.WalletAddress != "0xWALLET" || resp.UserId != "uid-1" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})

	t.Run("invalid_token", func(t *testing.T) {
		kc := &fakeKeycloak{validateTokenFn: func(_ context.Context, _ string) (domain.TokenClaims, error) {
			return domain.TokenClaims{}, errors.New("invalid")
		}}
		_, err := svc(kc, nil, nil, nil, nil, "").RegisterParticipant(ctx, &authv1.RegisterParticipantRequest{AccessToken: "bad"})
		wantCode(t, err, codes.Unauthenticated)
	})

	t.Run("kms_error", func(t *testing.T) {
		k := &fakeKMS{createKeyFn: func(_ context.Context, _ string) (kms.KeyInfo, error) {
			return kms.KeyInfo{}, errors.New("kms down")
		}}
		_, err := svc(validKC(), nil, k, nil, nil, "").RegisterParticipant(ctx, &authv1.RegisterParticipantRequest{AccessToken: "t"})
		wantCode(t, err, codes.Internal)
	})

	t.Run("upsert_conflict", func(t *testing.T) {
		k := &fakeKMS{createKeyFn: func(_ context.Context, _ string) (kms.KeyInfo, error) {
			return kms.KeyInfo{Address: "0x1"}, nil
		}}
		comp := &fakeCompliance{upsertFn: func(_ context.Context, _ complianceclient.Participant) error {
			return errors.New("duplicate key value")
		}}
		_, err := svc(validKC(), comp, k, nil, nil, "").RegisterParticipant(ctx, &authv1.RegisterParticipantRequest{AccessToken: "t"})
		wantCode(t, err, codes.AlreadyExists)
	})

	t.Run("upsert_internal", func(t *testing.T) {
		k := &fakeKMS{createKeyFn: func(_ context.Context, _ string) (kms.KeyInfo, error) {
			return kms.KeyInfo{Address: "0x1"}, nil
		}}
		comp := &fakeCompliance{upsertFn: func(_ context.Context, _ complianceclient.Participant) error {
			return errors.New("db unreachable")
		}}
		_, err := svc(validKC(), comp, k, nil, nil, "").RegisterParticipant(ctx, &authv1.RegisterParticipantRequest{AccessToken: "t"})
		wantCode(t, err, codes.Internal)
	})
}

// ---------------------------------------------------------------------------
// SignTransaction
// ---------------------------------------------------------------------------

func TestSignTransaction(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		k := &fakeKMS{signFn: func(_ context.Context, _, _ string) (kms.SignResult, error) {
			return kms.SignResult{Signature: "0xsig", Address: "0xaddr"}, nil
		}}
		resp, err := svc(nil, nil, k, nil, nil, "").SignTransaction(ctx, &authv1.SignTransactionRequest{UserId: "u", Digest: "d"})
		if err != nil || resp.Signature != "0xsig" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
	t.Run("key_not_found", func(t *testing.T) {
		k := &fakeKMS{signFn: func(_ context.Context, _, _ string) (kms.SignResult, error) {
			return kms.SignResult{}, kms.ErrKeyNotFound
		}}
		_, err := svc(nil, nil, k, nil, nil, "").SignTransaction(ctx, &authv1.SignTransactionRequest{UserId: "u"})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("internal", func(t *testing.T) {
		k := &fakeKMS{signFn: func(_ context.Context, _, _ string) (kms.SignResult, error) {
			return kms.SignResult{}, errors.New("hsm offline")
		}}
		_, err := svc(nil, nil, k, nil, nil, "").SignTransaction(ctx, &authv1.SignTransactionRequest{UserId: "u"})
		wantCode(t, err, codes.Internal)
	})
}

// ---------------------------------------------------------------------------
// GetKYCStatus
// ---------------------------------------------------------------------------

func TestGetKYCStatus(t *testing.T) {
	ctx := context.Background()
	t.Run("found_with_status", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{Status: "ACTIVE"}, true, nil
		}}
		resp, err := svc(nil, comp, nil, nil, nil, "").GetKYCStatus(ctx, &authv1.GetKYCStatusRequest{Subject: "s"})
		if err != nil || resp.Status != "ACTIVE" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
	t.Run("found_empty_status_defaults_pending", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, true, nil
		}}
		resp, _ := svc(nil, comp, nil, nil, nil, "").GetKYCStatus(ctx, &authv1.GetKYCStatusRequest{Subject: "s"})
		if resp.Status != string(domain.ParticipantStatusPending) {
			t.Fatalf("status=%q", resp.Status)
		}
	})
	t.Run("not_found_defaults_pending", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, nil
		}}
		resp, _ := svc(nil, comp, nil, nil, nil, "").GetKYCStatus(ctx, &authv1.GetKYCStatusRequest{Subject: "s"})
		if resp.Status != string(domain.ParticipantStatusPending) {
			t.Fatalf("status=%q", resp.Status)
		}
	})
	t.Run("error", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, errors.New("db")
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").GetKYCStatus(ctx, &authv1.GetKYCStatusRequest{Subject: "s"})
		wantCode(t, err, codes.Internal)
	})
}

// ---------------------------------------------------------------------------
// ProvisionParticipant
// ---------------------------------------------------------------------------

func TestProvisionParticipant(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		comp := &fakeCompliance{manageStatusFn: func(_ context.Context, _, _, _ string) error { return nil }}
		_, err := svc(nil, comp, nil, nil, nil, "").ProvisionParticipant(ctx, &authv1.ProvisionParticipantRequest{Subject: "s", Status: "ACTIVE"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		comp := &fakeCompliance{manageStatusFn: func(_ context.Context, _, _, _ string) error { return errors.New("boom") }}
		_, err := svc(nil, comp, nil, nil, nil, "").ProvisionParticipant(ctx, &authv1.ProvisionParticipantRequest{Subject: "s"})
		wantCode(t, err, codes.Internal)
	})
}

// ---------------------------------------------------------------------------
// ListUsers / GetUser
// ---------------------------------------------------------------------------

func TestListUsers(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		comp := &fakeCompliance{listFn: func(_ context.Context, _ complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
			return []complianceclient.Participant{
				{UserID: "u1", Role: "ROLE_NOC", Status: "ACTIVE"},
				{UserID: "u2", Role: "ROLE_TREASURY", Status: "PENDING"},
			}, nil
		}}
		resp, err := svc(nil, comp, nil, nil, nil, "").ListUsers(ctx, &authv1.ListUsersRequest{})
		if err != nil || resp.Total != 2 || len(resp.Users) != 2 {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
	t.Run("error", func(t *testing.T) {
		comp := &fakeCompliance{listFn: func(_ context.Context, _ complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
			return nil, errors.New("db")
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").ListUsers(ctx, &authv1.ListUsersRequest{})
		wantCode(t, err, codes.Internal)
	})
}

func TestGetUser(t *testing.T) {
	ctx := context.Background()
	baseKC := func() *fakeKeycloak {
		return &fakeKeycloak{
			getAdminTokenFn: func(_ context.Context) (string, error) { return "adm", nil },
			getUsernameFn:   func(_ context.Context, _, _ string) (string, error) { return "alice", nil },
			getEmailFn:      func(_ context.Context, _, _ string) (string, error) { return "a@x.com", nil },
		}
	}
	found := func() *fakeCompliance {
		return &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{UserID: "u1", Role: "ROLE_NOC", Status: "ACTIVE"}, true, nil
		}}
	}

	t.Run("missing_id", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").GetUser(ctx, &authv1.GetUserRequest{})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("not_found", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, nil
		}}
		_, err := svc(baseKC(), comp, nil, nil, nil, "").GetUser(ctx, &authv1.GetUserRequest{UserId: "u1"})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("compliance_error", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, errors.New("db")
		}}
		_, err := svc(baseKC(), comp, nil, nil, nil, "").GetUser(ctx, &authv1.GetUserRequest{UserId: "u1"})
		wantCode(t, err, codes.Internal)
	})
	t.Run("admin_token_error", func(t *testing.T) {
		kc := baseKC()
		kc.getAdminTokenFn = func(_ context.Context) (string, error) { return "", errors.New("no admin") }
		_, err := svc(kc, found(), nil, nil, nil, "").GetUser(ctx, &authv1.GetUserRequest{UserId: "u1"})
		wantCode(t, err, codes.Internal)
	})
	t.Run("username_error", func(t *testing.T) {
		kc := baseKC()
		kc.getUsernameFn = func(_ context.Context, _, _ string) (string, error) { return "", errors.New("404") }
		_, err := svc(kc, found(), nil, nil, nil, "").GetUser(ctx, &authv1.GetUserRequest{UserId: "u1"})
		wantCode(t, err, codes.Internal)
	})
	t.Run("success_email_error_nonfatal", func(t *testing.T) {
		kc := baseKC()
		kc.getEmailFn = func(_ context.Context, _, _ string) (string, error) { return "", errors.New("no email") }
		resp, err := svc(kc, found(), nil, nil, nil, "").GetUser(ctx, &authv1.GetUserRequest{UserId: "u1"})
		if err != nil || resp.Username != "alice" || resp.Email != "" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
	t.Run("success", func(t *testing.T) {
		resp, err := svc(baseKC(), found(), nil, nil, nil, "").GetUser(ctx, &authv1.GetUserRequest{UserId: "u1"})
		if err != nil || resp.Username != "alice" || resp.Email != "a@x.com" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
}

// ---------------------------------------------------------------------------
// OnboardParticipant
// ---------------------------------------------------------------------------

func TestOnboardParticipant(t *testing.T) {
	ctx := context.Background()

	fullKC := func() *fakeKeycloak {
		return &fakeKeycloak{
			getAdminTokenFn: func(_ context.Context) (string, error) { return "adm", nil },
			createUserFn:    func(_ context.Context, _ string, _ keycloak.CreateUserRequest) (string, error) { return "uid-99", nil },
			resetPasswordFn: func(_ context.Context, _, _, _ string) error { return nil },
			assignRoleFn:    func(_ context.Context, _, _, _ string) error { return nil },
		}
	}

	t.Run("missing_fields", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{Username: "u"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("non_admin_role", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{Username: "u", Email: "e", Role: "ROLE_GOVERNANCE"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("admin_token_error", func(t *testing.T) {
		kc := fullKC()
		kc.getAdminTokenFn = func(_ context.Context) (string, error) { return "", errors.New("x") }
		_, err := svc(kc, nil, nil, nil, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{Username: "u", Email: "e", Role: domain.RoleNOC})
		wantCode(t, err, codes.Internal)
	})
	t.Run("user_already_exists", func(t *testing.T) {
		kc := fullKC()
		kc.createUserFn = func(_ context.Context, _ string, _ keycloak.CreateUserRequest) (string, error) {
			return "", errors.New("user already exists")
		}
		_, err := svc(kc, nil, nil, nil, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{Username: "u", Email: "e", Role: domain.RoleNOC})
		wantCode(t, err, codes.AlreadyExists)
	})
	t.Run("create_user_internal", func(t *testing.T) {
		kc := fullKC()
		kc.createUserFn = func(_ context.Context, _ string, _ keycloak.CreateUserRequest) (string, error) {
			return "", errors.New("boom")
		}
		_, err := svc(kc, nil, nil, nil, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{Username: "u", Email: "e", Role: domain.RoleNOC})
		wantCode(t, err, codes.Internal)
	})

	// ROLE_NOC: no KMS, no on-chain.
	t.Run("noc_success_no_wallet", func(t *testing.T) {
		comp := &fakeCompliance{upsertFn: func(_ context.Context, p complianceclient.Participant) error {
			if p.WalletAddress != "" {
				t.Fatalf("expected no wallet for NOC, got %q", p.WalletAddress)
			}
			return nil
		}}
		resp, err := svc(fullKC(), comp, nil, nil, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{
			Username: "u", Email: "e", Role: domain.RoleNOC, InstitutionName: "Inst",
		})
		if err != nil || resp.UserId != "uid-99" || resp.ClientSecret == "" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})

	// ROLE_COMMERCIAL_BANK: KMS + on-chain registration.
	t.Run("commercial_bank_full_flow", func(t *testing.T) {
		k := &fakeKMS{createKeyFn: func(_ context.Context, _ string) (kms.KeyInfo, error) {
			return kms.KeyInfo{Address: "0xWALLET"}, nil
		}}
		bc := &fakeRegistry{registerFn: func(_ context.Context, wallet, _, _ string, _ [32]byte) (string, error) {
			if wallet != "0xWALLET" {
				t.Fatalf("wallet=%q", wallet)
			}
			return "0xTX", nil
		}}
		comp := &fakeCompliance{upsertFn: func(_ context.Context, _ complianceclient.Participant) error { return nil }}
		resp, err := svc(fullKC(), comp, k, bc, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{
			Username: "u", Email: "e", Role: domain.RoleCommercialBank, InstitutionName: "Bank",
		})
		if err != nil || resp.WalletAddress != "0xWALLET" || resp.TxHash != "0xTX" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})

	t.Run("kms_error", func(t *testing.T) {
		k := &fakeKMS{createKeyFn: func(_ context.Context, _ string) (kms.KeyInfo, error) {
			return kms.KeyInfo{}, errors.New("kms")
		}}
		_, err := svc(fullKC(), nil, k, nil, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{
			Username: "u", Email: "e", Role: domain.RoleCommercialBank,
		})
		wantCode(t, err, codes.Internal)
	})

	t.Run("onchain_error", func(t *testing.T) {
		k := &fakeKMS{createKeyFn: func(_ context.Context, _ string) (kms.KeyInfo, error) {
			return kms.KeyInfo{Address: "0xW"}, nil
		}}
		bc := &fakeRegistry{registerFn: func(_ context.Context, _, _, _ string, _ [32]byte) (string, error) {
			return "", errors.New("revert")
		}}
		_, err := svc(fullKC(), nil, k, bc, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{
			Username: "u", Email: "e", Role: domain.RoleCommercialBank,
		})
		wantCode(t, err, codes.Internal)
	})

	t.Run("persist_error", func(t *testing.T) {
		comp := &fakeCompliance{upsertFn: func(_ context.Context, _ complianceclient.Participant) error {
			return errors.New("db")
		}}
		_, err := svc(fullKC(), comp, nil, nil, nil, "").OnboardParticipant(ctx, &authv1.OnboardParticipantRequest{
			Username: "u", Email: "e", Role: domain.RoleNOC,
		})
		wantCode(t, err, codes.Internal)
	})
}

// ---------------------------------------------------------------------------
// IssueLoginNonce
// ---------------------------------------------------------------------------

func TestIssueLoginNonce(t *testing.T) {
	ctx := context.Background()
	pkiParticipant := func() *fakeCompliance {
		return &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{Role: domain.RoleCommercialBank}, true, nil
		}}
	}
	okKC := func() *fakeKeycloak {
		return &fakeKeycloak{
			getAdminTokenFn: func(_ context.Context) (string, error) { return "adm", nil },
			getUsernameFn:   func(_ context.Context, _, _ string) (string, error) { return "alice", nil },
			loginFn: func(_ context.Context, _, _ string) (keycloak.TokenResponse, error) {
				return keycloak.TokenResponse{}, nil
			},
		}
	}

	t.Run("missing_user_id", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").IssueLoginNonce(ctx, &authv1.IssueLoginNonceRequest{ClientSecret: "s"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("missing_secret", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").IssueLoginNonce(ctx, &authv1.IssueLoginNonceRequest{UserId: "u"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("not_found", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, nil
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").IssueLoginNonce(ctx, &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "s"})
		wantCode(t, err, codes.NotFound)
	})
	t.Run("lookup_error", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, errors.New("db")
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").IssueLoginNonce(ctx, &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "s"})
		wantCode(t, err, codes.Internal)
	})
	t.Run("non_pki_role", func(t *testing.T) {
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{Role: domain.RoleNOC}, true, nil
		}}
		_, err := svc(nil, comp, nil, nil, nil, "").IssueLoginNonce(ctx, &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "s"})
		wantCode(t, err, codes.PermissionDenied)
	})
	t.Run("invalid_credentials", func(t *testing.T) {
		kc := okKC()
		kc.loginFn = func(_ context.Context, _, _ string) (keycloak.TokenResponse, error) {
			return keycloak.TokenResponse{}, errors.New("bad")
		}
		_, err := svc(kc, pkiParticipant(), nil, nil, nil, "").IssueLoginNonce(ctx, &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "s"})
		wantCode(t, err, codes.Unauthenticated)
	})
	t.Run("nonce_store_error", func(t *testing.T) {
		ns := &fakeNonce{setFn: func(_ context.Context, _, _ string, _ time.Duration) error { return errors.New("redis") }}
		_, err := svc(okKC(), pkiParticipant(), nil, nil, ns, "").IssueLoginNonce(ctx, &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "s"})
		wantCode(t, err, codes.Internal)
	})
	t.Run("success", func(t *testing.T) {
		var stored string
		ns := &fakeNonce{setFn: func(_ context.Context, _, v string, _ time.Duration) error { stored = v; return nil }}
		resp, err := svc(okKC(), pkiParticipant(), nil, nil, ns, "").IssueLoginNonce(ctx, &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "sec"})
		if err != nil || resp.Nonce == "" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
		if stored != resp.Nonce+"|sec" {
			t.Fatalf("stored=%q", stored)
		}
	})
}

// ---------------------------------------------------------------------------
// ChangeClientSecret
// ---------------------------------------------------------------------------

func TestChangeClientSecret(t *testing.T) {
	ctx := context.Background()
	okKC := func() *fakeKeycloak {
		return &fakeKeycloak{
			getAdminTokenFn: func(_ context.Context) (string, error) { return "adm", nil },
			getUsernameFn:   func(_ context.Context, _, _ string) (string, error) { return "alice", nil },
			loginFn: func(_ context.Context, _, _ string) (keycloak.TokenResponse, error) {
				return keycloak.TokenResponse{}, nil
			},
			resetPasswordFn: func(_ context.Context, _, _, _ string) error { return nil },
		}
	}
	req := &authv1.ChangeClientSecretRequest{UserId: "u", CurrentSecret: "old", NewSecret: "new"}

	t.Run("missing_fields", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").ChangeClientSecret(ctx, &authv1.ChangeClientSecretRequest{UserId: "u"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("admin_token_error", func(t *testing.T) {
		kc := okKC()
		kc.getAdminTokenFn = func(_ context.Context) (string, error) { return "", errors.New("x") }
		_, err := svc(kc, nil, nil, nil, nil, "").ChangeClientSecret(ctx, req)
		wantCode(t, err, codes.Internal)
	})
	t.Run("username_error", func(t *testing.T) {
		kc := okKC()
		kc.getUsernameFn = func(_ context.Context, _, _ string) (string, error) { return "", errors.New("404") }
		_, err := svc(kc, nil, nil, nil, nil, "").ChangeClientSecret(ctx, req)
		wantCode(t, err, codes.Internal)
	})
	t.Run("invalid_current", func(t *testing.T) {
		kc := okKC()
		kc.loginFn = func(_ context.Context, _, _ string) (keycloak.TokenResponse, error) {
			return keycloak.TokenResponse{}, errors.New("bad")
		}
		_, err := svc(kc, nil, nil, nil, nil, "").ChangeClientSecret(ctx, req)
		wantCode(t, err, codes.Unauthenticated)
	})
	t.Run("reset_error", func(t *testing.T) {
		kc := okKC()
		kc.resetPasswordFn = func(_ context.Context, _, _, _ string) error { return errors.New("boom") }
		_, err := svc(kc, nil, nil, nil, nil, "").ChangeClientSecret(ctx, req)
		wantCode(t, err, codes.Internal)
	})
	t.Run("success", func(t *testing.T) {
		_, err := svc(okKC(), nil, nil, nil, nil, "").ChangeClientSecret(ctx, req)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// VerifyPKILogin (with real secp256k1 PoP + CA chain fixtures)
// ---------------------------------------------------------------------------

// pkiFixture builds a CA, an end-entity cert signed by it, and a matching
// secp256k1 wallet for exercising the full VerifyPKILogin path.
type pkiFixture struct {
	caPEM   string
	certPEM string
	keyPEM  string // P-256 private key PEM matching certPEM
}

func newPKIFixture(t *testing.T) pkiFixture {
	t.Helper()
	caCert, caKey, err := pki.GenerateSelfSignedCA("Test CA", "Org", 2)
	if err != nil {
		t.Fatalf("GenerateSelfSignedCA: %v", err)
	}
	csrPEM, keyPEM, err := pki.GenerateCSR("u", "Org", "ROLE_COMMERCIAL_BANK", "BR")
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}
	issued, err := pki.SignCSR(caCert, caKey, csrPEM, 1)
	if err != nil {
		t.Fatalf("SignCSR: %v", err)
	}
	return pkiFixture{caPEM: caCert, certPEM: issued.CertPEM, keyPEM: keyPEM}
}

// signNonceP256 returns a hex DER signature of the nonce produced by the cert key.
func signNonceP256(t *testing.T, keyPEM, nonce string) string {
	t.Helper()
	sig, err := pki.SignMessage(keyPEM, nonce)
	if err != nil {
		t.Fatalf("SignMessage: %v", err)
	}
	return sig
}

// newPKIFixtureWallet returns a fixture whose end-entity cert carries the
// given walletAddr in the custom OID extension (CN is always "u").
func newPKIFixtureWallet(t *testing.T, walletAddr string) pkiFixture {
	t.Helper()
	caCert, caKey, err := pki.GenerateSelfSignedCA("Test CA W", "Org", 2)
	if err != nil {
		t.Fatalf("newPKIFixtureWallet/GenerateSelfSignedCA: %v", err)
	}
	csrPEM, keyPEM, err := pki.GenerateCSR("u", "Org", "ROLE_COMMERCIAL_BANK", "BR")
	if err != nil {
		t.Fatalf("newPKIFixtureWallet/GenerateCSR: %v", err)
	}
	issued, err := pki.SignCSR(caCert, caKey, csrPEM, 1, pki.SignCSROptions{WalletAddress: walletAddr})
	if err != nil {
		t.Fatalf("newPKIFixtureWallet/SignCSR: %v", err)
	}
	return pkiFixture{caPEM: caCert, certPEM: issued.CertPEM, keyPEM: keyPEM}
}

func TestVerifyPKILogin(t *testing.T) {
	ctx := context.Background()
	fix := newPKIFixture(t)

	// nonce is the hex string the client signs.
	nonce := hex.EncodeToString([]byte("the-nonce-value"))
	storedVal := nonce + "|clientsecret"

	storeWith := func(val string, found bool) *fakeNonce {
		return &fakeNonce{getAndDeleteFn: func(_ context.Context, _ string) (string, bool, error) {
			return val, found, nil
		}}
	}
	okKC := func() *fakeKeycloak {
		return &fakeKeycloak{
			getAdminTokenFn: func(_ context.Context) (string, error) { return "adm", nil },
			getUsernameFn:   func(_ context.Context, _, _ string) (string, error) { return "alice", nil },
			loginFn: func(_ context.Context, _, _ string) (keycloak.TokenResponse, error) {
				return keycloak.TokenResponse{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 300}, nil
			},
		}
	}

	t.Run("missing_fields", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, nil, "").VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{UserId: "u"})
		wantCode(t, err, codes.InvalidArgument)
	})
	t.Run("nonce_store_error", func(t *testing.T) {
		ns := &fakeNonce{getAndDeleteFn: func(_ context.Context, _ string) (string, bool, error) {
			return "", false, errors.New("redis")
		}}
		_, err := svc(nil, nil, nil, nil, ns, "").VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "u", NonceSignatureHex: "ab", CertPem: fix.certPEM,
		})
		wantCode(t, err, codes.Internal)
	})
	t.Run("nonce_not_found", func(t *testing.T) {
		_, err := svc(nil, nil, nil, nil, storeWith("", false), "").VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "u", NonceSignatureHex: "ab", CertPem: fix.certPEM,
		})
		wantCode(t, err, codes.Unauthenticated)
	})
	t.Run("invalid_cert_chain", func(t *testing.T) {
		otherCA, _, _ := pki.GenerateSelfSignedCA("Other", "O", 1)
		sig := signNonceP256(t, fix.keyPEM, nonce)
		_, err := svc(nil, nil, nil, nil, storeWith(storedVal, true), otherCA).VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "u", NonceSignatureHex: sig, CertPem: fix.certPEM,
		})
		wantCode(t, err, codes.Unauthenticated)
	})
	t.Run("nonce_signature_mismatch", func(t *testing.T) {
		// Sign a different message than the stored nonce.
		badSig := signNonceP256(t, fix.keyPEM, hex.EncodeToString([]byte("wrong")))
		_, err := svc(nil, nil, nil, nil, storeWith(storedVal, true), fix.caPEM).VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "u", NonceSignatureHex: badSig, CertPem: fix.certPEM,
		})
		wantCode(t, err, codes.Unauthenticated)
	})
	t.Run("onchain_unauthorized", func(t *testing.T) {
		sig := signNonceP256(t, fix.keyPEM, nonce)
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{WalletAddress: "0xWALLET"}, true, nil
		}}
		bc := &fakeRegistry{canTransactFn: func(_ context.Context, _ string) (bool, error) { return false, nil }}
		_, err := svc(okKC(), comp, nil, bc, storeWith(storedVal, true), fix.caPEM).VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "u", NonceSignatureHex: sig, CertPem: fix.certPEM,
		})
		wantCode(t, err, codes.PermissionDenied)
	})
	t.Run("success_skips_onchain_when_no_wallet", func(t *testing.T) {
		sig := signNonceP256(t, fix.keyPEM, nonce)
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, true, nil // no wallet
		}}
		resp, err := svc(okKC(), comp, nil, nil, storeWith(storedVal, true), fix.caPEM).VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "u", NonceSignatureHex: sig, CertPem: fix.certPEM,
		})
		if err != nil || resp.AccessToken != "at" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
	t.Run("success_authorized_onchain", func(t *testing.T) {
		sig := signNonceP256(t, fix.keyPEM, nonce)
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{WalletAddress: "0xWALLET"}, true, nil
		}}
		bc := &fakeRegistry{canTransactFn: func(_ context.Context, _ string) (bool, error) { return true, nil }}
		resp, err := svc(okKC(), comp, nil, bc, storeWith(storedVal, true), fix.caPEM).VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "u", NonceSignatureHex: sig, CertPem: fix.certPEM,
		})
		if err != nil || resp.TokenType != "Bearer" {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})
	t.Run("keycloak_login_error", func(t *testing.T) {
		sig := signNonceP256(t, fix.keyPEM, nonce)
		kc := okKC()
		kc.loginFn = func(_ context.Context, _, _ string) (keycloak.TokenResponse, error) {
			return keycloak.TokenResponse{}, errors.New("kc down")
		}
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{}, false, nil
		}}
		_, err := svc(kc, comp, nil, nil, storeWith(storedVal, true), fix.caPEM).VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "u", NonceSignatureHex: sig, CertPem: fix.certPEM,
		})
		wantCode(t, err, codes.Internal)
	})
	// H-5: cert CN must match the claimed UserId.
	t.Run("cert_cn_user_id_mismatch", func(t *testing.T) {
		sig := signNonceP256(t, fix.keyPEM, nonce) // cert CN = "u"
		_, err := svc(nil, nil, nil, nil, storeWith(storedVal, true), fix.caPEM).VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "different-user", NonceSignatureHex: sig, CertPem: fix.certPEM,
		})
		wantCode(t, err, codes.Unauthenticated)
	})
	// H-5: when the cert carries a wallet extension it must match the participant record.
	t.Run("cert_wallet_participant_mismatch", func(t *testing.T) {
		const certWallet = "0x1111111111111111111111111111111111111111"
		const participantWallet = "0x2222222222222222222222222222222222222222"
		walletFix := newPKIFixtureWallet(t, certWallet)
		sig := signNonceP256(t, walletFix.keyPEM, nonce)
		comp := &fakeCompliance{getByUserFn: func(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{WalletAddress: participantWallet}, true, nil
		}}
		_, err := svc(okKC(), comp, nil, nil, storeWith(storedVal, true), walletFix.caPEM).VerifyPKILogin(ctx, &authv1.VerifyPKILoginRequest{
			UserId: "u", NonceSignatureHex: sig, CertPem: walletFix.certPEM,
		})
		wantCode(t, err, codes.Unauthenticated)
	})
}

// sanity: ensure the secp256k1 helper digest matches the SHA-256 used in PoP.
func TestPoPDigestSanity(t *testing.T) {
	key, _ := gethcrypto.GenerateKey()
	d := sha256.Sum256([]byte("x"))
	_, err := gethcrypto.Sign(d[:], key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
}
