package repository

import "time"

// ParticipantModel is the canonical GORM model for registered participants.
// It replaces the data-access ParticipantModel, removing ZKP/DID fields and
// adding PKI certificate fields.
type ParticipantModel struct {
	UserID              string     `gorm:"column:user_id;primaryKey"`
	InstitutionName     string     `gorm:"column:institution_name"`
	CNPJ                string     `gorm:"column:cnpj"`
	BankCode            string     `gorm:"column:bank_code"`
	CountryCode         string     `gorm:"column:country_code"`
	Role                string     `gorm:"column:participant_role"`
	WalletAddress       string     `gorm:"column:wallet_address;uniqueIndex"`
	Status              string     `gorm:"column:status;default:PENDING"`
	CertificateData     string     `gorm:"column:certificate_data;type:text"`
	CertificateExpiry   *time.Time `gorm:"column:certificate_expiry"`
	BlockchainPubKeyHex string     `gorm:"column:blockchain_pub_key_hex"`
	CsrPem              string     `gorm:"column:csr_pem;type:text"`
	PopNonce            string     `gorm:"column:pop_nonce"`
	PopNonceExpiresAt   *time.Time `gorm:"column:pop_nonce_expires_at"`
	CreatedAt           time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (ParticipantModel) TableName() string { return "participants" }

// AuditLogModel is an append-only audit trail (REQ-COM-006).
// Adds Category and Severity fields for the governance portal filters.
type AuditLogModel struct {
	LogID         string    `gorm:"column:log_id;primaryKey;type:uuid;default:gen_random_uuid()"`
	Timestamp     time.Time `gorm:"column:timestamp;autoCreateTime"`
	ActorSubject  string    `gorm:"column:actor_subject"`
	ActorAddress  string    `gorm:"column:actor_address"`
	ActionType    string    `gorm:"column:action_type"`
	TargetSubject string    `gorm:"column:target_subject"`
	CorrelationID string    `gorm:"column:correlation_id"`
	IPAddress     string    `gorm:"column:ip_address"`
	Result        string    `gorm:"column:result"`         // SUCCESS | FAILURE
	Category      string    `gorm:"column:category"`       // SESSION, CREDENTIAL, FREEZE, etc.
	Severity      string    `gorm:"column:severity"`       // INFO, WARNING, CRITICAL
	Details       string    `gorm:"column:details;type:jsonb"`
}

func (AuditLogModel) TableName() string { return "audit_logs" }

// SystemParameterModel stores key-value system configuration (AMM params, etc.).
type SystemParameterModel struct {
	Key       string    `gorm:"column:key;primaryKey"`
	Value     string    `gorm:"column:value;type:text"`
	UpdatedBy string    `gorm:"column:updated_by"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (SystemParameterModel) TableName() string { return "system_parameters" }

// TransferLimitModel stores CB-configured daily transfer limits per participant/currency (R1-10.1).
type TransferLimitModel struct {
	LimitID       string    `gorm:"column:limit_id;primaryKey;type:varchar(64)"`
	CentralBankID string    `gorm:"column:central_bank_id;not null;index"`
	ParticipantID string    `gorm:"column:participant_id;not null;default:''"`
	Currency      string    `gorm:"column:currency;not null;default:''"`
	MaxAmount     string    `gorm:"column:max_amount;not null"`
	IsActive      bool      `gorm:"column:is_active;not null;default:true"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TransferLimitModel) TableName() string { return "transfer_limits" }

// TransferVolumeModel tracks daily accumulated transfer volume per participant/currency (R1-10.1).
type TransferVolumeModel struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	ParticipantID  string    `gorm:"column:participant_id;not null;uniqueIndex:idx_vol_lookup"`
	Currency       string    `gorm:"column:currency;not null;uniqueIndex:idx_vol_lookup"`
	WindowDate     time.Time `gorm:"column:window_date;not null;uniqueIndex:idx_vol_lookup"`
	AccumulatedWei string    `gorm:"column:accumulated_wei;not null;default:'0'"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TransferVolumeModel) TableName() string { return "transfer_volumes" }
