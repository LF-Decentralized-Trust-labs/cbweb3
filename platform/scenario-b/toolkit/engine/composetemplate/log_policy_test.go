// SPDX-License-Identifier: Apache-2.0

// Every provisioning template must cap its container logs.
//
// This is ported from Scenario A's provisioning_log_policy_test.go, and the reason
// it exists there is the reason it now exists here: the Scenario A Paladin
// containers reached 96 GB of logs, about 2.4 GB/day. That was not runtime
// behaviour — no compose service declared a `logging:` block, so Docker's json-file
// driver ran with no size cap and no rotation. A full disk takes the network down;
// the Fabric environment lost its only orderer that way and stayed down for weeks.
//
// Scenario A was fixed and carded. Scenario B was not, and had **zero of sixteen**
// templates capped when this landed. The defect never crossed to the twin because
// nothing asserted it on this side. That is what this file is for.
//
// The audit is deliberately over the WHOLE template tree rather than the noisiest
// services. Paladin was merely the fastest writer in Scenario A; every other service
// had the same uncapped driver and only a longer fuse. Scenario B does not run
// Paladin at all, which lowers the expected rate and changes nothing about the risk:
// Besu, the relayer and Postgres all still write without a ceiling.
package composetemplate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// logComposeService is the slice of a compose service this test judges. Everything
// else is ignored, so an unrelated schema change does not break the guard.
type logComposeService struct {
	Logging struct {
		Driver  string            `yaml:"driver"`
		Options map[string]string `yaml:"options"`
	} `yaml:"logging"`
}

type logComposeFile struct {
	Services map[string]logComposeService `yaml:"services"`
}

// relayCompose is the Cacti relay's own compose file. It is NOT under
// provisioning/templates — the relay is brought up separately by
// provisioning/scripts/start-cacti.sh — so scanning the template tree alone misses
// it. That is not hypothetical: after every template was capped, a 69-container
// stack still had exactly one uncapped container, and it was this one. A guard whose
// scope is "the directory I happened to think of" finds what it was already
// looking at.
const relayCompose = "../../../interop/hub-and-spoke/cacti/docker-compose.yaml"

// logComposeTemplates returns every compose file whose services this policy governs:
// the provisioning templates, plus the relay. It FAILS rather than returning
// nothing: a scan that silently matched zero files would let the test below pass
// while proving nothing, which is the failure mode that matters most in a guard —
// and it is exactly how a whole scenario ended up with no coverage.
func logComposeTemplates(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		t.Fatalf("read %s: %v", templatesDir, err)
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.Contains(name, "compose") && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			found = append(found, filepath.Join(templatesDir, name))
		}
	}
	// Named explicitly rather than discovered, so it cannot quietly drop out of scope
	// the way it was never in it.
	if _, err := os.Stat(relayCompose); err != nil {
		t.Fatalf("relay compose not found at %s: %v — it holds a long-lived service that "+
			"polls both chains, and it is the file this guard was extended to cover", relayCompose, err)
	}
	found = append(found, relayCompose)
	// Seventeen at the time of writing (sixteen templates plus the relay). The floor is
	// deliberately below that so adding a template does not fail the build, and far
	// enough above zero that a broken path cannot pass.
	if len(found) < 15 {
		t.Fatalf("found %d compose templates under %s, expected at least 14 — the scan matched too "+
			"little, so this test proves nothing", len(found), templatesDir)
	}
	return found
}

// TestProvisioningTemplates_CapContainerLogs is the assertion the 96 GB incident
// asks for: every service bounds its own log growth.
//
// Both options are required. max-size alone rotates into an unbounded number of
// files; max-file alone caps the count of files that each grow forever.
func TestProvisioningTemplates_CapContainerLogs(t *testing.T) {
	var services int

	for _, path := range logComposeTemplates(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var cf logComposeFile
			if err := yaml.Unmarshal(raw, &cf); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			if len(cf.Services) == 0 {
				t.Fatalf("%s declares no services; this test would vacuously pass", path)
			}
			services += len(cf.Services)

			for name, svc := range cf.Services {
				if svc.Logging.Options["max-size"] == "" {
					t.Errorf("service %q in %s has no logging max-size: the json-file driver will "+
						"grow without limit until the disk fills", name, path)
				}
				if svc.Logging.Options["max-file"] == "" {
					t.Errorf("service %q in %s has no logging max-file: rotation without a file "+
						"count is still unbounded on disk", name, path)
				}
			}
		})
	}

	// Twenty-five at the time of writing (twenty-four template services plus the relay). Without this the suite would still pass if
	// the YAML shape changed such that no service was parsed at all — every subtest
	// would find an empty map and the t.Fatalf above fires per file, but a wholesale
	// parse change could leave the outer count at zero unnoticed.
	if services < 21 {
		t.Errorf("only %d services were audited across the tree; expected at least 20 — the "+
			"parse is matching too little to be a meaningful guard", services)
	}
}

// TestLogPolicy_FailsOnAnUncappedService is the guard on the guard: it proves the
// assertion above actually rejects the shape it is meant to reject, rather than
// passing because of how the YAML is read.
func TestLogPolicy_FailsOnAnUncappedService(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		raw    string
		capped bool
	}{
		{
			name: "no logging block at all — the state every Scenario B template was in",
			raw: `services:
  besu:
    image: hyperledger/besu:25.8.0
`,
		},
		{
			name: "max-size without max-file — rotates into unbounded files",
			raw: `services:
  besu:
    logging:
      driver: json-file
      options:
        max-size: "50m"
`,
		},
		{
			name: "max-file without max-size — a fixed count of files that grow forever",
			raw: `services:
  besu:
    logging:
      driver: json-file
      options:
        max-file: "5"
`,
		},
		{
			name: "both present",
			raw: `services:
  besu:
    logging:
      driver: json-file
      options:
        max-size: "50m"
        max-file: "5"
`,
			capped: true,
		},
	} {
		var cf logComposeFile
		if err := yaml.Unmarshal([]byte(tc.raw), &cf); err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		svc := cf.Services["besu"]
		got := svc.Logging.Options["max-size"] != "" && svc.Logging.Options["max-file"] != ""
		if got != tc.capped {
			t.Errorf("%s: judged capped=%v, want %v", tc.name, got, tc.capped)
		}
	}
}
