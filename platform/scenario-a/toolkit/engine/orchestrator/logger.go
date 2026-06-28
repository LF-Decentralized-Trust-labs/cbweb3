// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// logEntry is the internal representation of one structured log line.
type logEntry struct {
	Severity string `json:"severity"`
	SpokeID  string `json:"spoke_id"`
	Step     string `json:"step"`
	Action   string `json:"action"`
	Error    string `json:"error,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// logLine emits a single JSON log event to w. Fields:
//
//   - ts:        ISO-8601 UTC timestamp
//   - severity:  "INFO" | "ERROR"
//   - service:   always "orchestrator"
//   - spoke_id:  spoke identifier from manifest
//   - step:      canonical step name
//   - action:    "step_started" | "step_completed" | "step_skipped" | "step_failed"
//   - error:     present only when action = "step_failed"
//   - reason:    present only when action = "step_skipped" (always "already-done")
func logLine(w io.Writer, entry logEntry) {
	record := map[string]string{
		"ts":       time.Now().UTC().Format(time.RFC3339Nano),
		"severity": entry.Severity,
		"service":  "orchestrator",
		"spoke_id": entry.SpokeID,
		"step":     entry.Step,
		"action":   entry.Action,
	}
	if entry.Error != "" {
		record["error"] = entry.Error
	}
	if entry.Reason != "" {
		record["reason"] = entry.Reason
	}
	if entry.Detail != "" {
		record["detail"] = entry.Detail
	}

	data, err := json.Marshal(record)
	if err != nil {
		fmt.Fprintf(w, `{"severity":"ERROR","service":"orchestrator","action":"log_marshal_failed","error":%q}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(w, string(data))
}

func logStarted(w io.Writer, spokeID, step string) {
	logLine(w, logEntry{Severity: "INFO", SpokeID: spokeID, Step: step, Action: "step_started"})
}

func logCompleted(w io.Writer, spokeID, step string) {
	logLine(w, logEntry{Severity: "INFO", SpokeID: spokeID, Step: step, Action: "step_completed"})
}

func logSkipped(w io.Writer, spokeID, step string) {
	logLine(w, logEntry{Severity: "INFO", SpokeID: spokeID, Step: step, Action: "step_skipped", Reason: "already-done"})
}

func logFailed(w io.Writer, spokeID, step string, err error) {
	logLine(w, logEntry{Severity: "ERROR", SpokeID: spokeID, Step: step, Action: "step_failed", Error: err.Error()})
}

// logDetail emits a structured INFO line carrying a step-specific diagnostic
// (e.g. vote progress). Keeps in-step diagnostics in the same JSON format as the
// lifecycle events, per Constitution VI. A nil writer is a no-op.
func logDetail(w io.Writer, spokeID, step, detail string) {
	if w == nil {
		return
	}
	logLine(w, logEntry{Severity: "INFO", SpokeID: spokeID, Step: step, Action: "step_detail", Detail: detail})
}
