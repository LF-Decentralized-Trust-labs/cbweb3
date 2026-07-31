// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// firstClientMapper returns the first protocol mapper's config for clients[idx].
func firstClientMapperConfig(t *testing.T, realmJSON []byte, idx int) map[string]any {
	t.Helper()
	var realm map[string]any
	if err := json.Unmarshal(realmJSON, &realm); err != nil {
		t.Fatalf("invalid realm JSON: %v", err)
	}
	clients := realm["clients"].([]any)
	c := clients[idx].(map[string]any)
	mappers, ok := c["protocolMappers"].([]any)
	if !ok || len(mappers) == 0 {
		t.Fatalf("client %d has no protocolMappers: %v", idx, c)
	}
	m := mappers[0].(map[string]any)
	if m["protocolMapper"] != "oidc-audience-mapper" {
		t.Fatalf("expected oidc-audience-mapper, got %v", m["protocolMapper"])
	}
	return m["config"].(map[string]any)
}

// Backend login clients (governance/treasury) stamp the backend audience; the NOC
// realm client stamps the NOC audience. The two must be distinct so a token minted
// for one consumer cannot satisfy the other's aud check.
func TestRenderRealmJSON_AudienceMapper(t *testing.T) {
	if keycloakBackendAudience == keycloakNOCAudience {
		t.Fatal("backend and NOC audiences must differ")
	}

	entity, err := renderRealmJSON(centralBankRealmPlans("central-bank-brazil", nil)[0])
	if err != nil {
		t.Fatalf("renderRealmJSON (entity): %v", err)
	}
	if got := firstClientMapperConfig(t, entity, 0)["included.custom.audience"]; got != keycloakBackendAudience {
		t.Errorf("governance client audience = %v, want %v", got, keycloakBackendAudience)
	}
	if got := firstClientMapperConfig(t, entity, 1)["included.custom.audience"]; got != keycloakBackendAudience {
		t.Errorf("treasury client audience = %v, want %v", got, keycloakBackendAudience)
	}

	noc, err := renderRealmJSON(nocRealmPlan())
	if err != nil {
		t.Fatalf("renderRealmJSON (noc): %v", err)
	}
	if got := firstClientMapperConfig(t, noc, 0)["included.custom.audience"]; got != keycloakNOCAudience {
		t.Errorf("noc client audience = %v, want %v", got, keycloakNOCAudience)
	}
}

// A client with no Audience must not get an audience mapper (so aud stays opt-in).
func TestRenderRealmJSON_NoMapperWithoutAudience(t *testing.T) {
	data, err := renderRealmJSON(KeycloakRealmPlan{
		Realm:   "x",
		Clients: []KeycloakClientPlan{{ClientID: "x-client", Secret: "s", Roles: []string{"ROLE_X"}}},
	})
	if err != nil {
		t.Fatalf("renderRealmJSON: %v", err)
	}
	var realm map[string]any
	if err := json.Unmarshal(data, &realm); err != nil {
		t.Fatalf("invalid realm JSON: %v", err)
	}
	c := realm["clients"].([]any)[0].(map[string]any)
	if _, ok := c["protocolMappers"]; ok {
		t.Errorf("client without Audience must not carry protocolMappers: %v", c)
	}
}

// The rendered per-entity backend .env must set KEYCLOAK_AUDIENCE so the auth
// service enforces it.
func TestRenderEntityEnv_StampsAudience(t *testing.T) {
	out := filepath.Join(t.TempDir(), ".env")
	if err := RenderEntityEnv(EntityEnvData{EntityName: "central-bank-brazil", KCAudience: keycloakBackendAudience}, out); err != nil {
		t.Fatalf("RenderEntityEnv: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if !strings.Contains(string(b), "KEYCLOAK_AUDIENCE="+keycloakBackendAudience) {
		t.Errorf("rendered env must set KEYCLOAK_AUDIENCE=%s; got:\n%s", keycloakBackendAudience, b)
	}
}
