package providers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/identity/internal/identityprovider"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v5"
)

// localUser holds a pre-seeded participant for local development.
type localUser struct {
	Username  string
	Password  string
	Role      string
	KYCStatus identityprovider.KYCStatus
}

// localKey holds a generated secp256k1 key pair for a user.
type localKey struct {
	privKeyHex string // hex-encoded 32-byte private key
	address    string // EVM address (0x...)
}

// localCredential stores an issued KYC Verifiable Credential.
type localCredential struct {
	VCJWT      string
	ZKPPointer string
	IssuedAt   time.Time
	Revoked    bool
}

// LocalConfig contains in-memory provider options for local development.
type LocalConfig struct {
	JWTSecret     string
	AccessTokenTT time.Duration
}

// LocalProvider is a full simulation of the LNET D-Wallet + SSI-VC-API stack
// for local development and testing. No external dependencies are required.
//
// It satisfies the identityprovider.Provider interface completely, including
// SignTransaction (secp256k1), IssueKYCCredential (SHA-256 ZKP pointer),
// and ProvisionParticipant (KYC lifecycle management).
type LocalProvider struct {
	cfg              LocalConfig
	mu               sync.RWMutex
	users            map[string]localUser
	walletByUser     map[string]identityprovider.WalletResponse
	userByWallet     map[string]string
	keyByUser        map[string]localKey           // secp256k1 key per user
	kycStatus        map[string]identityprovider.KYCStatus // subject → KYC status
	credentials      map[string]localCredential    // zkpPointer → credential
	credBySubject    map[string]string             // subject → zkpPointer
	refreshTokens    map[string]string             // refreshToken → userID
	revokedTokens    map[string]struct{}           // revoked access tokens (blocklist)
}

// NewLocalProvider creates a local provider that simulates the LNET stack.
func NewLocalProvider(cfg LocalConfig) *LocalProvider {
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		cfg.JWTSecret = "local-identity-secret"
	}
	if cfg.AccessTokenTT <= 0 {
		cfg.AccessTokenTT = time.Hour
	}
	p := &LocalProvider{
		cfg: cfg,
		users: map[string]localUser{
			// Central Banks — auto-approved, bypass KYC gate, can issue credentials
			"central-bank-a": {Username: "central-bank-a", Password: "secret-bca", Role: identityprovider.RoleCentralBank, KYCStatus: identityprovider.KYCStatusApproved},
			"central-bank-b": {Username: "central-bank-b", Password: "secret-bcb", Role: identityprovider.RoleCentralBank, KYCStatus: identityprovider.KYCStatusApproved},
			// Commercial Banks — pre-approved for dev convenience
			"bank-a": {Username: "bank-a", Password: "secret-a", Role: identityprovider.RoleCommercialBank, KYCStatus: identityprovider.KYCStatusApproved},
			"bank-b": {Username: "bank-b", Password: "secret-b", Role: identityprovider.RoleCommercialBank, KYCStatus: identityprovider.KYCStatusApproved},
			// Sanctioned bank — for testing AML/CFT block scenarios (E2E-01-FL03)
			"bank-z": {Username: "bank-z", Password: "secret-z", Role: identityprovider.RoleCommercialBank, KYCStatus: identityprovider.KYCStatusRevoked},
			// Legacy users kept for backward compatibility
			"issueruser":   {Username: "issueruser", Password: "1234", Role: identityprovider.RoleCentralBank, KYCStatus: identityprovider.KYCStatusApproved},
			"holderuser":   {Username: "holderuser", Password: "1234", Role: identityprovider.RoleCommercialBank, KYCStatus: identityprovider.KYCStatusApproved},
			"verifieruser": {Username: "verifieruser", Password: "1234", Role: identityprovider.RoleCommercialBank, KYCStatus: identityprovider.KYCStatusApproved},
		},
		walletByUser:  map[string]identityprovider.WalletResponse{},
		userByWallet:  map[string]string{},
		keyByUser:     map[string]localKey{},
		kycStatus:     map[string]identityprovider.KYCStatus{},
		credentials:   map[string]localCredential{},
		credBySubject: map[string]string{},
		refreshTokens: map[string]string{},
		revokedTokens: map[string]struct{}{},
	}
	// Seed kycStatus from users so ProvisionParticipant overrides work cleanly
	for username, u := range p.users {
		p.kycStatus[username] = u.KYCStatus
	}
	return p
}

func (p *LocalProvider) Name() string { return ProviderLocal }

// Login validates credentials and returns JWT access + refresh tokens.
// The JWT simulates the LNET-issued token with enriched claims (D7 §7.4).
func (p *LocalProvider) Login(_ context.Context, req identityprovider.LoginRequest) (identityprovider.TokenResponse, error) {
	p.mu.RLock()
	u, ok := p.users[req.User]
	p.mu.RUnlock()
	if !ok || u.Password != req.Password {
		return identityprovider.TokenResponse{}, errors.New("invalid credentials")
	}

	// Retrieve wallet info if already provisioned (for enriched claims)
	p.mu.RLock()
	wallet := p.walletByUser[u.Username]
	p.mu.RUnlock()

	accessToken, err := p.issueAccessToken(u, wallet)
	if err != nil {
		return identityprovider.TokenResponse{}, err
	}
	refreshToken, err := randomHex(32)
	if err != nil {
		return identityprovider.TokenResponse{}, err
	}

	p.mu.Lock()
	p.refreshTokens[refreshToken] = u.Username
	p.mu.Unlock()

	return identityprovider.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(p.cfg.AccessTokenTT.Seconds()),
	}, nil
}

// RefreshToken issues a new access token from a valid refresh token.
func (p *LocalProvider) RefreshToken(_ context.Context, refreshToken string) (identityprovider.TokenResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	userID, ok := p.refreshTokens[refreshToken]
	if !ok {
		return identityprovider.TokenResponse{}, errors.New("invalid or expired refresh token")
	}
	u, ok := p.users[userID]
	if !ok {
		return identityprovider.TokenResponse{}, errors.New("user not found")
	}

	wallet := p.walletByUser[userID]
	accessToken, err := p.issueAccessToken(u, wallet)
	if err != nil {
		return identityprovider.TokenResponse{}, err
	}

	newRefresh, err := randomHex(32)
	if err != nil {
		return identityprovider.TokenResponse{}, err
	}
	delete(p.refreshTokens, refreshToken)
	p.refreshTokens[newRefresh] = userID

	return identityprovider.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(p.cfg.AccessTokenTT.Seconds()),
	}, nil
}

// RevokeToken adds the access token to the in-memory blocklist (logout).
func (p *LocalProvider) RevokeToken(_ context.Context, accessToken string) error {
	p.mu.Lock()
	p.revokedTokens[accessToken] = struct{}{}
	p.mu.Unlock()
	return nil
}

// ValidateToken validates local HS256 tokens and extracts enriched claims.
func (p *LocalProvider) ValidateToken(_ context.Context, accessToken string) (identityprovider.TokenClaims, error) {
	p.mu.RLock()
	_, revoked := p.revokedTokens[accessToken]
	p.mu.RUnlock()
	if revoked {
		return identityprovider.TokenClaims{}, errors.New("token has been revoked")
	}

	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(p.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return identityprovider.TokenClaims{}, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return identityprovider.TokenClaims{}, errors.New("invalid token claims")
	}
	subject, _ := claims["sub"].(string)
	if strings.TrimSpace(subject) == "" {
		subject, _ = claims["preferred_username"].(string)
	}
	if strings.TrimSpace(subject) == "" {
		return identityprovider.TokenClaims{}, errors.New("missing subject claim")
	}

	issuer, _ := claims["iss"].(string)
	did, _ := claims["did"].(string)
	wallet, _ := claims["wallet"].(string)
	country, _ := claims["country"].(string)
	bankID, _ := claims["bank_id"].(string)
	privacyGroup, _ := claims["privacy_group"].(string)
	roles := extractRoles(claims["roles"])

	return identityprovider.TokenClaims{
		Subject:      subject,
		Issuer:       issuer,
		Roles:        roles,
		DID:          did,
		Wallet:       wallet,
		Country:      country,
		BankID:       bankID,
		PrivacyGroup: privacyGroup,
	}, nil
}

// CreateWallet generates a secp256k1 key pair and derives a DID + EVM address.
// This simulates the D-Wallet API creating a wallet and registering the DID
// on the DID Registry smart contract.
func (p *LocalProvider) CreateWallet(ctx context.Context, accessToken string) (identityprovider.WalletResponse, error) {
	claims, err := p.ValidateToken(ctx, accessToken)
	if err != nil {
		return identityprovider.WalletResponse{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if existing, ok := p.walletByUser[claims.Subject]; ok {
		return existing, nil
	}

	key, err := gethcrypto.GenerateKey()
	if err != nil {
		return identityprovider.WalletResponse{}, fmt.Errorf("generating key: %w", err)
	}
	privHex := hex.EncodeToString(gethcrypto.FromECDSA(key))
	address := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	did := "did:lac:openprotest:" + strings.ToLower(address)

	p.keyByUser[claims.Subject] = localKey{privKeyHex: privHex, address: address}

	w := identityprovider.WalletResponse{
		DID:       did,
		Address:   address,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	p.walletByUser[claims.Subject] = w
	p.userByWallet[strings.ToLower(address)] = claims.Subject
	return w, nil
}

// BindWallet associates a user identity with a wallet address, enforcing uniqueness.
func (p *LocalProvider) BindWallet(_ context.Context, userID, walletAddress string) (identityprovider.WalletBinding, error) {
	userID = strings.TrimSpace(userID)
	walletAddress = strings.ToLower(strings.TrimSpace(walletAddress))
	if userID == "" || walletAddress == "" {
		return identityprovider.WalletBinding{}, errors.New("userID and walletAddress are required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if existingUser, ok := p.userByWallet[walletAddress]; ok && existingUser != userID {
		return identityprovider.WalletBinding{}, errors.New("wallet already bound to another user")
	}
	if existingWallet, ok := p.walletByUser[userID]; ok && !strings.EqualFold(existingWallet.Address, walletAddress) {
		return identityprovider.WalletBinding{}, errors.New("user already bound to another wallet")
	}

	wallet := identityprovider.WalletResponse{
		DID:       "did:lac:openprotest:" + walletAddress,
		Address:   walletAddress,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	p.walletByUser[userID] = wallet
	p.userByWallet[walletAddress] = userID
	return identityprovider.WalletBinding{
		UserID:        userID,
		WalletAddress: walletAddress,
	}, nil
}

// GetByUser retrieves an existing wallet binding for a user.
func (p *LocalProvider) GetByUser(_ context.Context, userID string) (identityprovider.WalletBinding, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return identityprovider.WalletBinding{}, false, nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	wallet, ok := p.walletByUser[userID]
	if !ok {
		return identityprovider.WalletBinding{}, false, nil
	}
	return identityprovider.WalletBinding{
		UserID:        userID,
		WalletAddress: wallet.Address,
	}, true, nil
}

// SignTransaction signs a 32-byte digest with the user's secp256k1 private key.
// This simulates the D-Wallet API signing endpoint — in production the key
// never leaves the LNET custody infrastructure.
func (p *LocalProvider) SignTransaction(_ context.Context, req identityprovider.SignRequest) (identityprovider.SignResponse, error) {
	if req.UserID == "" || req.DigestHex == "" {
		return identityprovider.SignResponse{}, errors.New("user_id and digest_hex are required")
	}

	p.mu.RLock()
	k, ok := p.keyByUser[req.UserID]
	p.mu.RUnlock()
	if !ok {
		return identityprovider.SignResponse{}, fmt.Errorf("no key found for user %q — call CreateWallet first", req.UserID)
	}

	digestBytes, err := hex.DecodeString(strings.TrimPrefix(req.DigestHex, "0x"))
	if err != nil {
		return identityprovider.SignResponse{}, fmt.Errorf("invalid digest hex: %w", err)
	}
	if len(digestBytes) != 32 {
		return identityprovider.SignResponse{}, errors.New("digest must be exactly 32 bytes")
	}

	privKeyBytes, err := hex.DecodeString(k.privKeyHex)
	if err != nil {
		return identityprovider.SignResponse{}, fmt.Errorf("corrupt key: %w", err)
	}
	privKey, err := gethcrypto.ToECDSA(privKeyBytes)
	if err != nil {
		return identityprovider.SignResponse{}, fmt.Errorf("parsing key: %w", err)
	}

	sig, err := gethcrypto.Sign(digestBytes, privKey)
	if err != nil {
		return identityprovider.SignResponse{}, fmt.Errorf("signing: %w", err)
	}

	return identityprovider.SignResponse{
		Signature:      "0x" + hex.EncodeToString(sig),
		Address:        k.address,
		SignerProvider: ProviderLocal,
	}, nil
}

// IssueKYCCredential creates a Verifiable Credential for a participant.
// This simulates the SSI-VC-API issuing a W3C VC and registering it on-chain.
// The ZKP pointer is a SHA-256 hash of the VC payload — usable as an on-chain
// pointer without exposing PII (NFR-SEC-003, REQ-COM-002).
func (p *LocalProvider) IssueKYCCredential(_ context.Context, req identityprovider.KYCCredentialRequest) (identityprovider.KYCCredentialResponse, error) {
	if req.Subject == "" || req.IssuerSubject == "" {
		return identityprovider.KYCCredentialResponse{}, errors.New("subject and issuer_subject are required")
	}

	issuedAt := time.Now().UTC()

	vcPayload := map[string]any{
		"@context":          []string{"https://www.w3.org/2018/credentials/v1"},
		"type":              []string{"VerifiableCredential", "KYCCredential"},
		"issuer":            req.IssuerSubject,
		"issuanceDate":      issuedAt.Format(time.RFC3339),
		"credentialSubject": map[string]any{
			"id":              req.Subject,
			"institutionName": req.InstitutionName,
			"countryCode":     req.CountryCode,
			"bankCode":        req.BankCode,
			"kycStatus":       string(identityprovider.KYCStatusApproved),
		},
	}
	payloadBytes, err := json.Marshal(vcPayload)
	if err != nil {
		return identityprovider.KYCCredentialResponse{}, fmt.Errorf("marshalling vc payload: %w", err)
	}

	hash := sha256.Sum256(payloadBytes)
	zkpPointer := hex.EncodeToString(hash[:])

	// Sign the pointer with issuer's key if available (simulation of VC JWT)
	vcJWT := "local.vc." + zkpPointer

	cred := localCredential{
		VCJWT:      vcJWT,
		ZKPPointer: zkpPointer,
		IssuedAt:   issuedAt,
		Revoked:    false,
	}

	p.mu.Lock()
	p.credentials[zkpPointer] = cred
	p.credBySubject[req.Subject] = zkpPointer
	p.kycStatus[req.Subject] = identityprovider.KYCStatusApproved
	p.mu.Unlock()

	return identityprovider.KYCCredentialResponse{
		VCJWT:      vcJWT,
		ZKPPointer: zkpPointer,
		IssuedAt:   issuedAt.Format(time.RFC3339),
	}, nil
}

// VerifyKYCProof checks that a ZKP pointer corresponds to a valid, non-revoked credential.
// This simulates the Claims Verifier smart contract on Besu (REQ-COM-002).
func (p *LocalProvider) VerifyKYCProof(_ context.Context, zkpPointer string) (bool, error) {
	if zkpPointer == "" {
		return false, errors.New("zkp_pointer is required")
	}
	p.mu.RLock()
	cred, ok := p.credentials[zkpPointer]
	p.mu.RUnlock()
	if !ok {
		return false, nil
	}
	return !cred.Revoked, nil
}

// GetKYCStatus returns the current KYC lifecycle status for a subject.
func (p *LocalProvider) GetKYCStatus(_ context.Context, subject string) (identityprovider.KYCStatus, error) {
	if subject == "" {
		return "", errors.New("subject is required")
	}
	p.mu.RLock()
	status, ok := p.kycStatus[subject]
	p.mu.RUnlock()
	if !ok {
		return identityprovider.KYCStatusPending, nil
	}
	return status, nil
}

// ProvisionParticipant sets the KYC status for a subject (Central Bank only).
// This is used for approval (APPROVED), freeze (FROZEN), unfreeze (APPROVED),
// and revocation (REVOKED) actions (REQ-COM-003).
func (p *LocalProvider) ProvisionParticipant(_ context.Context, subject string, status identityprovider.KYCStatus) error {
	if subject == "" {
		return errors.New("subject is required")
	}
	switch status {
	case identityprovider.KYCStatusApproved, identityprovider.KYCStatusPending,
		identityprovider.KYCStatusFrozen, identityprovider.KYCStatusRevoked:
	default:
		return fmt.Errorf("invalid kyc status: %q", status)
	}

	p.mu.Lock()
	p.kycStatus[subject] = status
	// If revoking, also revoke the credential pointer
	if status == identityprovider.KYCStatusRevoked {
		if pointer, ok := p.credBySubject[subject]; ok {
			if cred, ok := p.credentials[pointer]; ok {
				cred.Revoked = true
				p.credentials[pointer] = cred
			}
		}
	}
	p.mu.Unlock()
	return nil
}

// --- internal helpers ---

func (p *LocalProvider) issueAccessToken(u localUser, wallet identityprovider.WalletResponse) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":                u.Username,
		"preferred_username": u.Username,
		"roles":              []string{u.Role},
		"iss":                "cbweb3-auth-service",
		"aud":                "cbweb3-internal",
		"iat":                now.Unix(),
		"exp":                now.Add(p.cfg.AccessTokenTT).Unix(),
	}
	// Enrich with LNET-style claims (D7 §7.4)
	if wallet.DID != "" {
		claims["did"] = wallet.DID
	}
	if wallet.Address != "" {
		claims["wallet"] = wallet.Address
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(p.cfg.JWTSecret))
}

func extractRoles(raw any) []string {
	if direct, ok := raw.([]string); ok {
		out := make([]string, 0, len(direct))
		for _, role := range direct {
			if strings.TrimSpace(role) != "" {
				out = append(out, role)
			}
		}
		return out
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if role, ok := item.(string); ok && strings.TrimSpace(role) != "" {
			out = append(out, role)
		}
	}
	return out
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
