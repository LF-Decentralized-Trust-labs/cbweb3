// SPDX-License-Identifier: Apache-2.0

// Every provisioning template must cap its container logs, and none may hardcode
// the debug level.
//
// Both halves come from one incident: the Scenario A Paladin containers reached
// 96 GB of logs, about 2.4 GB/day since the July deploy. That was not runtime
// behaviour — `debug` was hardcoded in five places and no compose service declared
// a `logging:` block, so Docker's json-file driver ran with no size cap and no
// rotation. A full disk takes the network down: the Fabric environment lost its
// only orderer that way and stayed down for weeks.
//
// The audit is deliberately over the WHOLE template tree rather than the Paladin
// services alone. Paladin was merely the fastest writer; every other service had
// the same uncapped driver and only a longer fuse.
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// templatesDir is the provisioning template tree, relative to this package.
const templatesDir = "../../../provisioning/templates"

// composeService is the slice of a compose service this test judges. Everything
// else is ignored, so an unrelated schema change does not break the guard.
type composeService struct {
	Logging struct {
		Driver  string            `yaml:"driver"`
		Options map[string]string `yaml:"options"`
	} `yaml:"logging"`
}

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

// composeTemplates returns every compose template in the tree. It fails rather than
// returning nothing: a glob that silently matches zero files would let both tests
// below pass while proving nothing, which is the failure mode that matters most in
// a guard.
func composeTemplates(t *testing.T) []string {
	t.Helper()

	var found []string
	err := filepath.WalkDir(templatesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.Contains(name, "compose") && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", templatesDir, err)
	}
	if len(found) < 9 {
		t.Fatalf("found %d compose templates under %s, expected at least 9 — the glob matched too "+
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
	for _, path := range composeTemplates(t) {
		t.Run(filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}

			var cf composeFile
			if err := yaml.Unmarshal(raw, &cf); err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			if len(cf.Services) == 0 {
				t.Fatalf("%s declares no services; this test would vacuously pass", path)
			}

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
}

// TestProvisioningTemplates_DoNotHardcodeDebug covers the other half. The level
// stays reachable — it is genuinely useful during a bring-up — but as an
// environment override, not as a default nobody notices.
//
// The whole tree is scanned, not only compose files: three of the five original
// sites were in paladin-config *.tmpl files, where `level: debug` sets it for the
// Paladin process itself regardless of what compose says.
func TestProvisioningTemplates_DoNotHardcodeDebug(t *testing.T) {
	var offenders []string

	err := filepath.WalkDir(templatesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			// Comments are documentation, not configuration: a line explaining how to
			// turn debug on must not fail this.
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.Contains(trimmed, "level: debug") ||
				strings.Contains(trimmed, "PALADIN_LOG_LEVEL=debug") ||
				strings.Contains(trimmed, "PALADIN_LOG_LEVEL: debug") {
				offenders = append(offenders, filepath.ToSlash(path)+":"+itoa(i+1)+" "+trimmed)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", templatesDir, err)
	}

	if len(offenders) > 0 {
		t.Errorf("the debug log level is hardcoded in %d place(s); make it an env override with an "+
			"info default instead:\n  %s", len(offenders), strings.Join(offenders, "\n  "))
	}
}

// itoa avoids pulling strconv in for one call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
