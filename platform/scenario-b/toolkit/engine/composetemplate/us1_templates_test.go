package composetemplate

import "testing"

// US1 (+ entity-besu from convergence): hub and entity templates interpolate
// fully with their .env.example (SC-001) and contain no secrets (SC-006).
var us1Templates = []string{
	"hub",
	"entity-besu",
	"entity-infra",
	"entity-keycloak",
	"entity-backend",
	"entity-frontend",
}

func TestUS1TemplatesValidate(t *testing.T) {
	for _, name := range us1Templates {
		t.Run(name, func(t *testing.T) {
			tpl, env := loadTemplateAndEnv(t, name)
			r := Validate(tpl, env)
			if !r.OK {
				t.Fatalf("%s should validate, errors: %+v", name, r.Errors)
			}
		})
	}
}

// SC-005: a missing mandatory variable fails explicitly.
func TestUS1MissingMandatoryVarFails(t *testing.T) {
	for _, name := range us1Templates {
		t.Run(name, func(t *testing.T) {
			tpl, env := loadTemplateAndEnv(t, name)
			req := tpl.RequiredVars()
			if len(req) == 0 {
				t.Fatalf("%s has no mandatory vars — unexpected", name)
			}
			delete(env, req[0]) // drop one mandatory var
			r := Validate(tpl, env)
			if r.OK || !hasRule(r, "interpolation") {
				t.Fatalf("%s: dropping %s should fail interpolation, got %+v", name, req[0], r.Errors)
			}
		})
	}
}
