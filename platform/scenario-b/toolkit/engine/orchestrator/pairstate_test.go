package orchestrator

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

func TestPairID(t *testing.T) {
	if got := pairID("BRL", "ARS"); got != "W-BRL-ARS" {
		t.Fatalf("pairID = %q, want W-BRL-ARS", got)
	}
}

func TestPairStatus(t *testing.T) {
	cases := []struct {
		name       string
		out        string
		err        error
		wantStatus string
		wantExists bool
	}{
		{"active", "ACTIVE", nil, "ACTIVE", true},
		{"proposed", "PROPOSED", nil, "PROPOSED", true},
		{"revert", "", errBoom, "", false},
		{"empty", "", nil, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &exec.FakeRunner{
				Outputs: map[string][]byte{"cast": []byte(tc.out)},
				Errs:    map[string]error{},
			}
			if tc.err != nil {
				fake.Errs["cast"] = tc.err
			}
			status, exists, err := pairStatus(context.Background(), fake, "http://hub", "0xPR", "W-BRL-ARS")
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if status != tc.wantStatus || exists != tc.wantExists {
				t.Fatalf("got (%q,%v), want (%q,%v)", status, exists, tc.wantStatus, tc.wantExists)
			}
		})
	}
}

var errBoom = errBoomType("boom")

type errBoomType string

func (e errBoomType) Error() string { return string(e) }
