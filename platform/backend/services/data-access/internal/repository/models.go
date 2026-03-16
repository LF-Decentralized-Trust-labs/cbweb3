package repository

import "time"

// ParticipantModel is the canonical GORM model for participants (D7 §7.1 + REQ-CAP-004).
type ParticipantModel struct {
	UserID          string     `gorm:"column:user_id;primaryKey"`
	DID             string     `gorm:"column:did;uniqueIndex;not null"`
	WalletAddress   string     `gorm:"column:wallet_address;uniqueIndex;not null"`
	InstitutionName string     `gorm:"column:institution_name"`
	BankCode        string     `gorm:"column:bank_code"`
	CountryCode     string     `gorm:"column:country_code"`        // ISO 3166-1
	Role            string     `gorm:"column:participant_role"`    // CENTRAL_BANK, COMMERCIAL_BANK, MLP
	WalletType      string     `gorm:"column:wallet_type"`         // Direct, Correspondent, Escrow (REQ-CAP-004)
	KYCStatus       string     `gorm:"column:kyc_status;default:PENDING"`
	KYCVCHash       string     `gorm:"column:kyc_vc_hash"`         // ZKP pointer (SHA-256)
	KYCLastVerify   *time.Time `gorm:"column:kyc_last_verify"`
	PrivacyDomainID string     `gorm:"column:privacy_domain_id"`   // Paladin privacy domain
	CactiRelayAddr  string     `gorm:"column:cacti_relay_address"` // Cacti relay endpoint
	SignerProvider  string     `gorm:"column:signer_provider"`     // "local" | "dwallet_api"
	// KMSKeyID kept for backward-compat; not used with LNET D-Wallet custody.
	KMSKeyID   string    `gorm:"column:kms_key_id"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ParticipantModel) TableName() string { return "participants" }

// KYCCredentialModel stores Verifiable Credential pointers per participant.
type KYCCredentialModel struct {
	Subject    string    `gorm:"column:subject;primaryKey"`
	ZKPPointer string    `gorm:"column:zkp_pointer;uniqueIndex;not null"` // SHA-256 of VC payload
	VCJWT      string    `gorm:"column:vc_jwt;type:text"`
	IssuedAt   string    `gorm:"column:issued_at"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (KYCCredentialModel) TableName() string { return "kyc_credentials" }

// AuditLogModel is an append-only audit trail (REQ-COM-006 + D6 Evidence Bundle).
type AuditLogModel struct {
	LogID         string    `gorm:"column:log_id;primaryKey;type:uuid;default:gen_random_uuid()"`
	Timestamp     time.Time `gorm:"column:timestamp;autoCreateTime"`
	ActorSubject  string    `gorm:"column:actor_subject"`   // userID / sub of the JWT actor
	ActorAddress  string    `gorm:"column:actor_address"`   // wallet 0x...
	ActionType    string    `gorm:"column:action_type"`     // LOGIN, REGISTER, ISSUE_KYC, FREEZE, etc.
	TargetSubject string    `gorm:"column:target_subject"`  // subject affected (if different from actor)
	CorrelationID string    `gorm:"column:correlation_id"`  // X-Correlation-Id (NFR-OPS-001)
	IPAddress     string    `gorm:"column:ip_address"`
	Result        string    `gorm:"column:result"`          // SUCCESS | FAILURE
	Details       string    `gorm:"column:details;type:jsonb"` // additional context as JSON
}

func (AuditLogModel) TableName() string { return "audit_logs" }
