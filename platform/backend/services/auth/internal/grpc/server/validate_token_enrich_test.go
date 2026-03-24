package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
)

// ---------------------------------------------------------------------------
// Minimal mocks
// ---------------------------------------------------------------------------

// mockKeycloakValidate stubs keycloak.Client for ValidateToken tests.
type mockKeycloakValidate struct {
	claims domain.TokenClaims
	err    error
}

func (m *mockKeycloakValidate) Login(ctx context.Context, u, p string) (keycloak.TokenResponse, error) {
	return keycloak.TokenResponse{}, errors.New("unexpected")
}
func (m *mockKeycloakValidate) Refresh(ctx context.Context, rt string) (keycloak.TokenResponse, error) {
	return keycloak.TokenResponse{}, errors.New("unexpected")
}
func (m *mockKeycloakValidate) Logout(ctx context.Context, rt string) error {
	return errors.New("unexpected")
}
func (m *mockKeycloakValidate) ValidateToken(_ context.Context, _ string) (domain.TokenClaims, error) {
	return m.claims, m.err
}
func (m *mockKeycloakValidate) CreateUser(ctx context.Context, _ string, _ keycloak.CreateUserRequest) (string, error) {
	return "", errors.New("unexpected")
}
func (m *mockKeycloakValidate) GetAdminToken(ctx context.Context) (string, error) {
	return "", errors.New("unexpected")
}
func (m *mockKeycloakValidate) UpdateUsername(ctx context.Context, _, _, _ string) error {
	return errors.New("unexpected")
}
func (m *mockKeycloakValidate) ResetPassword(ctx context.Context, _, _, _ string) error {
	return errors.New("unexpected")
}
func (m *mockKeycloakValidate) GetUserUsername(ctx context.Context, _, _ string) (string, error) {
	return "", errors.New("unexpected")
}
func (m *mockKeycloakValidate) GetUserEmail(ctx context.Context, _, _ string) (string, error) {
	return "", errors.New("unexpected")
}
func (m *mockKeycloakValidate) AssignRealmRole(ctx context.Context, _, _, _ string) error {
	return nil
}

// mockComplianceValidate stubs complianceclient.Client for ValidateToken tests.
type mockComplianceValidate struct {
	participant complianceclient.Participant
	found       bool
	err         error
}

func (m *mockComplianceValidate) UpsertParticipant(_ context.Context, _ complianceclient.Participant) error {
	return errors.New("unexpected")
}
func (m *mockComplianceValidate) GetParticipantByUser(_ context.Context, _ string) (complianceclient.Participant, bool, error) {
	return m.participant, m.found, m.err
}
func (m *mockComplianceValidate) ListParticipants(_ context.Context, _ complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
	return nil, errors.New("unexpected")
}
func (m *mockComplianceValidate) CreateAuditLog(_ context.Context, _ complianceclient.AuditEntry) error {
	return nil
}
func (m *mockComplianceValidate) IssueParticipantCertificate(_ context.Context, _, _, _, _ string) (complianceclient.IssuedCertificate, error) {
	return complianceclient.IssuedCertificate{}, errors.New("unexpected")
}
func (m *mockComplianceValidate) ManageParticipantStatus(_ context.Context, _, _, _ string) error {
	return errors.New("unexpected")
}

// stubKMS satisfies kms.Provider (unused in ValidateToken path).
type stubKMS struct{}

func (s *stubKMS) Name() string { return "stub" }
func (s *stubKMS) CreateKey(_ context.Context, _ string) (kms.KeyInfo, error) {
	return kms.KeyInfo{}, nil
}
func (s *stubKMS) Sign(_ context.Context, _, _ string) (kms.SignResult, error) {
	return kms.SignResult{}, nil
}
func (s *stubKMS) GetAddress(_ context.Context, _ string) (string, error) { return "", nil }
func (s *stubKMS) DeleteKey(_ context.Context, _ string) error            { return nil }

// stubNonce satisfies noncestore.NonceStore (unused in ValidateToken path).
type stubNonce struct{}

func (s *stubNonce) Set(_ context.Context, _, _ string, _ time.Duration) error { return nil }
func (s *stubNonce) GetAndDelete(_ context.Context, _ string) (string, bool, error) {
	return "", false, nil
}

// stubRegistry satisfies blockchainRegistry (unused in ValidateToken path).
type stubRegistry struct{}

func (s *stubRegistry) SetParticipant(_ context.Context, _, _ string, _ bool) (string, error) {
	return "", nil
}
func (s *stubRegistry) IsMemberAuthorized(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (s *stubRegistry) GetMemberRole(_ context.Context, _ string) (uint8, error) { return 0, nil }

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestIdentityService(kc *mockKeycloakValidate, comp *mockComplianceValidate) *identityService {
	return &identityService{
		keycloak:         kc,
		kms:              &stubKMS{},
		compliance:       comp,
		blockchainClient: &stubRegistry{},
		nonceStore:       &stubNonce{},
	}
}

// ---------------------------------------------------------------------------
// TestValidateToken_Enrichment
// ---------------------------------------------------------------------------

func TestValidateToken_Enrichment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("enriches_role_when_jwt_empty", func(t *testing.T) {
		kc := &mockKeycloakValidate{claims: domain.TokenClaims{Subject: "uid-1", Roles: nil}}
		comp := &mockComplianceValidate{
			participant: complianceclient.Participant{UserID: "uid-1", Role: "ROLE_NOC", CountryCode: "BR"},
			found:       true,
		}
		svc := newTestIdentityService(kc, comp)
		resp, err := svc.ValidateToken(ctx, &authv1.ValidateTokenRequest{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Roles) != 1 || resp.Roles[0] != "ROLE_NOC" {
			t.Errorf("roles: got %v, want [ROLE_NOC]", resp.Roles)
		}
		if resp.Country != "BR" {
			t.Errorf("country: got %q, want BR", resp.Country)
		}
	})

	// JWT carries Keycloak default roles; compliance role must be appended.
	t.Run("appends_compliance_role_to_jwt_default_roles", func(t *testing.T) {
		defaultRoles := []string{"default-roles-cbweb3-spoke-a", "offline_access", "uma_authorization"}
		kc := &mockKeycloakValidate{claims: domain.TokenClaims{Subject: "uid-2", Roles: defaultRoles}}
		comp := &mockComplianceValidate{
			participant: complianceclient.Participant{UserID: "uid-2", Role: "ROLE_NOC"},
			found:       true,
		}
		svc := newTestIdentityService(kc, comp)
		resp, err := svc.ValidateToken(ctx, &authv1.ValidateTokenRequest{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !containsRoleSlice(resp.Roles, "ROLE_NOC") {
			t.Errorf("ROLE_NOC should be appended; got %v", resp.Roles)
		}
		if len(resp.Roles) != len(defaultRoles)+1 {
			t.Errorf("total roles: got %d, want %d; roles=%v", len(resp.Roles), len(defaultRoles)+1, resp.Roles)
		}
	})

	// Compliance role already present in JWT — must not be duplicated.
	t.Run("does_not_duplicate_role_already_in_jwt", func(t *testing.T) {
		kc := &mockKeycloakValidate{claims: domain.TokenClaims{Subject: "uid-2b", Roles: []string{"ROLE_NOC"}}}
		comp := &mockComplianceValidate{
			participant: complianceclient.Participant{UserID: "uid-2b", Role: "ROLE_NOC"},
			found:       true,
		}
		svc := newTestIdentityService(kc, comp)
		resp, err := svc.ValidateToken(ctx, &authv1.ValidateTokenRequest{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count := 0
		for _, r := range resp.Roles {
			if r == "ROLE_NOC" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("ROLE_NOC appears %d times (want 1); roles=%v", count, resp.Roles)
		}
	})

	t.Run("enriches_wallet_and_country_and_bankid", func(t *testing.T) {
		kc := &mockKeycloakValidate{claims: domain.TokenClaims{Subject: "uid-3", Roles: nil}}
		comp := &mockComplianceValidate{
			participant: complianceclient.Participant{
				UserID:        "uid-3",
				Role:          "ROLE_COMMERCIAL_BANK",
				WalletAddress: "0xABCD",
				CountryCode:   "BR",
				BankCode:      "001",
			},
			found: true,
		}
		svc := newTestIdentityService(kc, comp)
		resp, err := svc.ValidateToken(ctx, &authv1.ValidateTokenRequest{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Wallet != "0xABCD" {
			t.Errorf("wallet: got %q, want 0xABCD", resp.Wallet)
		}
		if resp.Country != "BR" {
			t.Errorf("country: got %q, want BR", resp.Country)
		}
		if resp.BankId != "001" {
			t.Errorf("bankId: got %q, want 001", resp.BankId)
		}
	})

	t.Run("does_not_overwrite_existing_wallet", func(t *testing.T) {
		kc := &mockKeycloakValidate{claims: domain.TokenClaims{Subject: "uid-4", Wallet: "0xJWT"}}
		comp := &mockComplianceValidate{
			participant: complianceclient.Participant{UserID: "uid-4", WalletAddress: "0xCOMP"},
			found:       true,
		}
		svc := newTestIdentityService(kc, comp)
		resp, err := svc.ValidateToken(ctx, &authv1.ValidateTokenRequest{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Wallet != "0xJWT" {
			t.Errorf("wallet should remain 0xJWT (from JWT), got %q", resp.Wallet)
		}
	})

	t.Run("compliance_error_does_not_fail_validation", func(t *testing.T) {
		kc := &mockKeycloakValidate{claims: domain.TokenClaims{Subject: "uid-5", Roles: []string{"ROLE_NOC"}}}
		comp := &mockComplianceValidate{err: errors.New("connection refused")}
		svc := newTestIdentityService(kc, comp)
		resp, err := svc.ValidateToken(ctx, &authv1.ValidateTokenRequest{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("compliance error must not reject token: %v", err)
		}
		if len(resp.Roles) != 1 || resp.Roles[0] != "ROLE_NOC" {
			t.Errorf("roles should stay from JWT, got %v", resp.Roles)
		}
	})

	t.Run("keycloak_error_returns_unauthenticated", func(t *testing.T) {
		kc := &mockKeycloakValidate{err: errors.New("invalid token")}
		comp := &mockComplianceValidate{}
		svc := newTestIdentityService(kc, comp)
		_, err := svc.ValidateToken(ctx, &authv1.ValidateTokenRequest{AccessToken: "bad"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
