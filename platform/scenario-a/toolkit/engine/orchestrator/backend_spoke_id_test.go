// SPDX-License-Identifier: Apache-2.0

// The payment-orchestrator must receive SPOKE_ID.
//
// It uses the spoke id for the receiver-locality check, and the value CANNOT be
// parsed back out of PALADIN_IDENTITY: a node name is `<spokeId>-<bankId>` with
// hyphens allowed in both halves, so "spoke-costa-rica-cb1" is indistinguishable
// from a spoke "spoke-costa" with a bank "rica-cb1".
//
// This is a guard rather than a nicety because the service FALLS BACK to that
// two-segment guess when the variable is missing. The fallback keeps an older
// environment starting, which is deliberate — but it also means deleting the
// template line would not fail a deploy, would not fail a health check, and would
// only surface as one warning line in a log nobody reads. The failure it hides is
// two spokes sharing their first two segments being treated as one.
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// backendComposePath is the entity-backend template, relative to this package.
const backendComposePath = "../../../provisioning/templates/entity-backend/backend-compose.yaml"

type spokeIDComposeFile struct {
	Services map[string]struct {
		Environment map[string]string `yaml:"environment"`
	} `yaml:"services"`
}

func TestBackendTemplate_PassesSpokeIDToPaymentOrchestrator(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(backendComposePath)
	if err != nil {
		t.Fatalf("read %s: %v", backendComposePath, err)
	}
	var cf spokeIDComposeFile
	if err := yaml.Unmarshal(raw, &cf); err != nil {
		t.Fatalf("parse %s: %v", backendComposePath, err)
	}

	svc, ok := cf.Services["payment-orchestrator"]
	if !ok {
		t.Fatalf("%s declares no payment-orchestrator service; this guard would pass "+
			"while proving nothing", filepath.ToSlash(backendComposePath))
	}

	got, ok := svc.Environment["SPOKE_ID"]
	if !ok {
		t.Fatal("payment-orchestrator has no SPOKE_ID in its environment. Without it the " +
			"service falls back to guessing the spoke id from PALADIN_IDENTITY by taking " +
			"two hyphen-separated segments, which reads spoke-costa-rica as spoke-costa and " +
			"cannot tell two spokes sharing those segments apart.")
	}

	// The engine already exports SPOKE_ID for this compose file (it names the
	// cb_tls volume), so the reference must be to that variable and not a literal
	// pinned in the template.
	if !strings.Contains(got, "${SPOKE_ID") {
		t.Errorf("SPOKE_ID is set to %q, which does not reference the ${SPOKE_ID} the "+
			"engine exports — a literal here would pin every entity to one spoke", got)
	}
}
