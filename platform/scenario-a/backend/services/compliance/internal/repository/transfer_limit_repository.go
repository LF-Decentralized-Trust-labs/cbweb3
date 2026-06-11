package repository

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ── memory implementation ────────────────────────────────────────────────────

type memoryTransferLimitStore struct {
	mu      sync.RWMutex
	limits  map[string]TransferLimit  // key = limitID
	volumes map[string]string         // key = participantID|currency|date → accumulated wei
}

var globalMemTLStore = &memoryTransferLimitStore{
	limits:  map[string]TransferLimit{},
	volumes: map[string]string{},
}

func (m *memoryRepository) CreateTransferLimit(_ context.Context, limit TransferLimit) error {
	globalMemTLStore.mu.Lock()
	defer globalMemTLStore.mu.Unlock()
	globalMemTLStore.limits[limit.LimitID] = limit
	return nil
}

func (m *memoryRepository) ListTransferLimits(_ context.Context, centralBankID string) ([]TransferLimit, error) {
	globalMemTLStore.mu.RLock()
	defer globalMemTLStore.mu.RUnlock()
	var result []TransferLimit
	for _, l := range globalMemTLStore.limits {
		if l.CentralBankID == centralBankID && l.IsActive {
			result = append(result, l)
		}
	}
	return result, nil
}

func (m *memoryRepository) FindApplicableLimit(_ context.Context, centralBankID, participantID, currency string) (*TransferLimit, error) {
	globalMemTLStore.mu.RLock()
	defer globalMemTLStore.mu.RUnlock()
	for _, l := range globalMemTLStore.limits {
		if !l.IsActive || l.CentralBankID != centralBankID {
			continue
		}
		pidMatch := l.ParticipantID == "" || l.ParticipantID == participantID
		curMatch := l.Currency == "" || strings.EqualFold(l.Currency, currency)
		if pidMatch && curMatch {
			lCopy := l
			return &lCopy, nil
		}
	}
	return nil, nil
}

func (m *memoryRepository) DeleteTransferLimit(_ context.Context, limitID string) error {
	globalMemTLStore.mu.Lock()
	defer globalMemTLStore.mu.Unlock()
	delete(globalMemTLStore.limits, limitID)
	return nil
}

func volumeKey(participantID, currency string, windowDate time.Time) string {
	return participantID + "|" + currency + "|" + windowDate.Format("2006-01-02")
}

func (m *memoryRepository) DeductTransferVolume(_ context.Context, participantID, currency, amountWei string, windowDate time.Time) error {
	globalMemTLStore.mu.Lock()
	defer globalMemTLStore.mu.Unlock()
	k := volumeKey(participantID, currency, windowDate)
	cur, _ := new(big.Int).SetString(globalMemTLStore.volumes[k], 10)
	if cur == nil {
		cur = new(big.Int)
	}
	amt, ok := new(big.Int).SetString(amountWei, 10)
	if !ok {
		return errors.New("invalid amountWei: " + amountWei)
	}
	globalMemTLStore.volumes[k] = new(big.Int).Add(cur, amt).String()
	return nil
}

func (m *memoryRepository) RestoreTransferVolume(_ context.Context, participantID, currency, amountWei string, windowDate time.Time) error {
	globalMemTLStore.mu.Lock()
	defer globalMemTLStore.mu.Unlock()
	k := volumeKey(participantID, currency, windowDate)
	cur, _ := new(big.Int).SetString(globalMemTLStore.volumes[k], 10)
	if cur == nil {
		cur = new(big.Int)
	}
	amt, ok := new(big.Int).SetString(amountWei, 10)
	if !ok {
		return nil
	}
	result := new(big.Int).Sub(cur, amt)
	if result.Sign() < 0 {
		result = new(big.Int)
	}
	globalMemTLStore.volumes[k] = result.String()
	return nil
}

func (m *memoryRepository) GetAccumulatedVolume(_ context.Context, participantID, currency string, windowDate time.Time) (string, error) {
	globalMemTLStore.mu.RLock()
	defer globalMemTLStore.mu.RUnlock()
	k := volumeKey(participantID, currency, windowDate)
	v := globalMemTLStore.volumes[k]
	if v == "" {
		return "0", nil
	}
	return v, nil
}

// ── GORM implementation ──────────────────────────────────────────────────────

func (r *gormRepository) CreateTransferLimit(ctx context.Context, limit TransferLimit) error {
	return r.db.WithContext(ctx).Create(&TransferLimitModel{
		LimitID:       limit.LimitID,
		CentralBankID: limit.CentralBankID,
		ParticipantID: limit.ParticipantID,
		Currency:      limit.Currency,
		MaxAmount:     limit.MaxAmount,
		IsActive:      limit.IsActive,
	}).Error
}

func (r *gormRepository) ListTransferLimits(ctx context.Context, centralBankID string) ([]TransferLimit, error) {
	var models []TransferLimitModel
	if err := r.db.WithContext(ctx).
		Where("central_bank_id = ? AND is_active = true", centralBankID).
		Order("created_at DESC").
		Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]TransferLimit, len(models))
	for i, m := range models {
		result[i] = transferLimitFromModel(m)
	}
	return result, nil
}

func (r *gormRepository) FindApplicableLimit(ctx context.Context, centralBankID, participantID, currency string) (*TransferLimit, error) {
	// Most specific match first: exact participant + currency
	candidates := []struct{ pid, cur string }{
		{participantID, currency},
		{participantID, ""},
		{"", currency},
		{"", ""},
	}
	for _, c := range candidates {
		var m TransferLimitModel
		err := r.db.WithContext(ctx).
			Where("central_bank_id = ? AND participant_id = ? AND currency = ? AND is_active = true", centralBankID, c.pid, c.cur).
			First(&m).Error
		if err == nil {
			l := transferLimitFromModel(m)
			return &l, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, nil
}

func (r *gormRepository) DeleteTransferLimit(ctx context.Context, limitID string) error {
	return r.db.WithContext(ctx).
		Model(&TransferLimitModel{}).
		Where("limit_id = ?", limitID).
		Update("is_active", false).Error
}

func (r *gormRepository) DeductTransferVolume(ctx context.Context, participantID, currency, amountWei string, windowDate time.Time) error {
	day := utcDay(windowDate)
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "participant_id"}, {Name: "currency"}, {Name: "window_date"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"accumulated_wei": gorm.Expr("transfer_volumes.accumulated_wei::numeric + ?::numeric", amountWei),
				"updated_at":      time.Now().UTC(),
			}),
		}).
		Create(&TransferVolumeModel{
			ParticipantID:  participantID,
			Currency:       currency,
			WindowDate:     day,
			AccumulatedWei: amountWei,
		}).Error
}

func (r *gormRepository) RestoreTransferVolume(ctx context.Context, participantID, currency, amountWei string, windowDate time.Time) error {
	day := utcDay(windowDate)
	return r.db.WithContext(ctx).
		Model(&TransferVolumeModel{}).
		Where("participant_id = ? AND currency = ? AND window_date = ?", participantID, currency, day).
		UpdateColumn("accumulated_wei", gorm.Expr(
			"GREATEST(accumulated_wei::numeric - ?::numeric, 0)::text", amountWei,
		)).Error
}

func (r *gormRepository) GetAccumulatedVolume(ctx context.Context, participantID, currency string, windowDate time.Time) (string, error) {
	day := utcDay(windowDate)
	var m TransferVolumeModel
	err := r.db.WithContext(ctx).
		Where("participant_id = ? AND currency = ? AND window_date = ?", participantID, currency, day).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "0", nil
	}
	if err != nil {
		return "", err
	}
	if m.AccumulatedWei == "" {
		return "0", nil
	}
	return m.AccumulatedWei, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func transferLimitFromModel(m TransferLimitModel) TransferLimit {
	return TransferLimit{
		LimitID:       m.LimitID,
		CentralBankID: m.CentralBankID,
		ParticipantID: m.ParticipantID,
		Currency:      m.Currency,
		MaxAmount:     m.MaxAmount,
		IsActive:      m.IsActive,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func utcDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
