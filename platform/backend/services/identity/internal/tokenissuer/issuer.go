package tokenissuer

import "context"

// IssueRequest carries the claims to embed in the internal JWT (D7 §7.4).
type IssueRequest struct {
	Subject      string
	Roles        []string
	// Enriched claims sourced from the identity provider after wallet provisioning.
	DID          string // W3C DID of the participant
	Wallet       string // EVM wallet address (0x...)
	Country      string // ISO 3166-1 country code
	BankID       string // short bank identifier for routing
	PrivacyGroup string // Paladin privacy group (placeholder for MVP)
}

// IssueResponse carries the signed internal access token.
type IssueResponse struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int
}

// Issuer defines the internal JWT lifecycle (issue + validate).
// The token returned to callers is ALWAYS an internal token —
// never the raw token from an external provider.
type Issuer interface {
	Name() string
	Issue(ctx context.Context, req IssueRequest) (IssueResponse, error)
	Validate(ctx context.Context, accessToken string) (IssueRequest, error)
}
