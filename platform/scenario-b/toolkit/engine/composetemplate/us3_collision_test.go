package composetemplate

import (
	"path/filepath"
	"strings"
	"testing"
)

// US3: two distinct-entity envs must not collide (SC-002).
func TestUS3NoCollisionBetweenEntities(t *testing.T) {
	tpl, envA := loadTemplateAndEnv(t, "entity-besu") // bank-a (offset 10)
	envB := loadEnv(t, filepath.Join("testdata", "entity-besu.bank-b.env"))

	if r := CheckNoCollision(tpl, envA, envB); !r.OK {
		t.Fatalf("distinct entities must not collide: %+v", r.Errors)
	}
	// Sanity: identical env collides on every discriminant.
	if r := CheckNoCollision(tpl, envA, envA); r.OK {
		t.Fatal("identical entity envs should collide")
	}
}

// FR-007: backend reaches the hub cross-stack via host-gateway (parametrized).
func TestUS3BackendCrossStack(t *testing.T) {
	tpl, env := loadTemplateAndEnv(t, "entity-backend")
	interp, _ := interpolate(tpl.Raw, env)
	if !strings.Contains(interp, "host.docker.internal") {
		t.Fatal("backend must reach hub via host.docker.internal")
	}
	if !strings.Contains(tpl.Raw, "host-gateway") {
		t.Fatal("backend must declare extra_hosts host-gateway")
	}
}
