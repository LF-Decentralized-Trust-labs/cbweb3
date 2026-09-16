// SPDX-License-Identifier: Apache-2.0

// Package services provides ParticipantResolver for looking up bank on-chain addresses.
package services

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// participantRow mirrors only the columns we need from the compliance service's participants table.
type participantRow struct {
	BankCode      string `gorm:"column:bank_code"`
	WalletAddress string `gorm:"column:wallet_address"`
	Status        string `gorm:"column:status"`
}

func (participantRow) TableName() string { return "participants" }

// ParticipantResolverIface resolves a bank_id to its on-chain wallet address.
type ParticipantResolverIface interface {
	// ResolveWalletAddress returns the on-chain wallet address for the given bank_code.
	// Returns an error if the participant is not found or not ACTIVE.
	ResolveWalletAddress(ctx context.Context, bankCode string) (string, error)
}

// ParticipantResolver looks up on-chain addresses from the local participants table.
// CB-B owns this table (populated by the onboarding/compliance flow), so it is the
// authoritative source for the wallet addresses of its member banks.
type ParticipantResolver struct {
	db *gorm.DB
}

// NewParticipantResolver creates a resolver backed by the given GORM database.
func NewParticipantResolver(db *gorm.DB) *ParticipantResolver {
	return &ParticipantResolver{db: db}
}

// ResolveWalletAddress looks up the on-chain address of a bank by its bank_code.
// Returns an error if the bank is not found, not ACTIVE, or has an empty wallet address.
func (r *ParticipantResolver) ResolveWalletAddress(ctx context.Context, bankCode string) (string, error) {
	if bankCode == "" {
		return "", fmt.Errorf("bank_code is required")
	}

	var row participantRow
	err := r.db.WithContext(ctx).
		Where("bank_code = ?", bankCode).
		First(&row).Error
	if err != nil {
		return "", fmt.Errorf("participant not found for bank_code=%q: %w", bankCode, err)
	}

	if !strings.EqualFold(row.Status, "ACTIVE") {
		return "", fmt.Errorf("participant bank_code=%q is not ACTIVE (status=%s)", bankCode, row.Status)
	}

	addr := strings.TrimSpace(row.WalletAddress)
	if addr == "" {
		return "", fmt.Errorf("participant bank_code=%q has no wallet_address", bankCode)
	}

	return addr, nil
}

// BeneficiaryEligibility is the answer CB-B gives another central bank about one of its own
// member banks. It is deliberately the SMALLEST useful answer.
//
// Eligible and a machine-readable code, and nothing else: no wallet address, no status string,
// no registry row. The asking central bank needs to know whether to start a payment, not who
// the beneficiary is or how far through onboarding it got. Address resolution stays where it
// already happens — inside the bridge-out, on this CB's own side.
type BeneficiaryEligibility struct {
	Eligible bool   `json:"eligible"`
	Code     string `json:"code"`
}

// Codes a central bank may return about its own member. They name the CONDITION, not the
// participant: "not onboarded" is as much as another sovereign needs.
const (
	// EligibilityOK means the bank can receive a cross-currency delivery right now.
	EligibilityOK = "ELIGIBLE"
	// EligibilityUnknownBank means no participant with that bank_code exists here.
	EligibilityUnknownBank = "UNKNOWN_BANK"
	// EligibilityNotActive means the participant exists but has not completed onboarding.
	// This is the condition behind the incident: KYC approved, portal never reopened, so the
	// bank sat in KYC_APPROVED and the delivery was rejected after the money had moved.
	EligibilityNotActive = "NOT_ACTIVE"
	// EligibilityNoWallet means the participant is active but carries no on-chain address, so
	// the release would have nowhere to go.
	EligibilityNoWallet = "NO_WALLET"
)

// CheckBeneficiaryEligibility answers whether a member bank can receive a delivery.
//
// It runs the SAME conditions ResolveWalletAddress does, in the same order, on purpose: a
// pre-flight that could disagree with the check that actually gates the release would be worse
// than no pre-flight, because it would authorise a payment the delivery then refuses.
//
// It returns no error for an ineligible bank — ineligibility is an answer, not a failure. An
// error here means the question could not be answered at all.
func (r *ParticipantResolver) CheckBeneficiaryEligibility(ctx context.Context, bankCode string) (BeneficiaryEligibility, error) {
	bankCode = strings.TrimSpace(bankCode)
	if bankCode == "" {
		return BeneficiaryEligibility{}, fmt.Errorf("bank_code is required")
	}

	var row participantRow
	if err := r.db.WithContext(ctx).Where("bank_code = ?", bankCode).First(&row).Error; err != nil {
		// A missing participant is an answer, not a lookup failure: this central bank knows its
		// own members, so "not one of mine" is authoritative.
		return BeneficiaryEligibility{Eligible: false, Code: EligibilityUnknownBank}, nil
	}
	if !strings.EqualFold(row.Status, "ACTIVE") {
		return BeneficiaryEligibility{Eligible: false, Code: EligibilityNotActive}, nil
	}
	if strings.TrimSpace(row.WalletAddress) == "" {
		return BeneficiaryEligibility{Eligible: false, Code: EligibilityNoWallet}, nil
	}
	return BeneficiaryEligibility{Eligible: true, Code: EligibilityOK}, nil
}
