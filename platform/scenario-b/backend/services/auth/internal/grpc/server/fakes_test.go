// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
)

// errUnset is returned by fake methods that a test did not wire up; surfacing it
// keeps unexpected call paths visible instead of silently returning zero values.
var errUnset = errors.New("fake: method not configured")

// ---------------------------------------------------------------------------
// fakeKeycloak — fully configurable keycloak.Client via func fields.
// ---------------------------------------------------------------------------

type fakeKeycloak struct {
	loginFn          func(ctx context.Context, username, password string) (keycloak.TokenResponse, error)
	refreshFn        func(ctx context.Context, refreshToken string) (keycloak.TokenResponse, error)
	logoutFn         func(ctx context.Context, refreshToken string) error
	validateTokenFn  func(ctx context.Context, accessToken string) (domain.TokenClaims, error)
	createUserFn     func(ctx context.Context, adminToken string, user keycloak.CreateUserRequest) (string, error)
	getAdminTokenFn  func(ctx context.Context) (string, error)
	updateUsernameFn func(ctx context.Context, adminToken, userID, newUsername string) error
	resetPasswordFn  func(ctx context.Context, adminToken, userID, password string) error
	getUsernameFn    func(ctx context.Context, adminToken, userID string) (string, error)
	getEmailFn       func(ctx context.Context, adminToken, userID string) (string, error)
	assignRoleFn     func(ctx context.Context, adminToken, userID, roleName string) error
}

func (f *fakeKeycloak) Login(ctx context.Context, u, p string) (keycloak.TokenResponse, error) {
	if f.loginFn != nil {
		return f.loginFn(ctx, u, p)
	}
	return keycloak.TokenResponse{}, errUnset
}

func (f *fakeKeycloak) Refresh(ctx context.Context, rt string) (keycloak.TokenResponse, error) {
	if f.refreshFn != nil {
		return f.refreshFn(ctx, rt)
	}
	return keycloak.TokenResponse{}, errUnset
}

func (f *fakeKeycloak) Logout(ctx context.Context, rt string) error {
	if f.logoutFn != nil {
		return f.logoutFn(ctx, rt)
	}
	return errUnset
}

func (f *fakeKeycloak) ValidateToken(ctx context.Context, at string) (domain.TokenClaims, error) {
	if f.validateTokenFn != nil {
		return f.validateTokenFn(ctx, at)
	}
	return domain.TokenClaims{}, errUnset
}

func (f *fakeKeycloak) CreateUser(ctx context.Context, adminToken string, user keycloak.CreateUserRequest) (string, error) {
	if f.createUserFn != nil {
		return f.createUserFn(ctx, adminToken, user)
	}
	return "", errUnset
}

func (f *fakeKeycloak) GetAdminToken(ctx context.Context) (string, error) {
	if f.getAdminTokenFn != nil {
		return f.getAdminTokenFn(ctx)
	}
	return "", errUnset
}

func (f *fakeKeycloak) UpdateUsername(ctx context.Context, adminToken, userID, newUsername string) error {
	if f.updateUsernameFn != nil {
		return f.updateUsernameFn(ctx, adminToken, userID, newUsername)
	}
	return errUnset
}

func (f *fakeKeycloak) ResetPassword(ctx context.Context, adminToken, userID, password string) error {
	if f.resetPasswordFn != nil {
		return f.resetPasswordFn(ctx, adminToken, userID, password)
	}
	return errUnset
}

func (f *fakeKeycloak) GetUserUsername(ctx context.Context, adminToken, userID string) (string, error) {
	if f.getUsernameFn != nil {
		return f.getUsernameFn(ctx, adminToken, userID)
	}
	return "", errUnset
}

func (f *fakeKeycloak) GetUserEmail(ctx context.Context, adminToken, userID string) (string, error) {
	if f.getEmailFn != nil {
		return f.getEmailFn(ctx, adminToken, userID)
	}
	return "", errUnset
}

func (f *fakeKeycloak) AssignRealmRole(ctx context.Context, adminToken, userID, roleName string) error {
	if f.assignRoleFn != nil {
		return f.assignRoleFn(ctx, adminToken, userID, roleName)
	}
	return nil
}

// ---------------------------------------------------------------------------
// fakeCompliance — fully configurable complianceclient.Client.
// ---------------------------------------------------------------------------

type fakeCompliance struct {
	upsertFn       func(ctx context.Context, p complianceclient.Participant) error
	getByUserFn    func(ctx context.Context, userID string) (complianceclient.Participant, bool, error)
	listFn         func(ctx context.Context, f complianceclient.ParticipantFilter) ([]complianceclient.Participant, error)
	auditFn        func(ctx context.Context, e complianceclient.AuditEntry) error
	issueCertFn    func(ctx context.Context, a, b, c, d string) (complianceclient.IssuedCertificate, error)
	signCSRFn      func(ctx context.Context, a, b, c, d, e string) (complianceclient.SignedCSR, error)
	manageStatusFn func(ctx context.Context, a, b, c string) error
}

func (f *fakeCompliance) UpsertParticipant(ctx context.Context, p complianceclient.Participant) error {
	if f.upsertFn != nil {
		return f.upsertFn(ctx, p)
	}
	return errUnset
}

func (f *fakeCompliance) GetParticipantByUser(ctx context.Context, userID string) (complianceclient.Participant, bool, error) {
	if f.getByUserFn != nil {
		return f.getByUserFn(ctx, userID)
	}
	return complianceclient.Participant{}, false, errUnset
}

func (f *fakeCompliance) ListParticipants(ctx context.Context, filter complianceclient.ParticipantFilter) ([]complianceclient.Participant, error) {
	if f.listFn != nil {
		return f.listFn(ctx, filter)
	}
	return nil, errUnset
}

func (f *fakeCompliance) CreateAuditLog(ctx context.Context, e complianceclient.AuditEntry) error {
	if f.auditFn != nil {
		return f.auditFn(ctx, e)
	}
	return nil
}

func (f *fakeCompliance) IssueParticipantCertificate(ctx context.Context, a, b, c, d string) (complianceclient.IssuedCertificate, error) {
	if f.issueCertFn != nil {
		return f.issueCertFn(ctx, a, b, c, d)
	}
	return complianceclient.IssuedCertificate{}, errUnset
}

func (f *fakeCompliance) SignParticipantCSR(ctx context.Context, a, b, c, d, e string) (complianceclient.SignedCSR, error) {
	if f.signCSRFn != nil {
		return f.signCSRFn(ctx, a, b, c, d, e)
	}
	return complianceclient.SignedCSR{}, errUnset
}

func (f *fakeCompliance) ManageParticipantStatus(ctx context.Context, a, b, c string) error {
	if f.manageStatusFn != nil {
		return f.manageStatusFn(ctx, a, b, c)
	}
	return errUnset
}

// ---------------------------------------------------------------------------
// fakeKMS — configurable kms.Provider.
// ---------------------------------------------------------------------------

type fakeKMS struct {
	createKeyFn  func(ctx context.Context, userID string) (kms.KeyInfo, error)
	signFn       func(ctx context.Context, userID, digestHex string) (kms.SignResult, error)
	getAddressFn func(ctx context.Context, userID string) (string, error)
}

func (f *fakeKMS) Name() string { return "fake" }

func (f *fakeKMS) CreateKey(ctx context.Context, userID string) (kms.KeyInfo, error) {
	if f.createKeyFn != nil {
		return f.createKeyFn(ctx, userID)
	}
	return kms.KeyInfo{}, errUnset
}

func (f *fakeKMS) Sign(ctx context.Context, userID, digestHex string) (kms.SignResult, error) {
	if f.signFn != nil {
		return f.signFn(ctx, userID, digestHex)
	}
	return kms.SignResult{}, errUnset
}

func (f *fakeKMS) GetAddress(ctx context.Context, userID string) (string, error) {
	if f.getAddressFn != nil {
		return f.getAddressFn(ctx, userID)
	}
	return "", errUnset
}

func (f *fakeKMS) DeleteKey(ctx context.Context, userID string) error { return nil }

// ---------------------------------------------------------------------------
// fakeNonce — configurable noncestore.NonceStore.
// ---------------------------------------------------------------------------

type fakeNonce struct {
	setFn          func(ctx context.Context, userID, nonce string, ttl time.Duration) error
	getAndDeleteFn func(ctx context.Context, userID string) (string, bool, error)
}

func (f *fakeNonce) Set(ctx context.Context, userID, nonce string, ttl time.Duration) error {
	if f.setFn != nil {
		return f.setFn(ctx, userID, nonce, ttl)
	}
	return nil
}

func (f *fakeNonce) GetAndDelete(ctx context.Context, userID string) (string, bool, error) {
	if f.getAndDeleteFn != nil {
		return f.getAndDeleteFn(ctx, userID)
	}
	return "", false, nil
}

// ---------------------------------------------------------------------------
// fakeRegistry — configurable blockchainRegistry.
// ---------------------------------------------------------------------------

type fakeRegistry struct {
	registerFn    func(ctx context.Context, wallet, name, role string, zk, institutionID [32]byte) (string, error)
	verifyFn      func(ctx context.Context, wallet string) (string, error)
	canTransactFn func(ctx context.Context, address string) (bool, error)
}

func (f *fakeRegistry) RegisterParticipant(ctx context.Context, wallet, name, role string, zk, institutionID [32]byte) (string, error) {
	if f.registerFn != nil {
		return f.registerFn(ctx, wallet, name, role, zk, institutionID)
	}
	return "", errUnset
}

func (f *fakeRegistry) GetInstitutionID(_ context.Context, _ string) ([32]byte, error) {
	return [32]byte{}, nil
}

func (f *fakeRegistry) VerifyParticipant(ctx context.Context, wallet string) (string, error) {
	if f.verifyFn != nil {
		return f.verifyFn(ctx, wallet)
	}
	return "", nil
}

func (f *fakeRegistry) UpdateStatus(_ context.Context, _ string, _ uint8) (string, error) {
	return "", nil
}

func (f *fakeRegistry) SetCertFingerprint(_ context.Context, _ string, _ [32]byte) (string, error) {
	return "", nil
}

func (f *fakeRegistry) CanTransact(ctx context.Context, address string) (bool, error) {
	if f.canTransactFn != nil {
		return f.canTransactFn(ctx, address)
	}
	return true, nil
}

func (f *fakeRegistry) IsWhitelisted(_ context.Context, _ string) (bool, error) { return true, nil }

func (f *fakeRegistry) GetParticipant(_ context.Context, _ string) (registry.OnChainParticipant, error) {
	return registry.OnChainParticipant{}, nil
}

func (f *fakeRegistry) GetCertFingerprint(_ context.Context, _ string) ([32]byte, error) {
	return [32]byte{}, nil
}
