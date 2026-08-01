// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
	"gorm.io/gorm"
)

// relayPinRow is a peer identity as this central bank recorded it at onboarding.
//
// The certificate is the one the CB itself issued when signing that bank's CSR, and the CSR is
// generated over the same PKI_DIR/<bankCode>.key the bank's relay signer uses — verified against a
// live stack, where the public key of bank-itau.key and of its issued certificate hash identically.
// That correspondence is what makes this table a pin source and not merely a record.
type relayPinRow struct {
	BankCode        string `gorm:"column:bank_code"`
	Status          string `gorm:"column:status"`
	CertificateData string `gorm:"column:certificate_data"`
}

func (relayPinRow) TableName() string { return "participants" }

// loadParticipantPins reads the peers this gateway can verify from its own participant records.
//
// Rows with no certificate are skipped: the bank has not completed credential issuance, so there is
// nothing to pin. Status is carried through rather than filtered here, because an INACTIVE
// participant has to actively suppress any file pin for the same id — see relayauth.BuildRegistry.
// Filtering inactive rows away at the query would silently restore the file pin and defeat
// revocation.
func loadParticipantPins(ctx context.Context, db *gorm.DB) ([]relayauth.ParticipantPin, error) {
	if db == nil {
		return nil, nil
	}
	var rows []relayPinRow
	if err := db.WithContext(ctx).
		Where("bank_code <> ''").
		Where("certificate_data <> ''").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	pins := make([]relayauth.ParticipantPin, 0, len(rows))
	for i := range rows {
		pins = append(pins, relayauth.ParticipantPin{
			ID:      rows[i].BankCode,
			CertPEM: rows[i].CertificateData,
			Active:  rows[i].Status == participantStatusActive,
		})
	}
	return pins, nil
}

// participantStatusActive is the compliance status that authorizes a peer. Deactivating a bank there
// is what revokes its ability to authenticate, which is the capability this pin source adds over
// certificate files.
const participantStatusActive = "ACTIVE"

// refreshRelayRegistry rebuilds the registry from both sources and swaps it in.
//
// Files are re-read on every refresh, not cached, so an operator can pin a non-participant peer (the
// Cacti relay is the case that matters — it calls a CB's internal endpoint on the bridge-out leg and
// will never be onboarded) without restarting the gateway.
func refreshRelayRegistry(ctx context.Context, store *relayauth.Store, db *gorm.DB, pkiDir string) {
	files, _ := relayauth.LoadRegistryGlob(pkiDir)
	pins, err := loadParticipantPins(ctx, db)
	if err != nil {
		// Keep the registry in force rather than replacing it with a file-only one: a transient
		// database error must not silently un-pin every onboarded bank and turn their signed
		// requests into 401s.
		log.Printf("[relay-auth] could not read participant pins, keeping the current registry: %v", err)
		return
	}
	next := relayauth.BuildRegistry(files, pins)
	prev := store.Get()
	store.Set(next)
	if prev == nil || !sameIDs(prev.IDs(), next.IDs()) {
		log.Printf("[relay-auth] registry refreshed: %d peer key(s) pinned %v", next.Len(), next.IDs())
	}
}

func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// startRelayRegistryRefresher re-reads the pin sources periodically.
//
// Without it the registry is a boot-time snapshot, and a bank onboarded afterwards would sign its
// internal calls with an id nobody has pinned — rejected with 401, and NOT downgraded to the shared
// secret, which is correct but takes the settlement path down until a restart. That is the same
// "non-empty but incomplete registry" hazard the boot warning describes, arriving later.
//
// Returns a stop function; nil when there is nothing to refresh.
func startRelayRegistryRefresher(store *relayauth.Store, db *gorm.DB, pkiDir string) func() {
	if store == nil || (db == nil && pkiDir == "") {
		return nil
	}
	interval := relayRegistryRefreshInterval()
	if interval <= 0 {
		log.Printf("[relay-auth] registry refresh disabled by RELAY_REGISTRY_REFRESH_SEC=0 — a bank onboarded after boot will not be verifiable until restart")
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshRelayRegistry(ctx, store, db, pkiDir)
			}
		}
	}()
	log.Printf("[relay-auth] registry refresher started (every %s)", interval)
	return cancel
}

const relayRegistryDefaultRefresh = 60 * time.Second

// relayRegistryRefreshInterval reads RELAY_REGISTRY_REFRESH_SEC. Zero disables the refresher; an
// invalid value falls back to the default with a warning rather than silently disabling the one thing
// that keeps a newly onboarded bank verifiable.
func relayRegistryRefreshInterval() time.Duration {
	raw := os.Getenv("RELAY_REGISTRY_REFRESH_SEC")
	if raw == "" {
		return relayRegistryDefaultRefresh
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs < 0 {
		log.Printf("[relay-auth] RELAY_REGISTRY_REFRESH_SEC=%q is not a non-negative integer — using %s", raw, relayRegistryDefaultRefresh)
		return relayRegistryDefaultRefresh
	}
	return time.Duration(secs) * time.Second
}
