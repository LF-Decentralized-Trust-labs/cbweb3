// Package init provides append-only PL/pgSQL triggers for Scenario B audit tables.
package init

import "gorm.io/gorm"

// appendOnlyTables lists all Scenario B tables that must be append-only (FR-048 / FR-049).
var appendOnlyTables = []string{
	"circuit_breaker_signatures",
	"disclosure_signatures",
	"liquidity_alerts",
	"swap_order_event_history",
	"bridged_asset_position_history",
	"circuit_breaker_event_history",
	"disclosure_event_history",
}

// CreateAppendOnlyTriggers installs BEFORE UPDATE and BEFORE DELETE triggers on all
// append-only audit tables. Each trigger raises an exception to prevent mutation.
func CreateAppendOnlyTriggers(db *gorm.DB) error {
	// Create the shared guard function (idempotent via OR REPLACE).
	guardFn := `
CREATE OR REPLACE FUNCTION append_only_guard() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'append_only_violation: table % is append-only', TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;`

	if err := db.Exec(guardFn).Error; err != nil {
		return err
	}

	for _, tbl := range appendOnlyTables {
		updateTrigger := `
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger
    WHERE tgname = 'trg_` + tbl + `_no_update'
      AND tgrelid = '` + tbl + `'::regclass
  ) THEN
    CREATE TRIGGER trg_` + tbl + `_no_update
    BEFORE UPDATE ON ` + tbl + `
    FOR EACH ROW EXECUTE FUNCTION append_only_guard();
  END IF;
END$$;`

		deleteTrigger := `
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger
    WHERE tgname = 'trg_` + tbl + `_no_delete'
      AND tgrelid = '` + tbl + `'::regclass
  ) THEN
    CREATE TRIGGER trg_` + tbl + `_no_delete
    BEFORE DELETE ON ` + tbl + `
    FOR EACH ROW EXECUTE FUNCTION append_only_guard();
  END IF;
END$$;`

		if err := db.Exec(updateTrigger).Error; err != nil {
			return err
		}
		if err := db.Exec(deleteTrigger).Error; err != nil {
			return err
		}
	}
	return nil
}
