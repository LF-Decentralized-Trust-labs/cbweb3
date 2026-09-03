// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLogLine_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	logLine(&buf, logEntry{
		Severity: "INFO",
		SpokeID:  "spoke-test",
		Step:     StepDeployContracts,
		Action:   "step_started",
	})
	var rec map[string]string
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("logLine output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
}

func TestLogLine_RequiredFields(t *testing.T) {
	var buf bytes.Buffer
	logLine(&buf, logEntry{
		Severity: "INFO",
		SpokeID:  "spoke-test",
		Step:     StepGenTLS,
		Action:   "step_completed",
	})
	var rec map[string]string
	_ = json.Unmarshal(buf.Bytes(), &rec)
	for _, field := range []string{"ts", "severity", "service", "spoke_id", "step", "action"} {
		if rec[field] == "" {
			t.Errorf("required field %q is empty in log output: %s", field, buf.String())
		}
	}
	if rec["service"] != "orchestrator" {
		t.Errorf("service = %q; want orchestrator", rec["service"])
	}
}

func TestLogLine_TimestampISO8601(t *testing.T) {
	var buf bytes.Buffer
	logLine(&buf, logEntry{Severity: "INFO", SpokeID: "s", Step: "x", Action: "step_started"})
	var rec map[string]string
	_ = json.Unmarshal(buf.Bytes(), &rec)
	_, err := time.Parse(time.RFC3339Nano, rec["ts"])
	if err != nil {
		_, err = time.Parse(time.RFC3339, rec["ts"])
	}
	if err != nil {
		t.Errorf("ts %q is not a valid ISO-8601 timestamp: %v", rec["ts"], err)
	}
}

func TestLogLine_SeverityValues(t *testing.T) {
	for _, sev := range []string{"INFO", "ERROR"} {
		var buf bytes.Buffer
		logLine(&buf, logEntry{Severity: sev, SpokeID: "s", Step: "x", Action: "step_started"})
		var rec map[string]string
		_ = json.Unmarshal(buf.Bytes(), &rec)
		if rec["severity"] != sev {
			t.Errorf("severity = %q; want %q", rec["severity"], sev)
		}
	}
}

func TestLogLine_ErrorFieldAbsentOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	logLine(&buf, logEntry{Severity: "INFO", SpokeID: "s", Step: "x", Action: "step_completed"})
	var rec map[string]string
	_ = json.Unmarshal(buf.Bytes(), &rec)
	if _, ok := rec["error"]; ok {
		t.Errorf("error field should not be present on step_completed: %s", buf.String())
	}
}

func TestLogLine_ReasonAbsentOnFailure(t *testing.T) {
	var buf bytes.Buffer
	logLine(&buf, logEntry{Severity: "ERROR", SpokeID: "s", Step: "x", Action: "step_failed", Error: "something went wrong"})
	var rec map[string]string
	_ = json.Unmarshal(buf.Bytes(), &rec)
	if _, ok := rec["reason"]; ok {
		t.Errorf("reason field should not be present on step_failed: %s", buf.String())
	}
	if rec["error"] == "" {
		t.Errorf("error field must be present on step_failed: %s", buf.String())
	}
}

func TestLogLine_SkippedHasReason(t *testing.T) {
	var buf bytes.Buffer
	logSkipped(&buf, "spoke-test", StepRenderConfigs)
	var rec map[string]string
	_ = json.Unmarshal(buf.Bytes(), &rec)
	if rec["action"] != "step_skipped" {
		t.Errorf("action = %q; want step_skipped", rec["action"])
	}
	if rec["reason"] != "already-done" {
		t.Errorf("reason = %q; want already-done", rec["reason"])
	}
}

func TestLogLine_FailedHasError(t *testing.T) {
	var buf bytes.Buffer
	logFailed(&buf, "spoke-test", StepDeployContracts, errors.New("forge test failed"))
	var rec map[string]string
	_ = json.Unmarshal(buf.Bytes(), &rec)
	if rec["action"] != "step_failed" {
		t.Errorf("action = %q; want step_failed", rec["action"])
	}
	if !strings.Contains(rec["error"], "forge test failed") {
		t.Errorf("error = %q; want to contain 'forge test failed'", rec["error"])
	}
	if rec["severity"] != "ERROR" {
		t.Errorf("severity = %q; want ERROR", rec["severity"])
	}
}

func TestLogLine_EndsWithNewline(t *testing.T) {
	var buf bytes.Buffer
	logLine(&buf, logEntry{Severity: "INFO", SpokeID: "s", Step: "x", Action: "step_started"})
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("log line does not end with newline: %q", buf.String())
	}
}
