// SPDX-License-Identifier: Apache-2.0

// This file covers the isConflictError classifier branches that the broader
// compliance-handler suite does not reach. Hermetic: pure error inspection.
package handlers

import (
	"errors"
	"testing"
)

func TestIsConflictError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error is not a conflict", err: nil, want: false},
		{name: "already exists", err: errors.New("participant already exists"), want: true},
		{name: "conflict keyword", err: errors.New("HTTP 409 Conflict"), want: true},
		{name: "duplicate keyword", err: errors.New("duplicate key value"), want: true},
		{name: "case-insensitive match", err: errors.New("Record ALREADY EXISTS"), want: true},
		{name: "unrelated error is not a conflict", err: errors.New("connection refused"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isConflictError(tc.err); got != tc.want {
				t.Fatalf("isConflictError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
