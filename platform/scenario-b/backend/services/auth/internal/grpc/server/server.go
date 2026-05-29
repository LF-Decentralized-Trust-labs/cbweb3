package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/noncestore"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
	pki "github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
	authv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// blockchainRegistry combines read and write access to the IdentityRegistry contract.
// auth-service requires both: writes during onboarding, reads during VerifyPKILogin.
type blockchainRegistry interface {
	registry.RegistryWriter
	registry.RegistryReader
}

type identityService struct {
	authv1.UnimplementedAuthServiceServer
	keycloak         keycloak.Client
	kms              kms.Provider
	compliance       complianceclient.Client
	blockchainClient blockchainRegistry
	caCertPEM        string // Central Bank CA cert PEM for PKI verification
	nonceStore       noncestore.NonceStore
}

// New builds a configured gRPC server and registers all identity handlers.
// ns is the NonceStore used for PKI 2FA nonces; use noncestore.NewInMemoryStore()
// for dev or noncestore.NewRedisStore() for production.
func New(kc keycloak.Client, kmsProvider kms.Provider, compliance complianceclient.Client, bc blockchainRegistry, caCertPEM string, ns noncestore.NonceStore) *grpc.Server {
	svc := &identityService{
		keycloak:         kc,
		kms:              kmsProvider,
		compliance:       compliance,
		blockchainClient: bc,
		caCertPEM:        caCertPEM,
		nonceStore:       ns,
	}
	grpcServer := grpc.NewServer()
	authv1.RegisterAuthServiceServer(grpcServer, svc)
	return grpcServer
}

// --- Auth handlers ---

func (s *identityService) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	user := resolveLoginUsernameForKeycloak(ctx, s.keycloak, req.User)
	tr, err := s.keycloak.Login(ctx, user, req.Password)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	s.emitAudit(ctx, "LOGIN", "", "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &authv1.LoginResponse{
		AccessToken:      tr.AccessToken,
		RefreshToken:     tr.RefreshToken,
		TokenType:        tr.TokenType,
		ExpiresIn:        int32(tr.ExpiresIn),
		RefreshExpiresIn: int32(tr.RefreshExpiresIn),
	}, nil
}

func (s *identityService) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	tr, err := s.keycloak.Refresh(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return &authv1.RefreshTokenResponse{
		AccessToken:      tr.AccessToken,
		RefreshToken:     tr.RefreshToken,
		TokenType:        tr.TokenType,
		ExpiresIn:        int32(tr.ExpiresIn),
		RefreshExpiresIn: int32(tr.RefreshExpiresIn),
	}, nil
}

func (s *identityService) RevokeToken(ctx context.Context, req *authv1.RevokeTokenRequest) (*authv1.RevokeTokenResponse, error) {
	// Keycloak logout requires the refresh_token. Prefer the dedicated field;
	// fall back to access_token for backward compatibility with older callers.
	token := req.RefreshToken
	if token == "" {
		token = req.AccessToken
	}
	if err := s.keycloak.Logout(ctx, token); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.emitAudit(ctx, "REVOKE_TOKEN", "", "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &authv1.RevokeTokenResponse{}, nil
}

func (s *identityService) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	claims, err := s.keycloak.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// Best-effort enrichment: supplement JWT claims with compliance data when
	// wallet/country/bankId or roles are absent (e.g. users onboarded before
	// realm-role assignment was introduced, or Keycloak protocol mappers not
	// configured for custom attributes).
	if claims.Subject != "" {
		if p, found, lookupErr := s.compliance.GetParticipantByUser(ctx, claims.Subject); lookupErr == nil && found {
			// Append the participant's platform role if not already present in the JWT.
			// Keycloak default realm roles (e.g. default-roles-*, offline_access,
			// uma_authorization) are always present and must not prevent adding
			// the application role (ROLE_NOC, ROLE_COMMERCIAL_BANK, …).
			if p.Role != "" && !containsRoleSlice(claims.Roles, p.Role) {
				claims.Roles = append(claims.Roles, p.Role)
			}
			if claims.Wallet == "" && p.WalletAddress != "" {
				claims.Wallet = p.WalletAddress
			}
			if claims.Country == "" && p.CountryCode != "" {
				claims.Country = p.CountryCode
			}
			if claims.BankID == "" && p.BankCode != "" {
				claims.BankID = p.BankCode
			}
		}
	}

	return &authv1.ValidateTokenResponse{
		Subject:      claims.Subject,
		Issuer:       claims.Issuer,
		Roles:        claims.Roles,
		Wallet:       claims.Wallet,
		Country:      claims.Country,
		BankId:       claims.BankID,
		PrivacyGroup: claims.PrivacyGroup,
	}, nil
}

// --- Participant Registration ---

func (s *identityService) RegisterParticipant(ctx context.Context, req *authv1.RegisterParticipantRequest) (*authv1.RegisterParticipantResponse, error) {
	claims, err := s.keycloak.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	keyInfo, err := s.kms.CreateKey(ctx, claims.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	upsertErr := s.compliance.UpsertParticipant(ctx, complianceclient.Participant{
		UserID:          claims.Subject,
		WalletAddress:   keyInfo.Address,
		CountryCode:     req.Country,
		BankCode:        req.BankCode,
		Role:            req.Role,
		InstitutionName: req.InstitutionName,
		Status:          string(domain.ParticipantStatusPending),
	})
	if upsertErr != nil {
		if isConflict(upsertErr) {
			return nil, status.Error(codes.AlreadyExists, upsertErr.Error())
		}
		return nil, status.Error(codes.Internal, upsertErr.Error())
	}

	s.emitAudit(ctx, "REGISTER_PARTICIPANT", claims.Subject, keyInfo.Address, "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &authv1.RegisterParticipantResponse{
		UserId:        claims.Subject,
		WalletAddress: keyInfo.Address,
	}, nil
}

// --- Signing ---

func (s *identityService) SignTransaction(ctx context.Context, req *authv1.SignTransactionRequest) (*authv1.SignTransactionResponse, error) {
	result, err := s.kms.Sign(ctx, req.UserId, req.Digest)
	if err != nil {
		if errors.Is(err, kms.ErrKeyNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &authv1.SignTransactionResponse{
		Signature: result.Signature,
		Address:   result.Address,
	}, nil
}

// --- KYC Status ---

func (s *identityService) GetKYCStatus(ctx context.Context, req *authv1.GetKYCStatusRequest) (*authv1.GetKYCStatusResponse, error) {
	participant, found, err := s.compliance.GetParticipantByUser(ctx, req.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return &authv1.GetKYCStatusResponse{
			Subject: req.Subject,
			Status:  string(domain.ParticipantStatusPending),
		}, nil
	}
	st := participant.Status
	if st == "" {
		st = string(domain.ParticipantStatusPending)
	}
	return &authv1.GetKYCStatusResponse{
		Subject: req.Subject,
		Status:  st,
	}, nil
}

func (s *identityService) ProvisionParticipant(ctx context.Context, req *authv1.ProvisionParticipantRequest) (*authv1.ProvisionParticipantResponse, error) {
	if err := s.compliance.ManageParticipantStatus(ctx, req.Subject, req.Status, "provisioned via identity service"); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.emitAudit(ctx, "PROVISION_PARTICIPANT", "", "", req.Subject, correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &authv1.ProvisionParticipantResponse{}, nil
}

// --- Administrative Participant Onboarding ---

// OnboardParticipant is called by the Central Bank to register a new
// Commercial Bank or Treasury user. The flow is:
//
//  1. Obtain a Keycloak admin token via service-account client_credentials.
//  2. Create the user in Keycloak.
//  3. Conditionally generate a KMS key pair (roles requiring KMS).
//  4. Conditionally register the wallet address on-chain via ParticipantRegistry
//     (roles requiring on-chain registration, signed with CB_PRIVATE_KEY).
//  5. Persist the participant record via data-access service.
func (s *identityService) OnboardParticipant(ctx context.Context, req *authv1.OnboardParticipantRequest) (*authv1.OnboardParticipantResponse, error) {
	if req.Username == "" || req.Email == "" || req.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "username, email, and role are required")
	}
	if !domain.IsAdminRole(req.Role) {
		return nil, status.Errorf(codes.InvalidArgument, "role %q is not a valid admin-onboarded role", req.Role)
	}

	// 1. Obtain Keycloak admin token.
	adminToken, err := s.keycloak.GetAdminToken(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "onboard: obtaining admin token: %v", err)
	}

	// 2. Create user in Keycloak.
	// firstName/lastName are required by the realm user-profile policy.
	// EmailVerified is set to true because onboarding is administrative —
	// the Central Bank has already validated the institution's identity.
	displayName := req.InstitutionName
	if displayName == "" {
		displayName = req.Username
	}
	userID, err := s.keycloak.CreateUser(ctx, adminToken, keycloak.CreateUserRequest{
		Username:      req.Username,
		Email:         req.Email,
		FirstName:     displayName,
		LastName:      displayName,
		EmailVerified: true,
		Enabled:       true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil, status.Errorf(codes.AlreadyExists, "onboard: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "onboard: creating keycloak user: %v", err)
	}

	resp := &authv1.OnboardParticipantResponse{UserId: userID}

	// 3. Optionally generate KMS key.
	if domain.RequiresKMS(req.Role) {
		keyInfo, kmsErr := s.kms.CreateKey(ctx, userID)
		if kmsErr != nil {
			return nil, status.Errorf(codes.Internal, "onboard: creating kms key: %v", kmsErr)
		}
		resp.WalletAddress = keyInfo.Address
	}

	// 4. Optionally register on-chain.
	// Only the Central Bank calls OnboardParticipant — signing uses the
	// static CB_PRIVATE_KEY (StaticKeySigner). Commercial banks do NOT
	// perform on-chain registration directly; they proxy via CENTRAL_BANK_API_URL.
	if domain.RequiresOnChain(req.Role) && resp.WalletAddress != "" {
		txHash, chainErr := s.blockchainClient.RegisterParticipant(ctx, resp.WalletAddress, displayName, req.Role, [32]byte{})
		if chainErr != nil {
			return nil, status.Errorf(codes.Internal, "onboard: on-chain registration: %v", chainErr)
		}
		resp.TxHash = txHash
	}

	// 5. Persist participant record.
	// CertificateData is intentionally empty here — the certificate is issued
	// separately via POST /governance/registry/csr (Modalidade B, preferred)
	// or POST /governance/registry/credential (Modalidade A).
	if upsertErr := s.compliance.UpsertParticipant(ctx, complianceclient.Participant{
		UserID:          userID,
		WalletAddress:   resp.WalletAddress,
		CountryCode:     req.Country,
		BankCode:        req.BankCode,
		Role:            req.Role,
		InstitutionName: req.InstitutionName,
		Status:          string(domain.ParticipantStatusPending),
	}); upsertErr != nil {
		return nil, status.Errorf(codes.Internal, "onboard: persisting participant: %v", upsertErr)
	}

	// 6. Generate a cryptographically secure clientSecret (256 bits, base64-encoded).
	//    This is used as the first authentication factor in PKI login (IssueLoginNonce)
	//    and as the credential for password-based roles.
	//    The raw secret is returned ONCE in this response; Keycloak stores only the
	//    Argon2 hash. It is not recoverable after this point.
	rawBytes := make([]byte, 32)
	if _, rndErr := rand.Read(rawBytes); rndErr != nil {
		return nil, status.Errorf(codes.Internal, "onboard: generating client secret: %v", rndErr)
	}
	rawSecret := base64.StdEncoding.EncodeToString(rawBytes)

	if pErr := s.keycloak.ResetPassword(ctx, adminToken, userID, rawSecret); pErr != nil {
		log.Printf("WARN: onboard: setting password for %s: %v (non-fatal)", userID, pErr)
	}
	resp.ClientSecret = rawSecret

	// Assign the participant role in Keycloak so the JWT realm_access.roles
	// contains it and auth/me returns it correctly. Non-fatal.
	if roleErr := s.keycloak.AssignRealmRole(ctx, adminToken, userID, req.Role); roleErr != nil {
		log.Printf("WARN: onboard: assigning realm role %s to %s: %v (non-fatal)", req.Role, userID, roleErr)
	}

	s.emitAudit(ctx, "ONBOARD_PARTICIPANT", userID, resp.WalletAddress, "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return resp, nil
}

// --- User Management ---

// ListUsers returns a filtered list of registered participants sourced from the compliance service.
func (s *identityService) ListUsers(ctx context.Context, req *authv1.ListUsersRequest) (*authv1.ListUsersResponse, error) {
	participants, err := s.compliance.ListParticipants(ctx, complianceclient.ParticipantFilter{
		Role:   req.Role,
		Status: req.Status,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list users: %v", err)
	}
	users := make([]*authv1.UserSummary, 0, len(participants))
	for _, p := range participants {
		users = append(users, &authv1.UserSummary{
			UserId:          p.UserID,
			InstitutionName: p.InstitutionName,
			Role:            p.Role,
			Status:          p.Status,
			WalletAddress:   p.WalletAddress,
			Country:         p.CountryCode,
			BankCode:        p.BankCode,
		})
	}
	return &authv1.ListUsersResponse{Users: users, Total: int32(len(users))}, nil
}

// GetUser returns the full profile of a single participant, enriched with the Keycloak username.
func (s *identityService) GetUser(ctx context.Context, req *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	participant, found, err := s.compliance.GetParticipantByUser(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: compliance lookup: %v", err)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	adminToken, err := s.keycloak.GetAdminToken(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: admin token: %v", err)
	}
	username, err := s.keycloak.GetUserUsername(ctx, adminToken, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: keycloak lookup: %v", err)
	}
	email, err := s.keycloak.GetUserEmail(ctx, adminToken, req.UserId)
	if err != nil {
		log.Printf("WARN: GetUser: email lookup %s: %v (non-fatal)", req.UserId, err)
	}

	return &authv1.GetUserResponse{
		UserId:          participant.UserID,
		Username:        username,
		Email:           email,
		InstitutionName: participant.InstitutionName,
		Role:            participant.Role,
		Status:          participant.Status,
		WalletAddress:   participant.WalletAddress,
		Country:         participant.CountryCode,
		BankCode:        participant.BankCode,
	}, nil
}

// --- Audit helper ---

// IssueLoginNonce is PKI login step 1.
// It validates the clientSecret (first factor via Keycloak), checks that the
// user has a PKI role, then issues a short-lived nonce for the client to sign.
// The clientSecret is stored alongside the nonce so that VerifyPKILogin can
// authenticate with Keycloak in step 2 without asking for it again.
func (s *identityService) IssueLoginNonce(ctx context.Context, req *authv1.IssueLoginNonceRequest) (*authv1.IssueLoginNonceResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.ClientSecret == "" {
		return nil, status.Error(codes.InvalidArgument, "client_secret is required")
	}

	// 0. Verify this is a PKI-enabled participant.
	participant, found, lookupErr := s.compliance.GetParticipantByUser(ctx, req.UserId)
	if lookupErr != nil {
		return nil, status.Errorf(codes.Internal, "issue nonce: compliance lookup: %v", lookupErr)
	}
	if !found {
		return nil, status.Error(codes.NotFound, "participant not found")
	}
	if !domain.RequiresPKI(participant.Role) {
		return nil, status.Error(codes.PermissionDenied, "PKI_NOT_REQUIRED")
	}

	// 1. Validate clientSecret (first factor) via Keycloak.
	adminToken, adminErr := s.keycloak.GetAdminToken(ctx)
	if adminErr != nil {
		return nil, status.Errorf(codes.Internal, "issue nonce: admin token: %v", adminErr)
	}
	username, usernameErr := s.keycloak.GetUserUsername(ctx, adminToken, req.UserId)
	if usernameErr != nil {
		return nil, status.Errorf(codes.Internal, "issue nonce: get username: %v", usernameErr)
	}
	if _, loginErr := s.keycloak.Login(ctx, username, req.ClientSecret); loginErr != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	// 2. Generate nonce.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, status.Errorf(codes.Internal, "generate nonce: %v", err)
	}
	nonce := hex.EncodeToString(raw)

	// 3. Store "nonce|clientSecret" so VerifyPKILogin can retrieve the secret.
	//    The separator "|" is safe because base64 does not contain it.
	stored := nonce + "|" + req.ClientSecret
	if err := s.nonceStore.Set(ctx, req.UserId, stored, 30*time.Minute); err != nil {
		return nil, status.Errorf(codes.Internal, "nonce store: %v", err)
	}

	return &authv1.IssueLoginNonceResponse{Nonce: nonce}, nil
}

// VerifyPKILogin completes PKI login step 2:
//  1. Retrieves and validates the nonce from the store.
//  2. Verifies the X.509 cert chain against the Central Bank CA.
//  3. Validates the nonce signature against the cert's public key.
//  4. Checks on-chain authorization for the participant's wallet (best-effort).
//  5. Issues a Keycloak token on success.
func (s *identityService) VerifyPKILogin(ctx context.Context, req *authv1.VerifyPKILoginRequest) (*authv1.VerifyPKILoginResponse, error) {
	if req.UserId == "" || req.NonceSignatureHex == "" || req.CertPem == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, nonce_signature_hex, and cert_pem are required")
	}

	// 1. Retrieve and consume nonce (single-use, TTL enforced by store).
	//    Stored value format: "nonce_hex|clientSecret" (set by IssueLoginNonce).
	stored, found, err := s.nonceStore.GetAndDelete(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "nonce store: %v", err)
	}
	if !found {
		return nil, status.Error(codes.Unauthenticated, "nonce expired or not found — restart PKI login")
	}
	parts := strings.SplitN(stored, "|", 2)
	nonce := parts[0]
	clientSecret := ""
	if len(parts) == 2 {
		clientSecret = parts[1]
	}

	// 2. Verify certificate chain against Central Bank CA
	if s.caCertPEM != "" {
		if err := pki.VerifyChain(req.CertPem, s.caCertPEM); err != nil {
			return nil, status.Error(codes.Unauthenticated, "INVALID_CERTIFICATE_CHAIN")
		}
	}

	// 3. Validate nonce signature against cert's public key
	if err := pki.ValidateSignature(req.CertPem, nonce, req.NonceSignatureHex); err != nil {
		return nil, status.Error(codes.Unauthenticated, "NONCE_SIGNATURE_MISMATCH")
	}

	// 4. On-chain authorization check (best-effort: warn on errors, hard-fail only when explicitly unauthorized)
	if participant, found, lookupErr := s.compliance.GetParticipantByUser(ctx, req.UserId); lookupErr != nil {
		log.Printf("WARN: VerifyPKILogin: compliance lookup %s: %v (skipping on-chain check)", req.UserId, lookupErr)
	} else if found && participant.WalletAddress != "" {
		authorized, authErr := s.blockchainClient.CanTransact(ctx, participant.WalletAddress)
		if authErr != nil {
			log.Printf("WARN: VerifyPKILogin: on-chain check %s: %v (skipping)", req.UserId, authErr)
		} else if !authorized {
			return nil, status.Error(codes.PermissionDenied, "wallet not authorized on-chain")
		}
	}

	// 5. Issue Keycloak token using the clientSecret validated in step 1.
	//    The username may differ from the UUID (Keycloak read-only username policy),
	//    so we fetch it via the Admin API. PKI authentication was already proven by
	//    signature verification; the password here satisfies Keycloak's credential
	//    requirement and doubles as the first-factor secret.
	adminToken, err := s.keycloak.GetAdminToken(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "PKI: get admin token: %v", err)
	}
	username, err := s.keycloak.GetUserUsername(ctx, adminToken, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "PKI: get user username: %v", err)
	}
	tr, err := s.keycloak.Login(ctx, username, clientSecret)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "PKI: keycloak token: %v", err)
	}

	s.emitAudit(ctx, "PKI_LOGIN", req.UserId, "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")

	return &authv1.VerifyPKILoginResponse{
		AccessToken:      tr.AccessToken,
		RefreshToken:     tr.RefreshToken,
		TokenType:        "Bearer",
		ExpiresIn:        int32(tr.ExpiresIn),
		RefreshExpiresIn: int32(tr.RefreshExpiresIn),
	}, nil
}

// ChangeClientSecret allows an authenticated user to rotate their clientSecret.
// The current secret is validated via Keycloak before setting the new one.
func (s *identityService) ChangeClientSecret(ctx context.Context, req *authv1.ChangeClientSecretRequest) (*authv1.ChangeClientSecretResponse, error) {
	if req.UserId == "" || req.CurrentSecret == "" || req.NewSecret == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, current_secret, and new_secret are required")
	}

	adminToken, err := s.keycloak.GetAdminToken(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "change client secret: admin token: %v", err)
	}
	username, err := s.keycloak.GetUserUsername(ctx, adminToken, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "change client secret: get username: %v", err)
	}
	if _, loginErr := s.keycloak.Login(ctx, username, req.CurrentSecret); loginErr != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid current client secret")
	}
	if err := s.keycloak.ResetPassword(ctx, adminToken, req.UserId, req.NewSecret); err != nil {
		return nil, status.Errorf(codes.Internal, "change client secret: reset: %v", err)
	}

	s.emitAudit(ctx, "CHANGE_CLIENT_SECRET", req.UserId, "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &authv1.ChangeClientSecretResponse{}, nil
}

// emitAudit fires an audit log entry asynchronously (fire-and-forget).
// Failures in audit logging must NOT block the main business operation.
func (s *identityService) emitAudit(ctx context.Context, action, actorSubject, actorAddress, targetSubject, correlationID, ip, result string) {
	go func() {
		if err := s.compliance.CreateAuditLog(context.Background(), complianceclient.AuditEntry{
			ActionType:    action,
			ActorSubject:  actorSubject,
			ActorAddress:  actorAddress,
			TargetSubject: targetSubject,
			CorrelationID: correlationID,
			IPAddress:     ip,
			Result:        result,
			Category:      "SESSION",
			Severity:      "INFO",
		}); err != nil {
			log.Printf("WARN: audit emit failed: %v", err)
		}
	}()
}

// correlationIDFromCtx extracts X-Correlation-Id from gRPC incoming metadata.
func correlationIDFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-correlation-id"); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// ipAddressFromCtx extracts the client IP from gRPC incoming metadata.
func ipAddressFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-forwarded-for"); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// containsRoleSlice reports whether role is present in the slice.
func containsRoleSlice(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// isConflict returns true if the error message indicates a duplicate/conflict.
func isConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{"conflict", "duplicate", "already exists", "unique"} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
