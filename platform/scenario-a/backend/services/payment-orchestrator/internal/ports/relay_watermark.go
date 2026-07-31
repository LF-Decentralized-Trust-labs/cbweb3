// SPDX-License-Identifier: Apache-2.0

package ports

import "context"

// RelayWatermarkStore persists the last processed event cursor per relay stream
// so the Cacti poller can resume after a restart instead of resetting to the
// current time (which would silently drop events observed during downtime).
//
// The cursor is a monotonically non-decreasing int64 (a Unix-ms timestamp for
// the HTLC event poller). Callers key it by a stream identifier (e.g. "lock" or
// "settle").
type RelayWatermarkStore interface {
	// GetWatermark returns the persisted cursor for kind. found is false when no
	// watermark has been recorded yet, in which case the caller falls back to its
	// default (e.g. the current time).
	GetWatermark(ctx context.Context, kind string) (value int64, found bool, err error)

	// SetWatermark records value as the latest processed cursor for kind. It must
	// be safe to call repeatedly; implementations upsert by kind.
	SetWatermark(ctx context.Context, kind string, value int64) error
}
