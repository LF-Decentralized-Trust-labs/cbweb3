// SPDX-License-Identifier: Apache-2.0

// Every Besu node that serves JSON-RPC over WebSocket must declare its connection
// ceiling explicitly, as a parameter.
//
// Ported from the Scenario A incident (SUP-152): the Chile central bank's Besu ran
// on the implicit default of --rpc-ws-max-active-connections=80, set nowhere in the
// templates. Leaked connections filled all 80 slots, every new connection was
// refused, the local Paladin node could not open its block listener and never
// finished starting, and the spoke settled nothing for fifteen days. Nobody could
// see the ceiling because it did not exist in configuration.
//
// Scenario B runs the same Besu image with the same default. The defect is the
// implicit value, not the scenario, so the guard is the same.
//
// The scan is over the WHOLE template tree and keys on --rpc-ws-enabled rather than
// on a list of known Besu templates: a new Besu template added later is covered the
// moment it enables WebSocket, without anyone remembering this file exists.
package composetemplate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// wsLimitVar is the one variable an operator sets to change the ceiling. All Besu
// templates read the same name so the knob does not depend on which template an
// entity happens to use.
const wsLimitVar = "BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS"

// wsLimitFlagRe accepts the flag only in its parameterised form with a numeric
// default: ${BESU_RPC_WS_MAX_ACTIVE_CONNECTIONS:-<n>}. A literal number would make
// the limit explicit but not tunable per environment; a variable without a default
// would make compose fail on every existing env file.
var wsLimitFlagRe = regexp.MustCompile(
	`^--rpc-ws-max-active-connections=\$\{` + wsLimitVar + `:-[1-9][0-9]*\}$`)

type besuComposeService struct {
	Command []string `yaml:"command"`
}

type besuComposeFile struct {
	Services map[string]besuComposeService `yaml:"services"`
}

func TestBesuTemplates_DeclareWebSocketConnectionLimit(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(templatesDir, "*.compose.yaml"))
	if err != nil {
		t.Fatalf("glob %s: %v", templatesDir, err)
	}

	wsServices := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var cf besuComposeFile
		if err := yaml.Unmarshal(raw, &cf); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for name, svc := range cf.Services {
			if !hasArg(svc.Command, "--rpc-ws-enabled") {
				continue
			}
			wsServices++

			var limits []string
			for _, arg := range svc.Command {
				if strings.HasPrefix(arg, "--rpc-ws-max-active-connections") {
					limits = append(limits, arg)
				}
			}
			switch {
			case len(limits) == 0:
				t.Errorf("service %q in %s enables WebSocket RPC without "+
					"--rpc-ws-max-active-connections: it runs on Besu's implicit default of 80, "+
					"which is the ceiling that took a Scenario A spoke down for fifteen days",
					name, filepath.Base(path))
			case len(limits) > 1:
				t.Errorf("service %q in %s sets --rpc-ws-max-active-connections %d times (%v); "+
					"exactly one must be in force", name, filepath.Base(path), len(limits), limits)
			case !wsLimitFlagRe.MatchString(limits[0]):
				t.Errorf("service %q in %s sets %q; want the parameterised form "+
					"--rpc-ws-max-active-connections=${%s:-<n>}", name, filepath.Base(path),
					limits[0], wsLimitVar)
			}
		}
	}

	// The hub and the three entity Besu templates enable WebSocket today. Fewer than
	// that means the scan stopped seeing them, and the loop above proved nothing.
	if wsServices < 4 {
		t.Fatalf("found %d Besu services with WebSocket RPC under %s, expected at least 4: "+
			"the scan matched too little, so this test proves nothing", wsServices, templatesDir)
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want || strings.HasPrefix(a, want+"=") {
			return true
		}
	}
	return false
}
