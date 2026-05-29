// Package audit provides an append-only audit log helper for Scenario B.
// Entries are inserted via GORM Create; UPDATE and DELETE are blocked by
// database-level PL/pgSQL triggers (see internal/db/init/triggers.go).
package audit

import (
	"time"

	"gorm.io/gorm"
)

// Entry represents a single immutable audit log record.
type Entry struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	EntryTable string   `gorm:"column:table_name;not null"`
	EventKind string    `gorm:"column:event_kind;not null"`
	RefID     string    `gorm:"column:ref_id;not null"`
	Meta      string    `gorm:"column:meta;type:jsonb"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName returns the fixed audit log table name.
func (Entry) TableName() string { return "audit_log" }

// Append inserts an immutable audit entry. meta is serialised as JSON-encoded string.
// This function must never UPDATE or DELETE existing rows (FR-048 / FR-049).
func Append(db *gorm.DB, tableName, eventKind, refID string, meta map[string]string) error {
	encoded := encodeMetaMap(meta)
	return db.Create(&Entry{
		EntryTable: tableName,
		EventKind: eventKind,
		RefID:     refID,
		Meta:      encoded,
	}).Error
}

// encodeMetaMap serialises a string map as a simple JSON object without importing encoding/json
// to keep the audit package dependency-light.
func encodeMetaMap(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	out := "{"
	first := true
	for k, v := range m {
		if !first {
			out += ","
		}
		out += `"` + jsonEscape(k) + `":"` + jsonEscape(v) + `"`
		first = false
	}
	out += "}"
	return out
}

func jsonEscape(s string) string {
	result := ""
	for _, r := range s {
		switch r {
		case '"':
			result += `\"`
		case '\\':
			result += `\\`
		case '\n':
			result += `\n`
		case '\r':
			result += `\r`
		case '\t':
			result += `\t`
		default:
			result += string(r)
		}
	}
	return result
}
