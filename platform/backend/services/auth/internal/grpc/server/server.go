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

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/identity"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/complianceclient"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/grpc/contract"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/grpc/jsoncodec"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/kms"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/auth/internal/noncestore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// blockchainRegistry combines read and write access to the ParticipantRegistry contract.
// auth-service requires both: writes during RegisterParticipant / ProvisionParticipant,
// reads during VerifyPKILogin.
type blockchainRegistry interface {
	registry.RegistryWriter
	registry.RegistryReader
}

type identityService struct {
	keycloak         keycloak.Client
	kms              kms.Provider
	compliance       complianceclient.Client
	blockchainClient blockchainRegistry
	caCertPEM        string // Central Bank CA cert PEM for PKI verification
	nonceStore       noncestore.NonceStore
}

type identityServiceServer interface {
	Login(ctx context.Context, req *contract.LoginRequest) (*contract.LoginResponse, error)
	RefreshToken(ctx context.Context, req *contract.RefreshTokenRequest) (*contract.RefreshTokenResponse, error)
	RevokeToken(ctx context.Context, req *contract.RevokeTokenRequest) (*contract.RevokeTokenResponse, error)
	ValidateToken(ctx context.Context, req *contract.ValidateTokenRequest) (*contract.ValidateTokenResponse, error)
	RegisterParticipant(ctx context.Context, req *contract.RegisterParticipantRequest) (*contract.RegisterParticipantResponse, error)
	SignTransaction(ctx context.Context, req *contract.SignTransactionRequest) (*contract.SignTransactionResponse, error)
	GetKYCStatus(ctx context.Context, req *contract.GetKYCStatusRequest) (*contract.GetKYCStatusResponse, error)
	ProvisionParticipant(ctx context.Context, req *contract.ProvisionParticipantRequest) (*contract.ProvisionParticipantResponse, error)
	OnboardParticipant(ctx context.Context, req *contract.OnboardParticipantRequest) (*contract.OnboardParticipantResponse, error)
	ListUsers(ctx context.Context, req *contract.ListUsersRequest) (*contract.ListUsersResponse, error)
	GetUser(ctx context.Context, req *contract.GetUserRequest) (*contract.GetUserResponse, error)
	IssueLoginNonce(ctx context.Context, req *contract.IssueLoginNonceRequest) (*contract.IssueLoginNonceResponse, error)
	VerifyPKILogin(ctx context.Context, req *contract.VerifyPKILoginRequest) (*contract.VerifyPKILoginResponse, error)
	ChangeClientSecret(ctx context.Context, req *contract.ChangeClientSecretRequest) (*contract.ChangeClientSecretResponse, error)
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
	grpcServer := grpc.NewServer(grpc.ForceServerCodec(jsoncodec.Codec{}))
	grpcServer.RegisterService(&grpc.ServiceDesc{
		ServiceName: contract.ServiceName,
		HandlerType: (*identityServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Login", Handler: loginHandler},
			{MethodName: "RefreshToken", Handler: refreshTokenHandler},
			{MethodName: "RevokeToken", Handler: revokeTokenHandler},
			{MethodName: "ValidateToken", Handler: validateTokenHandler},
			{MethodName: "RegisterParticipant", Handler: registerParticipantHandler},
			{MethodName: "SignTransaction", Handler: signTransactionHandler},
			{MethodName: "GetKYCStatus", Handler: getKYCStatusHandler},
			{MethodName: "ProvisionParticipant", Handler: provisionParticipantHandler},
			{MethodName: "OnboardParticipant", Handler: onboardParticipantHandler},
			{MethodName: "ListUsers", Handler: listUsersHandler},
			{MethodName: "GetUser", Handler: getUserHandler},
			{MethodName: "IssueLoginNonce", Handler: issueLoginNonceHandler},
			{MethodName: "VerifyPKILogin", Handler: verifyPKILoginHandler},
			{MethodName: "ChangeClientSecret", Handler: changeClientSecretHandler},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "identity.v1",
	}, svc)
	return grpcServer
}

// --- Auth handlers ---

func (s *identityService) Login(ctx context.Context, req *contract.LoginRequest) (*contract.LoginResponse, error) {
	user := resolveLoginUsernameForKeycloak(ctx, s.keycloak, req.User)
	tr, err := s.keycloak.Login(ctx, user, req.Password)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	s.emitAudit(ctx, "LOGIN", "", "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &contract.LoginResponse{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		ExpiresIn:    int32(tr.ExpiresIn),
	}, nil
}

func (s *identityService) RefreshToken(ctx context.Context, req *contract.RefreshTokenRequest) (*contract.RefreshTokenResponse, error) {
	tr, err := s.keycloak.Refresh(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return &contract.RefreshTokenResponse{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		ExpiresIn:    int32(tr.ExpiresIn),
	}, nil
}

func (s *identityService) RevokeToken(ctx context.Context, req *contract.RevokeTokenRequest) (*contract.RevokeTokenResponse, error) {
	// NOTE: Keycloak's logout endpoint requires the refresh_token, but this
	// gRPC method only receives an access_token field. Callers should send
	// the refresh_token in the AccessToken field until this is reconciled.
	if err := s.keycloak.Logout(ctx, req.AccessToken); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.emitAudit(ctx, "REVOKE_TOKEN", "", "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &contract.RevokeTokenResponse{}, nil
}

func (s *identityService) ValidateToken(ctx context.Context, req *contract.ValidateTokenRequest) (*contract.ValidateTokenResponse, error) {
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

	return &contract.ValidateTokenResponse{
		Subject:      claims.Subject,
		Issuer:       claims.Issuer,
		Roles:        claims.Roles,
		Wallet:       claims.Wallet,
		Country:      claims.Country,
		BankID:       claims.BankID,
		PrivacyGroup: claims.PrivacyGroup,
	}, nil
}

// --- Participant Registration ---

func (s *identityService) RegisterParticipant(ctx context.Context, req *contract.RegisterParticipantRequest) (*contract.RegisterParticipantResponse, error) {
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
	return &contract.RegisterParticipantResponse{
		UserID:        claims.Subject,
		WalletAddress: keyInfo.Address,
	}, nil
}

// --- Signing ---

func (s *identityService) SignTransaction(ctx context.Context, req *contract.SignTransactionRequest) (*contract.SignTransactionResponse, error) {
	result, err := s.kms.Sign(ctx, req.UserID, req.Digest)
	if err != nil {
		if errors.Is(err, kms.ErrKeyNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &contract.SignTransactionResponse{
		Signature: result.Signature,
		Address:   result.Address,
	}, nil
}

// --- KYC Status ---

func (s *identityService) GetKYCStatus(ctx context.Context, req *contract.GetKYCStatusRequest) (*contract.GetKYCStatusResponse, error) {
	participant, found, err := s.compliance.GetParticipantByUser(ctx, req.Subject)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !found {
		return &contract.GetKYCStatusResponse{
			Subject: req.Subject,
			Status:  string(domain.ParticipantStatusPending),
		}, nil
	}
	st := participant.Status
	if st == "" {
		st = string(domain.ParticipantStatusPending)
	}
	return &contract.GetKYCStatusResponse{
		Subject: req.Subject,
		Status:  st,
	}, nil
}

func (s *identityService) ProvisionParticipant(ctx context.Context, req *contract.ProvisionParticipantRequest) (*contract.ProvisionParticipantResponse, error) {
	if err := s.compliance.ManageParticipantStatus(ctx, req.Subject, req.Status, "provisioned via identity service"); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.emitAudit(ctx, "PROVISION_PARTICIPANT", "", "", req.Subject, correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &contract.ProvisionParticipantResponse{}, nil
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
func (s *identityService) OnboardParticipant(ctx context.Context, req *contract.OnboardParticipantRequest) (*contract.OnboardParticipantResponse, error) {
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

	resp := &contract.OnboardParticipantResponse{UserID: userID}

	// 3. Optionally generate KMS key.
	if domain.RequiresKMS(req.Role) {
		keyInfo, kmsErr := s.kms.CreateKey(ctx, userID)
		if kmsErr != nil {
			return nil, status.Errorf(codes.Internal, "onboard: creating kms key: %v", kmsErr)
		}
		resp.WalletAddress = keyInfo.Address
	}

	// 4. Optionally register on-chain.
	if domain.RequiresOnChain(req.Role) && resp.WalletAddress != "" {
		txHash, chainErr := s.blockchainClient.SetParticipant(ctx, resp.WalletAddress, req.Role, true)
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
func (s *identityService) ListUsers(ctx context.Context, req *contract.ListUsersRequest) (*contract.ListUsersResponse, error) {
	participants, err := s.compliance.ListParticipants(ctx, complianceclient.ParticipantFilter{
		Role:   req.Role,
		Status: req.Status,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list users: %v", err)
	}
	users := make([]contract.UserSummary, 0, len(participants))
	for _, p := range participants {
		users = append(users, contract.UserSummary{
			UserID:          p.UserID,
			InstitutionName: p.InstitutionName,
			Role:            p.Role,
			Status:          p.Status,
			WalletAddress:   p.WalletAddress,
			Country:         p.CountryCode,
			BankCode:        p.BankCode,
		})
	}
	return &contract.ListUsersResponse{Users: users, Total: len(users)}, nil
}

// GetUser returns the full profile of a single participant, enriched with the Keycloak username.
func (s *identityService) GetUser(ctx context.Context, req *contract.GetUserRequest) (*contract.GetUserResponse, error) {
	if req.UserID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	participant, found, err := s.compliance.GetParticipantByUser(ctx, req.UserID)
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
	username, err := s.keycloak.GetUserUsername(ctx, adminToken, req.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: keycloak lookup: %v", err)
	}
	email, err := s.keycloak.GetUserEmail(ctx, adminToken, req.UserID)
	if err != nil {
		log.Printf("WARN: GetUser: email lookup %s: %v (non-fatal)", req.UserID, err)
	}

	return &contract.GetUserResponse{
		UserID:          participant.UserID,
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
func (s *identityService) IssueLoginNonce(ctx context.Context, req *contract.IssueLoginNonceRequest) (*contract.IssueLoginNonceResponse, error) {
	if req.UserID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.ClientSecret == "" {
		return nil, status.Error(codes.InvalidArgument, "client_secret is required")
	}

	// 0. Verify this is a PKI-enabled participant.
	participant, found, lookupErr := s.compliance.GetParticipantByUser(ctx, req.UserID)
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
	username, usernameErr := s.keycloak.GetUserUsername(ctx, adminToken, req.UserID)
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
	if err := s.nonceStore.Set(ctx, req.UserID, stored, 5*time.Minute); err != nil {
		return nil, status.Errorf(codes.Internal, "nonce store: %v", err)
	}

	return &contract.IssueLoginNonceResponse{Nonce: nonce}, nil
}

// VerifyPKILogin completes PKI login step 2:
//  1. Retrieves and validates the nonce from the store.
//  2. Verifies the X.509 cert chain against the Central Bank CA.
//  3. Validates the nonce signature against the cert's public key.
//  4. Checks on-chain authorization for the participant's wallet (best-effort).
//  5. Issues a Keycloak token on success.
func (s *identityService) VerifyPKILogin(ctx context.Context, req *contract.VerifyPKILoginRequest) (*contract.VerifyPKILoginResponse, error) {
	if req.UserID == "" || req.NonceSignatureHex == "" || req.CertPEM == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, nonce_signature_hex, and cert_pem are required")
	}

	// 1. Retrieve and consume nonce (single-use, TTL enforced by store).
	//    Stored value format: "nonce_hex|clientSecret" (set by IssueLoginNonce).
	stored, found, err := s.nonceStore.GetAndDelete(ctx, req.UserID)
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
		if err := pki.VerifyChain(req.CertPEM, s.caCertPEM); err != nil {
			return nil, status.Error(codes.Unauthenticated, "INVALID_CERTIFICATE_CHAIN")
		}
	}

	// 3. Validate nonce signature against cert's public key
	if err := pki.ValidateSignature(req.CertPEM, nonce, req.NonceSignatureHex); err != nil {
		return nil, status.Error(codes.Unauthenticated, "NONCE_SIGNATURE_MISMATCH")
	}

	// 4. On-chain authorization check (best-effort: warn on errors, hard-fail only when explicitly unauthorized)
	if participant, found, lookupErr := s.compliance.GetParticipantByUser(ctx, req.UserID); lookupErr != nil {
		log.Printf("WARN: VerifyPKILogin: compliance lookup %s: %v (skipping on-chain check)", req.UserID, lookupErr)
	} else if found && participant.WalletAddress != "" {
		authorized, authErr := s.blockchainClient.IsMemberAuthorized(ctx, participant.WalletAddress)
		if authErr != nil {
			log.Printf("WARN: VerifyPKILogin: on-chain check %s: %v (skipping)", req.UserID, authErr)
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
	username, err := s.keycloak.GetUserUsername(ctx, adminToken, req.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "PKI: get user username: %v", err)
	}
	tr, err := s.keycloak.Login(ctx, username, clientSecret)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "PKI: keycloak token: %v", err)
	}

	s.emitAudit(ctx, "PKI_LOGIN", req.UserID, "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")

	return &contract.VerifyPKILoginResponse{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    "Bearer",
	}, nil
}

// ChangeClientSecret allows an authenticated user to rotate their clientSecret.
// The current secret is validated via Keycloak before setting the new one.
func (s *identityService) ChangeClientSecret(ctx context.Context, req *contract.ChangeClientSecretRequest) (*contract.ChangeClientSecretResponse, error) {
	if req.UserID == "" || req.CurrentClientSecret == "" || req.NewClientSecret == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, current_client_secret, and new_client_secret are required")
	}

	adminToken, err := s.keycloak.GetAdminToken(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "change client secret: admin token: %v", err)
	}
	username, err := s.keycloak.GetUserUsername(ctx, adminToken, req.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "change client secret: get username: %v", err)
	}
	if _, loginErr := s.keycloak.Login(ctx, username, req.CurrentClientSecret); loginErr != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid current client secret")
	}
	if err := s.keycloak.ResetPassword(ctx, adminToken, req.UserID, req.NewClientSecret); err != nil {
		return nil, status.Errorf(codes.Internal, "change client secret: reset: %v", err)
	}

	s.emitAudit(ctx, "CHANGE_CLIENT_SECRET", req.UserID, "", "", correlationIDFromCtx(ctx), ipAddressFromCtx(ctx), "SUCCESS")
	return &contract.ChangeClientSecretResponse{}, nil
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

// --- gRPC handler adapters (boilerplate) ---

func loginHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.LoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.LoginMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).Login(ctx, req.(*contract.LoginRequest))
	})
}

func refreshTokenHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.RefreshTokenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).RefreshToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.RefreshTokenMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).RefreshToken(ctx, req.(*contract.RefreshTokenRequest))
	})
}

func revokeTokenHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.RevokeTokenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).RevokeToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.RevokeTokenMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).RevokeToken(ctx, req.(*contract.RevokeTokenRequest))
	})
}

func validateTokenHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ValidateTokenRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).ValidateToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ValidateTokenMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).ValidateToken(ctx, req.(*contract.ValidateTokenRequest))
	})
}

func registerParticipantHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.RegisterParticipantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).RegisterParticipant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.RegisterParticipantMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).RegisterParticipant(ctx, req.(*contract.RegisterParticipantRequest))
	})
}

func signTransactionHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.SignTransactionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).SignTransaction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.SignTransactionMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).SignTransaction(ctx, req.(*contract.SignTransactionRequest))
	})
}

func getKYCStatusHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.GetKYCStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).GetKYCStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetKYCStatusMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).GetKYCStatus(ctx, req.(*contract.GetKYCStatusRequest))
	})
}

func provisionParticipantHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ProvisionParticipantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).ProvisionParticipant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ProvisionParticipantMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).ProvisionParticipant(ctx, req.(*contract.ProvisionParticipantRequest))
	})
}

func onboardParticipantHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.OnboardParticipantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).OnboardParticipant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.OnboardParticipantMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).OnboardParticipant(ctx, req.(*contract.OnboardParticipantRequest))
	})
}

func issueLoginNonceHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.IssueLoginNonceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).IssueLoginNonce(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.IssueLoginNonceMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).IssueLoginNonce(ctx, req.(*contract.IssueLoginNonceRequest))
	})
}

func verifyPKILoginHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.VerifyPKILoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).VerifyPKILogin(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.VerifyPKILoginMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).VerifyPKILogin(ctx, req.(*contract.VerifyPKILoginRequest))
	})
}

func changeClientSecretHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ChangeClientSecretRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).ChangeClientSecret(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ChangeClientSecretMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).ChangeClientSecret(ctx, req.(*contract.ChangeClientSecretRequest))
	})
}

func listUsersHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.ListUsersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).ListUsers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.ListUsersMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).ListUsers(ctx, req.(*contract.ListUsersRequest))
	})
}

func getUserHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(contract.GetUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*identityService).GetUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: contract.GetUserMethod}
	return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
		return srv.(*identityService).GetUser(ctx, req.(*contract.GetUserRequest))
	})
}
