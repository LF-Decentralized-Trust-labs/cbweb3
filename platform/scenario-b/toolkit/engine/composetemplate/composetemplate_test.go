// SPDX-License-Identifier: Apache-2.0

package composetemplate

import (
	"os"
	"path/filepath"
	"testing"
)

// Núcleo: Load faz parse; Validate detecta var obrigatória ausente e segredo.
func TestValidateInterpolationAndSecrets(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	os.WriteFile(good, []byte(`services:
  app:
    container_name: "${APP_NAME:?}"
    image: "nginx:${TAG:-latest}"
    ports:
      - "${APP_PORT:?}:80"
volumes:
  app_data:
    name: "${VOL_PREFIX:?}_app_data"
`), 0o644)

	tpl, err := Load(good)
	if err != nil {
		t.Fatal(err)
	}

	// Env completo → OK.
	full := map[string]string{"APP_NAME": "svc-a", "APP_PORT": "8080", "VOL_PREFIX": "a"}
	if r := Validate(tpl, full); !r.OK {
		t.Fatalf("esperava OK, erros: %+v", r.Errors)
	}

	// Var obrigatória ausente → erro de interpolação.
	partial := map[string]string{"APP_NAME": "svc-a", "VOL_PREFIX": "a"} // falta APP_PORT
	r := Validate(tpl, partial)
	if r.OK || !hasRule(r, "interpolation") {
		t.Fatalf("esperava erro de interpolation, got %+v", r.Errors)
	}

	// RequiredVars não inclui a de default (TAG).
	req := tpl.RequiredVars()
	if contains(req, "TAG") || !contains(req, "APP_PORT") {
		t.Fatalf("RequiredVars inesperado: %v", req)
	}
}

func TestValidateRejectsEmbeddedSecret(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yaml")
	os.WriteFile(bad, []byte(`services:
  db:
    image: postgres
    environment:
      POSTGRES_PASSWORD: "hunter2literal"
`), 0o644)
	tpl, _ := Load(bad)
	r := Validate(tpl, map[string]string{})
	if r.OK || !hasRule(r, "no-secrets") {
		t.Fatalf("esperava erro no-secrets, got %+v", r.Errors)
	}
}

func TestNoSecretsAllowsVarReference(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.yaml")
	os.WriteFile(ok, []byte(`services:
  db:
    image: postgres
    environment:
      POSTGRES_PASSWORD: "${POSTGRES_PASSWORD:?}"
volumes:
  pg:
    name: "${VP:?}_pg"
`), 0o644)
	tpl, _ := Load(ok)
	r := Validate(tpl, map[string]string{"POSTGRES_PASSWORD": "x", "VP": "a"})
	if !r.OK {
		t.Fatalf("referência via ${} não deveria disparar no-secrets: %+v", r.Errors)
	}
}

func hasRule(r Result, rule string) bool {
	for _, e := range r.Errors {
		if e.Rule == rule {
			return true
		}
	}
	return false
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
