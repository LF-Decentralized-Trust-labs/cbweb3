package domain

// Role constants — canonical participant roles across the platform.
const (
	RoleCentralBank    = "CENTRAL_BANK"
	RoleCommercialBank = "COMMERCIAL_BANK"
	RoleMLP            = "MLP"
)

// KYCStatus is the canonical KYC lifecycle enum (D7 §7.1).
type KYCStatus string

const (
	KYCStatusPending  KYCStatus = "PENDING"
	KYCStatusApproved KYCStatus = "APPROVED"
	KYCStatusFrozen   KYCStatus = "FROZEN"
	KYCStatusRevoked  KYCStatus = "REVOKED"
)

// TokenClaims is the normalized token claims payload (D7 §7.4).
type TokenClaims struct {
	Subject      string   `json:"subject"`
	Issuer       string   `json:"issuer"`
	Roles        []string `json:"roles"`
	DID          string   `json:"did,omitempty"`
	Wallet       string   `json:"wallet,omitempty"`
	Country      string   `json:"country,omitempty"`
	BankID       string   `json:"bank_id,omitempty"`
	PrivacyGroup string   `json:"privacy_group,omitempty"`
}
