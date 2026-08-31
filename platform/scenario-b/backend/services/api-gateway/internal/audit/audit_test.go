// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEntry_TableName(t *testing.T) {
	if (Entry{}).TableName() != "audit_log" {
		t.Fatalf("unexpected table name: %s", (Entry{}).TableName())
	}
}

func TestEncodeMetaMap_Empty(t *testing.T) {
	if got := encodeMetaMap(nil); got != "{}" {
		t.Fatalf("expected empty object for nil, got %s", got)
	}
	if got := encodeMetaMap(map[string]string{}); got != "{}" {
		t.Fatalf("expected empty object for empty map, got %s", got)
	}
}

func TestEncodeMetaMap_SingleAndMulti(t *testing.T) {
	// Single key — deterministic, exact match.
	if got := encodeMetaMap(map[string]string{"k": "v"}); got != `{"k":"v"}` {
		t.Fatalf("unexpected single-key encoding: %s", got)
	}

	// Multi-key — map ordering is non-deterministic, so round-trip through json.
	encoded := encodeMetaMap(map[string]string{"a": "1", "b": "2"})
	var back map[string]string
	if err := json.Unmarshal([]byte(encoded), &back); err != nil {
		t.Fatalf("encoded value is not valid JSON (%q): %v", encoded, err)
	}
	if back["a"] != "1" || back["b"] != "2" {
		t.Fatalf("unexpected multi-key round-trip: %v", back)
	}
	if !strings.HasPrefix(encoded, "{") || !strings.HasSuffix(encoded, "}") {
		t.Fatalf("encoding not wrapped in braces: %s", encoded)
	}
}

func TestEncodeMetaMap_EscapesSpecialChars(t *testing.T) {
	encoded := encodeMetaMap(map[string]string{"key": "line1\nline2\t\"q\"\\back\rend"})
	var back map[string]string
	if err := json.Unmarshal([]byte(encoded), &back); err != nil {
		t.Fatalf("escaped encoding is not valid JSON (%q): %v", encoded, err)
	}
	if back["key"] != "line1\nline2\t\"q\"\\back\rend" {
		t.Fatalf("escape round-trip mismatch: %q", back["key"])
	}
}

func TestJSONEscape(t *testing.T) {
	cases := map[string]string{
		`"`:        `\"`,
		`\`:        `\\`,
		"\n":       `\n`,
		"\r":       `\r`,
		"\t":       `\t`,
		"plain":    "plain",
		"a\"b\\c":  `a\"b\\c`,
	}
	for in, want := range cases {
		if got := jsonEscape(in); got != want {
			t.Fatalf("jsonEscape(%q) = %q, want %q", in, got, want)
		}
	}
}
