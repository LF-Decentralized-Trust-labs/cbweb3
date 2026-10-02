// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The settlement report is the ONLY record of an incoming PvP leg: the receiving
// bank's own orchestrator has none, because the counterparty locked the leg on
// another node and the amount is private. If the orchestrator cannot find its
// central bank's URL, the report is never sent, pvp_settled_legs is never written,
// and the credit side of every bank's statement is permanently empty.
//
// That is not hypothetical — it was the state of every toolkit deploy, including
// LNET, because the reporter looked for CB_INTERNAL_API_URL and that name appeared
// nowhere but in the code that read it. The orchestrator now falls back to
// CENTRAL_BANK_API_URL, which this template renders. These guards fail if that
// rendering goes away, so the feature cannot be switched off again by deleting a
// line nobody connects to a statement.
const entityEnvTemplateFile = "entityenv.go"

func entityEnvTemplateSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(entityEnvTemplateFile)
	if err != nil {
		t.Fatalf("read %s: %v", entityEnvTemplateFile, err)
	}
	return string(b)
}

func TestEntityEnv_RendersCentralBankAPIURL(t *testing.T) {
	src := entityEnvTemplateSource(t)
	if !regexp.MustCompile(`CENTRAL_BANK_API_URL=\{\{\.CentralBankAPIURL\}\}`).MatchString(src) {
		t.Error("the entity env template no longer renders CENTRAL_BANK_API_URL — the settlement " +
			"reporter defaults to it, so removing it silently empties the credit side of every " +
			"bank's statement")
	}
}

// TestBackendTemplate_LoadsTheEntityEnvFile checks the other half of the path: the
// value reaches the orchestrator container through env_file, not an explicit
// environment key, so a change from env_file to a hand-listed set would drop it
// without touching the name.
func TestBackendTemplate_LoadsTheEntityEnvFile(t *testing.T) {
	b, err := os.ReadFile("../../../provisioning/templates/entity-backend/backend-compose.yaml")
	if err != nil {
		t.Skipf("backend template not readable from here: %v", err)
	}
	src := string(b)
	if !strings.Contains(src, "ENTITY_ENV_FILE") {
		t.Fatal("the backend template no longer references ENTITY_ENV_FILE")
	}
	// The orchestrator service block must load it.
	orch := serviceBlock(src, "payment-orchestrator")
	if orch == "" {
		t.Fatal("payment-orchestrator service block not found in the backend template")
	}
	if !strings.Contains(orch, "env_file") {
		t.Error("payment-orchestrator no longer loads env_file — CENTRAL_BANK_API_URL would not " +
			"reach it, and settled PvP legs would stop being reported")
	}
}

// serviceBlock returns the YAML lines of one compose service, from its two-space key
// to the next service at the same indentation.
func serviceBlock(src, name string) string {
	lines := strings.Split(src, "\n")
	start := -1
	for i, l := range lines {
		if l == "  "+name+":" {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start + 1; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, "  ") && !strings.HasPrefix(l, "   ") && strings.HasSuffix(strings.TrimSpace(l), ":") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return strings.Join(lines[start:], "\n")
}
