package repository

// ParticipantModel is the GORM model for participants persistence.
type ParticipantModel struct {
	UserID         string `gorm:"column:user_id;primaryKey"`
	DID            string `gorm:"column:did;not null"`
	WalletAddress  string `gorm:"column:wallet_address;not null"`
	Country        string `gorm:"column:country"`
	BankCode       string `gorm:"column:bank_code"`
	Role           string `gorm:"column:role"`
	SignerProvider string `gorm:"column:signer_provider"`
	KMSKeyID       string `gorm:"column:kms_key_id"`
}

func (ParticipantModel) TableName() string { return "participants" }
