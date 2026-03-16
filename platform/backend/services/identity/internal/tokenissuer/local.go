package tokenissuer

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type LocalIssuer struct {
	secret   string
	issuer   string
	audience string
	ttl      time.Duration
}

func NewLocalIssuer(secret, issuer, audience string, ttl time.Duration) Issuer {
	if strings.TrimSpace(secret) == "" {
		secret = "identity-internal-secret"
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	if strings.TrimSpace(issuer) == "" {
		issuer = "identity-internal"
	}
	if strings.TrimSpace(audience) == "" {
		audience = "cbweb3-internal"
	}
	return &LocalIssuer{
		secret:   secret,
		issuer:   issuer,
		audience: audience,
		ttl:      ttl,
	}
}

func (i *LocalIssuer) Name() string { return "local" }

// Issue creates a signed HS256 JWT with enriched claims (D7 §7.4).
func (i *LocalIssuer) Issue(_ context.Context, req IssueRequest) (IssueResponse, error) {
	if strings.TrimSpace(req.Subject) == "" {
		return IssueResponse{}, errors.New("subject is required")
	}
	now := time.Now().UTC()
	c := jwt.MapClaims{
		"sub":   req.Subject,
		"iss":   i.issuer,
		"aud":   i.audience,
		"roles": req.Roles,
		"iat":   now.Unix(),
		"exp":   now.Add(i.ttl).Unix(),
	}
	// Enrich with LNET/D7 claims when available
	if req.DID != "" {
		c["did"] = req.DID
	}
	if req.Wallet != "" {
		c["wallet"] = req.Wallet
	}
	if req.Country != "" {
		c["country"] = req.Country
	}
	if req.BankID != "" {
		c["bank_id"] = req.BankID
	}
	if req.PrivacyGroup != "" {
		c["privacy_group"] = req.PrivacyGroup
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := token.SignedString([]byte(i.secret))
	if err != nil {
		return IssueResponse{}, err
	}
	return IssueResponse{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresIn:   int(i.ttl.Seconds()),
	}, nil
}

// Validate parses the internal HS256 JWT and extracts all claims.
func (i *LocalIssuer) Validate(_ context.Context, accessToken string) (IssueRequest, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return IssueRequest{}, errors.New("access token is required")
	}
	token, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
		return []byte(i.secret), nil
	})
	if err != nil || !token.Valid {
		return IssueRequest{}, errors.New("invalid internal token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return IssueRequest{}, errors.New("invalid internal token claims")
	}
	subject, _ := claims["sub"].(string)
	if strings.TrimSpace(subject) == "" {
		return IssueRequest{}, errors.New("missing subject claim")
	}
	var roles []string
	if raw, ok := claims["roles"].([]any); ok {
		for _, item := range raw {
			if role, ok := item.(string); ok && strings.TrimSpace(role) != "" {
				roles = append(roles, role)
			}
		}
	}
	did, _ := claims["did"].(string)
	wallet, _ := claims["wallet"].(string)
	country, _ := claims["country"].(string)
	bankID, _ := claims["bank_id"].(string)
	privacyGroup, _ := claims["privacy_group"].(string)

	return IssueRequest{
		Subject:      subject,
		Roles:        roles,
		DID:          did,
		Wallet:       wallet,
		Country:      country,
		BankID:       bankID,
		PrivacyGroup: privacyGroup,
	}, nil
}
