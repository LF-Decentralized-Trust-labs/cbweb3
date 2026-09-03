// SPDX-License-Identifier: Apache-2.0

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
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
	pki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── configurable fakes (func fields; zero-value = "not expected") ─────────────

type fakeKeycloak struct {
	loginFn       func(ctx context.Context, u, p string) (keycloak.TokenResponse, error)
	refreshFn     func(ctx context.Context, rt string) (keycloak.TokenResponse, error)
	logoutFn      func(ctx context.Context, rt string) error
	validateFn    func(ctx context.Context, t string) (domain.TokenClaims, error)
	createUserFn  func(ctx context.Context, adm string, r keycloak.CreateUserRequest) (string, error)
	adminTokenFn  func(ctx context.Context) (string, error)
	resetPwFn     func(ctx context.Context, adm, uid, pw string) error
	getUsernameFn func(ctx context.Context, adm, uid string) (string, error)
	getEmailFn    func(ctx context.Context, adm, uid string) (string, error)
	assignRoleFn  func(ctx context.Context, adm, uid, role string) error
}

func (f *fakeKeycloak) Login(ctx context.Context, u, p string) (keycloak.TokenResponse, error) {
	return f.loginFn(ctx, u, p)
}
func (f *fakeKeycloak) Refresh(ctx context.Context, rt string) (keycloak.TokenResponse, error) {
	return f.refreshFn(ctx, rt)
}
func (f *fakeKeycloak) Logout(ctx context.Context, rt string) error { return f.logoutFn(ctx, rt) }
func (f *fakeKeycloak) ValidateToken(ctx context.Context, t string) (domain.TokenClaims, error) {
	return f.validateFn(ctx, t)
}
func (f *fakeKeycloak) CreateUser(ctx context.Context, adm string, r keycloak.CreateUserRequest) (string, error) {
	return f.createUserFn(ctx, adm, r)
}
func (f *fakeKeycloak) GetAdminToken(ctx context.Context) (string, error) {
	return f.adminTokenFn(ctx)
}
func (f *fakeKeycloak) UpdateUsername(ctx context.Context, adm, uid, n string) error { return nil }
func (f *fakeKeycloak) ResetPassword(ctx context.Context, adm, uid, pw string) error {
	return f.resetPwFn(ctx, adm, uid, pw)
}
func (f *fakeKeycloak) GetUserUsername(ctx context.Context, adm, uid string) (string, error) {
	return f.getUsernameFn(ctx, adm, uid)
}
func (f *fakeKeycloak) GetUserEmail(ctx context.Context, adm, uid string) (string, error) {
	return f.getEmailFn(ctx, adm, uid)
}
func (f *fakeKeycloak) AssignRealmRole(ctx context.Context, adm, uid, role string) error {
	if f.assignRoleFn != nil {
		return f.assignRoleFn(ctx, adm, uid, role)
	}
	return nil
}

type fakeCompliance struct {
	upsertFn func(ctx context.Context, p complianceclient.Participant) error
	getFn    func(ctx context.Context, uid string) (complianceclient.Participant, bool, error)
	listFn   func(ctx context.Context, f complianceclient.ParticipantFilter) ([]complianceclient.Participant, error)
	manageFn func(ctx context.Context, sub, st, reason string) error
}

func (f *fakeCompliance) UpsertParticipant(ctx context.Context, p complianceclient.Participant) error {
	return f.upsertFn(ctx, p)
}
func (f *fakeCompliance) GetParticipantByUser(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
	return f.getFn(ctx, uid)
}
func (f *fakeCompliance) ListParticipants(ctx context.Context, fl complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
	return f.listFn(ctx, fl)
}
func (f *fakeCompliance) CreateAuditLog(ctx context.Context, e complianceclient.AuditEntry) error {
	return nil
}
func (f *fakeCompliance) IssueParticipantCertificate(_ context.Context, _, _, _, _ string) (complianceclient.IssuedCertificate, error) {
	return complianceclient.IssuedCertificate{}, errors.New("unexpected")
}
func (f *fakeCompliance) SignParticipantCSR(_ context.Context, _, _, _, _, _ string) (complianceclient.SignedCSR, error) {
	return complianceclient.SignedCSR{}, errors.New("unexpected")
}
func (f *fakeCompliance) ManageParticipantStatus(ctx context.Context, sub, st, reason string) error {
	return f.manageFn(ctx, sub, st, reason)
}

type fakeKMS struct {
	createFn func(ctx context.Context, uid string) (kms.KeyInfo, error)
	signFn   func(ctx context.Context, uid, digest string) (kms.SignResult, error)
}

func (f *fakeKMS) Name() string { return "fake" }
func (f *fakeKMS) CreateKey(ctx context.Context, uid string) (kms.KeyInfo, error) {
	return f.createFn(ctx, uid)
}
func (f *fakeKMS) Sign(ctx context.Context, uid, digest string) (kms.SignResult, error) {
	return f.signFn(ctx, uid, digest)
}
func (f *fakeKMS) GetAddress(_ context.Context, _ string) (string, error) { return "", nil }
func (f *fakeKMS) DeleteKey(_ context.Context, _ string) error            { return nil }

type fakeNonce struct {
	store map[string]string
}

func newFakeNonce() *fakeNonce { return &fakeNonce{store: map[string]string{}} }
func (f *fakeNonce) Set(_ context.Context, k, v string, _ time.Duration) error {
	f.store[k] = v
	return nil
}
func (f *fakeNonce) GetAndDelete(_ context.Context, k string) (string, bool, error) {
	v, ok := f.store[k]
	delete(f.store, k)
	return v, ok, nil
}

type fakeRegistry struct {
	registerFn    func(ctx context.Context, addr, inst, role string, fp, institutionID [32]byte) (string, error)
	verifyFn      func(ctx context.Context, addr string) (string, error)
	canTransactFn func(ctx context.Context, addr string) (bool, error)
}

func (f *fakeRegistry) RegisterParticipant(ctx context.Context, addr, inst, role string, fp, institutionID [32]byte) (string, error) {
	return f.registerFn(ctx, addr, inst, role, fp, institutionID)
}
func (f *fakeRegistry) VerifyParticipant(ctx context.Context, addr string) (string, error) {
	if f.verifyFn != nil {
		return f.verifyFn(ctx, addr)
	}
	return "", nil
}
func (f *fakeRegistry) UpdateStatus(_ context.Context, _ string, _ uint8) (string, error) {
	return "", nil
}
func (f *fakeRegistry) SetCertFingerprint(_ context.Context, _ string, _ [32]byte) (string, error) {
	return "", nil
}
func (f *fakeRegistry) CanTransact(ctx context.Context, addr string) (bool, error) {
	if f.canTransactFn != nil {
		return f.canTransactFn(ctx, addr)
	}
	return true, nil
}
func (f *fakeRegistry) IsWhitelisted(_ context.Context, _ string) (bool, error) { return true, nil }
func (f *fakeRegistry) GetParticipant(_ context.Context, _ string) (registry.OnChainParticipant, error) {
	return registry.OnChainParticipant{}, nil
}
func (f *fakeRegistry) GetInstitutionID(_ context.Context, _ string) ([32]byte, error) {
	return [32]byte{}, nil
}
func (f *fakeRegistry) GetCertFingerprint(_ context.Context, _ string) ([32]byte, error) {
	return [32]byte{}, nil
}

func okToken() keycloak.TokenResponse {
	return keycloak.TokenResponse{AccessToken: "at", RefreshToken: "rt", TokenType: "Bearer", ExpiresIn: 300, RefreshExpiresIn: 600}
}

// ── Login / Refresh / Revoke ──────────────────────────────────────────────────

func TestLogin(t *testing.T) {
	t.Parallel()
	kc := &fakeKeycloak{
		adminTokenFn:  func(ctx context.Context) (string, error) { return "adm", nil },
		getUsernameFn: func(ctx context.Context, a, u string) (string, error) { return "", nil },
		loginFn:       func(ctx context.Context, u, p string) (keycloak.TokenResponse, error) { return okToken(), nil },
	}
	svc := &identityService{keycloak: kc, compliance: &fakeCompliance{}}
	resp, err := svc.Login(context.Background(), &authv1.LoginRequest{User: "noc-user", Password: "pw"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if resp.AccessToken != "at" || resp.ExpiresIn != 300 {
		t.Errorf("unexpected resp %+v", resp)
	}

	// Failure path.
	kc.loginFn = func(ctx context.Context, u, p string) (keycloak.TokenResponse, error) {
		return keycloak.TokenResponse{}, errors.New("bad creds")
	}
	if _, err := svc.Login(context.Background(), &authv1.LoginRequest{User: "noc-user"}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestRefreshToken(t *testing.T) {
	t.Parallel()
	kc := &fakeKeycloak{refreshFn: func(ctx context.Context, rt string) (keycloak.TokenResponse, error) { return okToken(), nil }}
	svc := &identityService{keycloak: kc, compliance: &fakeCompliance{}}
	resp, err := svc.RefreshToken(context.Background(), &authv1.RefreshTokenRequest{RefreshToken: "rt"})
	if err != nil || resp.AccessToken != "at" {
		t.Fatalf("refresh: resp=%+v err=%v", resp, err)
	}

	kc.refreshFn = func(ctx context.Context, rt string) (keycloak.TokenResponse, error) {
		return keycloak.TokenResponse{}, errors.New("expired")
	}
	if _, err := svc.RefreshToken(context.Background(), &authv1.RefreshTokenRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

func TestRevokeToken(t *testing.T) {
	t.Parallel()
	var loggedOut string
	kc := &fakeKeycloak{logoutFn: func(ctx context.Context, rt string) error { loggedOut = rt; return nil }}
	svc := &identityService{keycloak: kc, compliance: &fakeCompliance{}}

	// Prefers refresh token.
	if _, err := svc.RevokeToken(context.Background(), &authv1.RevokeTokenRequest{RefreshToken: "rtok", AccessToken: "atok"}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if loggedOut != "rtok" {
		t.Errorf("expected refresh token used, got %q", loggedOut)
	}

	// Falls back to access token.
	if _, err := svc.RevokeToken(context.Background(), &authv1.RevokeTokenRequest{AccessToken: "atok"}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if loggedOut != "atok" {
		t.Errorf("expected access token fallback, got %q", loggedOut)
	}

	// Error path.
	kc.logoutFn = func(ctx context.Context, rt string) error { return errors.New("boom") }
	if _, err := svc.RevokeToken(context.Background(), &authv1.RevokeTokenRequest{RefreshToken: "x"}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

// ── RegisterParticipant ───────────────────────────────────────────────────────

func TestRegisterParticipant(t *testing.T) {
	t.Parallel()
	base := func() *identityService {
		return &identityService{
			keycloak: &fakeKeycloak{validateFn: func(ctx context.Context, tk string) (domain.TokenClaims, error) {
				return domain.TokenClaims{Subject: "uid-1"}, nil
			}},
			kms: &fakeKMS{createFn: func(ctx context.Context, uid string) (kms.KeyInfo, error) {
				return kms.KeyInfo{Address: "0xwallet"}, nil
			}},
			compliance: &fakeCompliance{upsertFn: func(ctx context.Context, p complianceclient.Participant) error { return nil }},
		}
	}
	resp, err := base().RegisterParticipant(context.Background(), &authv1.RegisterParticipantRequest{AccessToken: "tk", Role: "ROLE_NOC"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp.UserId != "uid-1" || resp.WalletAddress != "0xwallet" {
		t.Errorf("unexpected resp %+v", resp)
	}

	// Token invalid.
	s1 := base()
	s1.keycloak = &fakeKeycloak{validateFn: func(ctx context.Context, tk string) (domain.TokenClaims, error) {
		return domain.TokenClaims{}, errors.New("bad")
	}}
	if _, err := s1.RegisterParticipant(context.Background(), &authv1.RegisterParticipantRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}

	// KMS failure.
	s2 := base()
	s2.kms = &fakeKMS{createFn: func(ctx context.Context, uid string) (kms.KeyInfo, error) {
		return kms.KeyInfo{}, errors.New("kms down")
	}}
	if _, err := s2.RegisterParticipant(context.Background(), &authv1.RegisterParticipantRequest{AccessToken: "tk"}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}

	// Upsert conflict → AlreadyExists.
	s3 := base()
	s3.compliance = &fakeCompliance{upsertFn: func(ctx context.Context, p complianceclient.Participant) error { return errors.New("duplicate key") }}
	if _, err := s3.RegisterParticipant(context.Background(), &authv1.RegisterParticipantRequest{AccessToken: "tk"}); status.Code(err) != codes.AlreadyExists {
		t.Errorf("expected AlreadyExists, got %v", err)
	}

	// Upsert other error → Internal.
	s4 := base()
	s4.compliance = &fakeCompliance{upsertFn: func(ctx context.Context, p complianceclient.Participant) error { return errors.New("db error") }}
	if _, err := s4.RegisterParticipant(context.Background(), &authv1.RegisterParticipantRequest{AccessToken: "tk"}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

// ── SignTransaction ───────────────────────────────────────────────────────────

func TestSignTransaction(t *testing.T) {
	t.Parallel()
	svc := &identityService{kms: &fakeKMS{signFn: func(ctx context.Context, uid, d string) (kms.SignResult, error) {
		return kms.SignResult{Signature: "0xsig", Address: "0xaddr"}, nil
	}}}
	resp, err := svc.SignTransaction(context.Background(), &authv1.SignTransactionRequest{UserId: "u", Digest: "0xdig"})
	if err != nil || resp.Signature != "0xsig" {
		t.Fatalf("sign: resp=%+v err=%v", resp, err)
	}

	// Key not found → NotFound.
	svc.kms = &fakeKMS{signFn: func(ctx context.Context, uid, d string) (kms.SignResult, error) {
		return kms.SignResult{}, kms.ErrKeyNotFound
	}}
	if _, err := svc.SignTransaction(context.Background(), &authv1.SignTransactionRequest{UserId: "u"}); status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}

	// Other error → Internal.
	svc.kms = &fakeKMS{signFn: func(ctx context.Context, uid, d string) (kms.SignResult, error) {
		return kms.SignResult{}, errors.New("hsm down")
	}}
	if _, err := svc.SignTransaction(context.Background(), &authv1.SignTransactionRequest{UserId: "u"}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

// ── GetKYCStatus / ProvisionParticipant ───────────────────────────────────────

func TestGetKYCStatus(t *testing.T) {
	t.Parallel()
	// Found, with status.
	svc := &identityService{compliance: &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{Status: "ACTIVE"}, true, nil
	}}}
	resp, _ := svc.GetKYCStatus(context.Background(), &authv1.GetKYCStatusRequest{Subject: "u"})
	if resp.Status != "ACTIVE" {
		t.Errorf("status = %q", resp.Status)
	}

	// Found, empty status → defaults PENDING.
	svc.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{}, true, nil
	}}
	resp, _ = svc.GetKYCStatus(context.Background(), &authv1.GetKYCStatusRequest{Subject: "u"})
	if resp.Status != string(domain.ParticipantStatusPending) {
		t.Errorf("expected PENDING default, got %q", resp.Status)
	}

	// Not found → PENDING.
	svc.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{}, false, nil
	}}
	resp, _ = svc.GetKYCStatus(context.Background(), &authv1.GetKYCStatusRequest{Subject: "u"})
	if resp.Status != string(domain.ParticipantStatusPending) {
		t.Errorf("expected PENDING for not found, got %q", resp.Status)
	}

	// Error → Internal.
	svc.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{}, false, errors.New("down")
	}}
	if _, err := svc.GetKYCStatus(context.Background(), &authv1.GetKYCStatusRequest{Subject: "u"}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

func TestProvisionParticipant(t *testing.T) {
	t.Parallel()
	svc := &identityService{compliance: &fakeCompliance{manageFn: func(ctx context.Context, s, st, r string) error { return nil }}}
	if _, err := svc.ProvisionParticipant(context.Background(), &authv1.ProvisionParticipantRequest{Subject: "u", Status: "ACTIVE"}); err != nil {
		t.Fatalf("provision: %v", err)
	}

	svc.compliance = &fakeCompliance{manageFn: func(ctx context.Context, s, st, r string) error { return errors.New("boom") }}
	if _, err := svc.ProvisionParticipant(context.Background(), &authv1.ProvisionParticipantRequest{Subject: "u"}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

// ── OnboardParticipant ────────────────────────────────────────────────────────

func newOnboardSvc() (*identityService, *fakeKeycloak, *fakeKMS, *fakeRegistry, *fakeCompliance) {
	kc := &fakeKeycloak{
		adminTokenFn: func(ctx context.Context) (string, error) { return "adm", nil },
		createUserFn: func(ctx context.Context, a string, r keycloak.CreateUserRequest) (string, error) {
			return "new-uid", nil
		},
		resetPwFn:     func(ctx context.Context, a, u, p string) error { return nil },
		getUsernameFn: func(ctx context.Context, a, u string) (string, error) { return "uname", nil },
	}
	km := &fakeKMS{createFn: func(ctx context.Context, uid string) (kms.KeyInfo, error) { return kms.KeyInfo{Address: "0xw"}, nil }}
	reg := &fakeRegistry{
		registerFn: func(ctx context.Context, a, i, r string, fp, institutionID [32]byte) (string, error) {
			return "0xtx", nil
		},
	}
	comp := &fakeCompliance{upsertFn: func(ctx context.Context, p complianceclient.Participant) error { return nil }}
	return &identityService{keycloak: kc, kms: km, blockchainClient: reg, compliance: comp}, kc, km, reg, comp
}

func TestOnboardParticipant_CommercialBank_FullFlow(t *testing.T) {
	t.Parallel()
	svc, _, _, reg, _ := newOnboardSvc()
	var registered bool
	reg.registerFn = func(ctx context.Context, a, i, r string, fp, institutionID [32]byte) (string, error) {
		registered = true
		return "0xtx", nil
	}

	resp, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{
		Username: "bank1", Email: "b@x.com", Role: domain.RoleCommercialBank, InstitutionName: "Bank One",
	})
	if err != nil {
		t.Fatalf("onboard: %v", err)
	}
	if resp.UserId != "new-uid" || resp.WalletAddress != "0xw" || resp.TxHash != "0xtx" || resp.ClientSecret == "" {
		t.Errorf("unexpected resp %+v", resp)
	}
	if !registered {
		t.Error("expected on-chain registration for commercial bank")
	}
}

// The institutionId written on-chain must come from the bank code, not the display name:
// it is what makes two wallets of one institution count as one in the AMM resume quorum,
// and a name-derived id would split them the moment the name is spelled differently.
func TestOnboardParticipant_DerivesInstitutionIDFromBankCode(t *testing.T) {
	t.Parallel()
	svc, _, _, reg, _ := newOnboardSvc()
	var got [32]byte
	reg.registerFn = func(ctx context.Context, a, i, r string, fp, institutionID [32]byte) (string, error) {
		got = institutionID
		return "0xtx", nil
	}

	if _, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{
		Username: "bank1", Email: "b@x.com", Role: domain.RoleCommercialBank,
		InstitutionName: "Bank One", BankCode: "bank-a",
	}); err != nil {
		t.Fatalf("onboard: %v", err)
	}

	if want := registry.InstitutionIDFromString("bank-a"); got != want {
		t.Fatalf("institutionId = %x, want keccak256(bank-a) = %x", got, want)
	}
	if got == registry.InstitutionIDFromString("Bank One") {
		t.Error("institutionId was derived from the display name, not the bank code")
	}
}

func TestOnboardParticipant_PasswordRole_NoKMSNoChain(t *testing.T) {
	t.Parallel()
	svc, _, km, reg, _ := newOnboardSvc()
	km.createFn = func(ctx context.Context, uid string) (kms.KeyInfo, error) {
		t.Fatal("KMS should not be called")
		return kms.KeyInfo{}, nil
	}
	reg.registerFn = func(ctx context.Context, a, i, r string, fp, institutionID [32]byte) (string, error) {
		t.Fatal("chain should not be called")
		return "", nil
	}

	resp, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{
		Username: "noc1", Email: "n@x.com", Role: domain.RoleNOC,
	})
	if err != nil {
		t.Fatalf("onboard: %v", err)
	}
	if resp.WalletAddress != "" || resp.TxHash != "" {
		t.Errorf("expected no wallet/tx for password role, got %+v", resp)
	}
}

func TestOnboardParticipant_Validation(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newOnboardSvc()

	// Missing fields.
	if _, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{Username: "x"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
	// Invalid role.
	if _, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{Username: "x", Email: "e", Role: "ROLE_BOGUS"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument for bad role, got %v", err)
	}
}

func TestOnboardParticipant_ErrorPaths(t *testing.T) {
	t.Parallel()

	// admin token error.
	svc, kc, _, _, _ := newOnboardSvc()
	kc.adminTokenFn = func(ctx context.Context) (string, error) { return "", errors.New("no token") }
	if _, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{Username: "x", Email: "e", Role: domain.RoleNOC}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}

	// user already exists.
	svc, kc, _, _, _ = newOnboardSvc()
	kc.createUserFn = func(ctx context.Context, a string, r keycloak.CreateUserRequest) (string, error) {
		return "", errors.New("user already exists")
	}
	if _, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{Username: "x", Email: "e", Role: domain.RoleNOC}); status.Code(err) != codes.AlreadyExists {
		t.Errorf("expected AlreadyExists, got %v", err)
	}

	// create user other error.
	svc, kc, _, _, _ = newOnboardSvc()
	kc.createUserFn = func(ctx context.Context, a string, r keycloak.CreateUserRequest) (string, error) {
		return "", errors.New("kc down")
	}
	if _, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{Username: "x", Email: "e", Role: domain.RoleNOC}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}

	// KMS error for commercial bank.
	svc, _, km, _, _ := newOnboardSvc()
	km.createFn = func(ctx context.Context, uid string) (kms.KeyInfo, error) {
		return kms.KeyInfo{}, errors.New("kms down")
	}
	if _, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{Username: "x", Email: "e", Role: domain.RoleCommercialBank}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal (kms), got %v", err)
	}

	// On-chain error for commercial bank.
	svc, _, _, reg, _ := newOnboardSvc()
	reg.registerFn = func(ctx context.Context, a, i, r string, fp, institutionID [32]byte) (string, error) {
		return "", errors.New("chain down")
	}
	if _, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{Username: "x", Email: "e", Role: domain.RoleCommercialBank}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal (chain), got %v", err)
	}

	// Upsert error.
	svc, _, _, _, comp := newOnboardSvc()
	comp.upsertFn = func(ctx context.Context, p complianceclient.Participant) error { return errors.New("db down") }
	if _, err := svc.OnboardParticipant(context.Background(), &authv1.OnboardParticipantRequest{Username: "x", Email: "e", Role: domain.RoleNOC}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal (upsert), got %v", err)
	}
}

// ── ListUsers / GetUser ───────────────────────────────────────────────────────

func TestListUsers(t *testing.T) {
	t.Parallel()
	svc := &identityService{compliance: &fakeCompliance{listFn: func(ctx context.Context, f complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
		return []complianceclient.Participant{{UserID: "a", Role: "ROLE_NOC"}, {UserID: "b"}}, nil
	}}}
	resp, err := svc.ListUsers(context.Background(), &authv1.ListUsersRequest{Role: "ROLE_NOC"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.Total != 2 || len(resp.Users) != 2 {
		t.Errorf("expected 2 users, got %+v", resp)
	}

	svc.compliance = &fakeCompliance{listFn: func(ctx context.Context, f complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
		return nil, errors.New("down")
	}}
	if _, err := svc.ListUsers(context.Background(), &authv1.ListUsersRequest{}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

func TestGetUser(t *testing.T) {
	t.Parallel()
	base := func() *identityService {
		return &identityService{
			compliance: &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
				return complianceclient.Participant{UserID: "u", InstitutionName: "Bank"}, true, nil
			}},
			keycloak: &fakeKeycloak{
				adminTokenFn:  func(ctx context.Context) (string, error) { return "adm", nil },
				getUsernameFn: func(ctx context.Context, a, u string) (string, error) { return "uname", nil },
				getEmailFn:    func(ctx context.Context, a, u string) (string, error) { return "e@x.com", nil },
			},
		}
	}
	resp, err := base().GetUser(context.Background(), &authv1.GetUserRequest{UserId: "u"})
	if err != nil {
		t.Fatalf("getuser: %v", err)
	}
	if resp.Username != "uname" || resp.Email != "e@x.com" || resp.InstitutionName != "Bank" {
		t.Errorf("unexpected resp %+v", resp)
	}

	// Missing id.
	if _, err := base().GetUser(context.Background(), &authv1.GetUserRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	// Not found.
	s1 := base()
	s1.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{}, false, nil
	}}
	if _, err := s1.GetUser(context.Background(), &authv1.GetUserRequest{UserId: "u"}); status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}

	// Compliance error.
	s2 := base()
	s2.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{}, false, errors.New("down")
	}}
	if _, err := s2.GetUser(context.Background(), &authv1.GetUserRequest{UserId: "u"}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}

	// Email lookup error is non-fatal.
	s3 := base()
	s3.keycloak = &fakeKeycloak{
		adminTokenFn:  func(ctx context.Context) (string, error) { return "adm", nil },
		getUsernameFn: func(ctx context.Context, a, u string) (string, error) { return "uname", nil },
		getEmailFn:    func(ctx context.Context, a, u string) (string, error) { return "", errors.New("no email") },
	}
	if resp, err := s3.GetUser(context.Background(), &authv1.GetUserRequest{UserId: "u"}); err != nil || resp.Email != "" {
		t.Errorf("email error should be non-fatal: resp=%+v err=%v", resp, err)
	}
}

// ── IssueLoginNonce ───────────────────────────────────────────────────────────

func newPKISvc() *identityService {
	return &identityService{
		compliance: &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
			return complianceclient.Participant{UserID: uid, Role: domain.RoleCommercialBank, WalletAddress: "0xw"}, true, nil
		}},
		keycloak: &fakeKeycloak{
			adminTokenFn:  func(ctx context.Context) (string, error) { return "adm", nil },
			getUsernameFn: func(ctx context.Context, a, u string) (string, error) { return "uname", nil },
			loginFn:       func(ctx context.Context, u, p string) (keycloak.TokenResponse, error) { return okToken(), nil },
		},
		blockchainClient: &fakeRegistry{canTransactFn: func(ctx context.Context, addr string) (bool, error) { return true, nil }},
		nonceStore:       newFakeNonce(),
	}
}

func TestIssueLoginNonce(t *testing.T) {
	t.Parallel()
	svc := newPKISvc()
	resp, err := svc.IssueLoginNonce(context.Background(), &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "sec"})
	if err != nil || resp.Nonce == "" {
		t.Fatalf("issue nonce: resp=%+v err=%v", resp, err)
	}

	// Validation.
	if _, err := svc.IssueLoginNonce(context.Background(), &authv1.IssueLoginNonceRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}
	if _, err := svc.IssueLoginNonce(context.Background(), &authv1.IssueLoginNonceRequest{UserId: "u"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument (secret), got %v", err)
	}

	// Not found.
	s1 := newPKISvc()
	s1.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{}, false, nil
	}}
	if _, err := s1.IssueLoginNonce(context.Background(), &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "s"}); status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", err)
	}

	// Non-PKI role.
	s2 := newPKISvc()
	s2.compliance = &fakeCompliance{getFn: func(ctx context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{Role: domain.RoleNOC}, true, nil
	}}
	if _, err := s2.IssueLoginNonce(context.Background(), &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "s"}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", err)
	}

	// Bad first-factor credentials.
	s3 := newPKISvc()
	s3.keycloak.(*fakeKeycloak).loginFn = func(ctx context.Context, u, p string) (keycloak.TokenResponse, error) {
		return keycloak.TokenResponse{}, errors.New("bad")
	}
	if _, err := s3.IssueLoginNonce(context.Background(), &authv1.IssueLoginNonceRequest{UserId: "u", ClientSecret: "s"}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}
}

// ── VerifyPKILogin (full crypto round-trip) ───────────────────────────────────

func TestVerifyPKILogin_Success(t *testing.T) {
	t.Parallel()
	caCert, caKey, err := pki.GenerateSelfSignedCA("CB CA", "CB", 5)
	if err != nil {
		t.Fatalf("ca: %v", err)
	}
	issued, err := pki.IssueCertificate(caCert, caKey, pki.CertRequest{Subject: "u", Org: "Bank", Role: domain.RoleCommercialBank, ValidYears: 1})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	svc := newPKISvc()
	svc.caCertPEM = caCert
	ns := svc.nonceStore.(*fakeNonce)

	// Simulate IssueLoginNonce having stored "nonce|secret".
	nonce := "deadbeef"
	_ = ns.Set(context.Background(), "u", nonce+"|sec", time.Minute)

	sig, err := pki.SignMessage(issued.PrivKeyPEM, nonce)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	resp, err := svc.VerifyPKILogin(context.Background(), &authv1.VerifyPKILoginRequest{
		UserId: "u", NonceSignatureHex: sig, CertPem: issued.CertPEM,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if resp.AccessToken != "at" || resp.TokenType != "Bearer" {
		t.Errorf("unexpected resp %+v", resp)
	}
}

func TestVerifyPKILogin_Failures(t *testing.T) {
	t.Parallel()

	// Validation.
	svc := newPKISvc()
	if _, err := svc.VerifyPKILogin(context.Background(), &authv1.VerifyPKILoginRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	// Nonce expired / not found.
	svc = newPKISvc()
	if _, err := svc.VerifyPKILogin(context.Background(), &authv1.VerifyPKILoginRequest{UserId: "u", NonceSignatureHex: "ab", CertPem: "x"}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated (no nonce), got %v", err)
	}

	// Bad cert chain (CA set, garbage cert).
	caCert, _, _ := pki.GenerateSelfSignedCA("CB CA", "CB", 5)
	svc = newPKISvc()
	svc.caCertPEM = caCert
	ns := svc.nonceStore.(*fakeNonce)
	_ = ns.Set(context.Background(), "u", "deadbeef|sec", time.Minute)
	if _, err := svc.VerifyPKILogin(context.Background(), &authv1.VerifyPKILoginRequest{UserId: "u", NonceSignatureHex: "ab", CertPem: "garbage-cert"}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated (bad chain), got %v", err)
	}

	// Valid chain, wrong signature.
	caCert2, caKey2, _ := pki.GenerateSelfSignedCA("CB CA", "CB", 5)
	issued, _ := pki.IssueCertificate(caCert2, caKey2, pki.CertRequest{Subject: "u", Org: "Bank", Role: domain.RoleCommercialBank, ValidYears: 1})
	svc = newPKISvc()
	svc.caCertPEM = caCert2
	ns = svc.nonceStore.(*fakeNonce)
	_ = ns.Set(context.Background(), "u", "deadbeef|sec", time.Minute)
	if _, err := svc.VerifyPKILogin(context.Background(), &authv1.VerifyPKILoginRequest{UserId: "u", NonceSignatureHex: "00ff", CertPem: issued.CertPEM}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated (sig mismatch), got %v", err)
	}

	// Valid chain + sig, but wallet not authorized on-chain → PermissionDenied.
	svc = newPKISvc()
	svc.caCertPEM = caCert2
	svc.blockchainClient = &fakeRegistry{canTransactFn: func(ctx context.Context, addr string) (bool, error) { return false, nil }}
	ns = svc.nonceStore.(*fakeNonce)
	nonce := "deadbeef"
	_ = ns.Set(context.Background(), "u", nonce+"|sec", time.Minute)
	sig, _ := pki.SignMessage(issued.PrivKeyPEM, nonce)
	if _, err := svc.VerifyPKILogin(context.Background(), &authv1.VerifyPKILoginRequest{UserId: "u", NonceSignatureHex: sig, CertPem: issued.CertPEM}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied (not authorized), got %v", err)
	}

	// H-5: cert CN must match the claimed UserId.
	caCert3, caKey3, _ := pki.GenerateSelfSignedCA("CB CA 3", "CB", 5)
	issued3, _ := pki.IssueCertificate(caCert3, caKey3, pki.CertRequest{Subject: "u", Org: "Bank", Role: domain.RoleCommercialBank, ValidYears: 1})
	svc3 := newPKISvc()
	svc3.caCertPEM = caCert3
	ns3 := svc3.nonceStore.(*fakeNonce)
	_ = ns3.Set(context.Background(), "other-user", "aabbccdd|sec", time.Minute)
	sig3, _ := pki.SignMessage(issued3.PrivKeyPEM, "aabbccdd")
	if _, err := svc3.VerifyPKILogin(context.Background(), &authv1.VerifyPKILoginRequest{
		UserId: "other-user", NonceSignatureHex: sig3, CertPem: issued3.CertPEM,
	}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated (CN mismatch), got %v", err)
	}

	// H-5: when the cert carries a wallet extension it must match the participant record.
	const certWalletAddr = "0x1111111111111111111111111111111111111111"
	const participantWalletAddr = "0x2222222222222222222222222222222222222222"
	caCert4, caKey4, _ := pki.GenerateSelfSignedCA("CB CA 4", "CB", 5)
	issued4, _ := pki.IssueCertificate(caCert4, caKey4, pki.CertRequest{
		Subject: "u", Org: "Bank", Role: domain.RoleCommercialBank, ValidYears: 1, WalletAddress: certWalletAddr,
	})
	svc4 := newPKISvc()
	svc4.caCertPEM = caCert4
	svc4.compliance = &fakeCompliance{getFn: func(_ context.Context, uid string) (complianceclient.Participant, bool, error) {
		return complianceclient.Participant{UserID: uid, Role: domain.RoleCommercialBank, WalletAddress: participantWalletAddr}, true, nil
	}}
	ns4 := svc4.nonceStore.(*fakeNonce)
	_ = ns4.Set(context.Background(), "u", "11223344|sec", time.Minute)
	sig4, _ := pki.SignMessage(issued4.PrivKeyPEM, "11223344")
	if _, err := svc4.VerifyPKILogin(context.Background(), &authv1.VerifyPKILoginRequest{
		UserId: "u", NonceSignatureHex: sig4, CertPem: issued4.CertPEM,
	}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated (wallet mismatch), got %v", err)
	}
}

// ── ChangeClientSecret ────────────────────────────────────────────────────────

func TestChangeClientSecret(t *testing.T) {
	t.Parallel()
	base := func() *identityService {
		return &identityService{keycloak: &fakeKeycloak{
			adminTokenFn:  func(ctx context.Context) (string, error) { return "adm", nil },
			getUsernameFn: func(ctx context.Context, a, u string) (string, error) { return "uname", nil },
			loginFn:       func(ctx context.Context, u, p string) (keycloak.TokenResponse, error) { return okToken(), nil },
			resetPwFn:     func(ctx context.Context, a, u, p string) error { return nil },
		}, compliance: &fakeCompliance{}}
	}
	if _, err := base().ChangeClientSecret(context.Background(), &authv1.ChangeClientSecretRequest{UserId: "u", CurrentSecret: "old", NewSecret: "new"}); err != nil {
		t.Fatalf("change: %v", err)
	}

	// Validation.
	if _, err := base().ChangeClientSecret(context.Background(), &authv1.ChangeClientSecretRequest{UserId: "u"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", err)
	}

	// Wrong current secret.
	s1 := base()
	s1.keycloak.(*fakeKeycloak).loginFn = func(ctx context.Context, u, p string) (keycloak.TokenResponse, error) {
		return keycloak.TokenResponse{}, errors.New("bad")
	}
	if _, err := s1.ChangeClientSecret(context.Background(), &authv1.ChangeClientSecretRequest{UserId: "u", CurrentSecret: "x", NewSecret: "y"}); status.Code(err) != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %v", err)
	}

	// Reset failure.
	s2 := base()
	s2.keycloak.(*fakeKeycloak).resetPwFn = func(ctx context.Context, a, u, p string) error { return errors.New("boom") }
	if _, err := s2.ChangeClientSecret(context.Background(), &authv1.ChangeClientSecretRequest{UserId: "u", CurrentSecret: "x", NewSecret: "y"}); status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %v", err)
	}
}

func TestIsConflict(t *testing.T) {
	t.Parallel()
	if isConflict(nil) {
		t.Error("nil should not be conflict")
	}
	for _, m := range []string{"duplicate key", "already exists", "UNIQUE violation", "conflict detected"} {
		if !isConflict(errors.New(m)) {
			t.Errorf("%q should be conflict", m)
		}
	}
	if isConflict(errors.New("some other error")) {
		t.Error("unrelated error should not be conflict")
	}
}
